// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	ie "github.com/self-host/self-host/internal/errors"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

type visitorController struct {
	visitors sync.Map

	rateLimit       int
	maxBurst        int
	cleanUpInterval time.Duration
	tickInterval    time.Duration
}

// Retrieve and return the rate limiter for the current visitor if it
// already exists. Otherwise create a new rate limiter and add it to
// the visitors map, using the API token as the key.
func (c *visitorController) GetVisitor(token string) (*rate.Limiter, time.Time) {
	now := time.Now()
	if existing, ok := c.visitors.Load(token); ok {
		v := existing.(*visitor)
		v.lastSeen.Store(now.UnixNano())
		return v.limiter, now
	}

	rt := rate.Every(time.Hour / time.Duration(c.rateLimit))
	candidate := &visitor{
		limiter: rate.NewLimiter(rt, c.maxBurst),
	}
	candidate.lastSeen.Store(now.UnixNano())

	actual, _ := c.visitors.LoadOrStore(token, candidate)
	v := actual.(*visitor)
	v.lastSeen.Store(now.UnixNano())
	return v.limiter, now
}

// Background task
func (c *visitorController) Start() {
	go func() {
		ticker := time.NewTicker(c.tickInterval)
		defer ticker.Stop()

		for {
			<-ticker.C

			cutoff := time.Now().Add(-c.cleanUpInterval).UnixNano()
			c.visitors.Range(func(key, value any) bool {
				v, ok := value.(*visitor)
				if !ok || v == nil || v.lastSeen.Load() < cutoff {
					c.visitors.Delete(key)
				}
				return true
			})
		}
	}()
}

func newVisitorController(r, b int, cleanUp time.Duration) *visitorController {
	tickInterval := time.Minute / 10
	if cleanUp > 0 && cleanUp < tickInterval {
		tickInterval = cleanUp
	}

	return &visitorController{
		rateLimit:       r,
		maxBurst:        b,
		cleanUpInterval: cleanUp,
		tickInterval:    tickInterval,
	}
}

// Rate control middleware
func RateControl(reqPerHour int, maxburst int, cleanup time.Duration) func(http.Handler) http.Handler {
	// FIXME: From config, somehow.
	vc := newVisitorController(reqPerHour, maxburst, cleanup)
	vc.Start()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			domain, apiKey, ok := r.BasicAuth()
			if ok == false {
				ie.SendHTTPError(w, ie.ErrorUnauthorized)
				return
			}

			if domain == "" || apiKey == "" {
				ie.SendHTTPError(w, ie.ErrorUnauthorized)
				return
			}

			lutkey := domain + "." + apiKey
			limiter, _ := vc.GetVisitor(lutkey)

			if limiter.Allow() == false {
				hourRate := limiter.Limit() * 3600

				// Number of requests per hour
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%v", hourRate))

				// FIXME: How do we represent these when we have a leaky bucket?
				// w.Header().Set("X-RateLimit-Reset", ... ))
				// w.Header().Set("X-RateLimit-Remaining", ...)

				ie.SendHTTPError(w, ie.ErrorTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
