// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"database/sql"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/self-host/self-host/postgres"
)

type PolicyCheckService struct {
	q *postgres.Queries
}

type AccessDecision struct {
	Resource       string
	Access         bool
	PolicyUUID     *uuid.UUID
	GroupUUID      *uuid.UUID
	Priority       *int32
	Effect         *string
	Action         string
	MatchedPattern *string
}

type policyCheckCacheKey struct{}

type policyDecisionCache struct {
	mu        sync.RWMutex
	decisions map[string]AccessDecision
}

type accessDecisionKey struct {
	token    string
	action   string
	resource string
}

// NewPolicyCheck service
func NewPolicyCheckService(db *sql.DB) *PolicyCheckService {
	return &PolicyCheckService{
		q: postgres.New(db),
	}
}

func WithPolicyCheckCache(ctx context.Context) context.Context {
	if _, ok := ctx.Value(policyCheckCacheKey{}).(*policyDecisionCache); ok {
		return ctx
	}

	return context.WithValue(ctx, policyCheckCacheKey{}, &policyDecisionCache{
		decisions: make(map[string]AccessDecision),
	})
}

func cacheLookup(ctx context.Context, key accessDecisionKey) (AccessDecision, bool) {
	cache, ok := ctx.Value(policyCheckCacheKey{}).(*policyDecisionCache)
	if !ok {
		return AccessDecision{}, false
	}

	cache.mu.RLock()
	decision, ok := cache.decisions[cacheKeyString(key)]
	cache.mu.RUnlock()
	return decision, ok
}

func cacheStoreMany(ctx context.Context, token []byte, action string, decisions []AccessDecision) {
	cache, ok := ctx.Value(policyCheckCacheKey{}).(*policyDecisionCache)
	if !ok {
		return
	}

	cache.mu.Lock()
	for _, decision := range decisions {
		cache.decisions[cacheKeyString(accessDecisionKey{
			token:    string(token),
			action:   action,
			resource: decision.Resource,
		})] = decision
	}
	cache.mu.Unlock()
}

func cacheKeyString(key accessDecisionKey) string {
	return key.token + "\x00" + key.action + "\x00" + key.resource
}

func (pc *PolicyCheckService) UserHasAccessViaToken(ctx context.Context, token []byte, action string, resource string) (bool, error) {
	decision, err := pc.ExplainUserAccessViaToken(ctx, token, action, resource)
	if err != nil {
		return false, err
	}

	return decision.Access, nil
}

func (pc *PolicyCheckService) UserHasManyAccessViaToken(ctx context.Context, token []byte, action string, resources []string) (bool, error) {
	if len(resources) == 0 {
		return false, nil
	}

	decisions, err := pc.ExplainUserAccessManyViaToken(ctx, token, action, resources)
	if err != nil {
		return false, err
	}

	for _, decision := range decisions {
		if !decision.Access {
			return false, nil
		}
	}

	return true, nil
}

func (pc *PolicyCheckService) ExplainUserAccessViaToken(ctx context.Context, token []byte, action string, resource string) (*AccessDecision, error) {
	decisions, err := pc.ExplainUserAccessManyViaToken(ctx, token, action, []string{resource})
	if err != nil {
		return nil, err
	}
	if len(decisions) == 0 {
		return &AccessDecision{
			Resource: resource,
			Access:   false,
			Action:   action,
		}, nil
	}

	return &decisions[0], nil
}

func (pc *PolicyCheckService) ExplainUserAccessManyViaToken(ctx context.Context, token []byte, action string, resources []string) ([]AccessDecision, error) {
	if len(resources) == 0 {
		return nil, nil
	}

	uniqueResources := dedupeStrings(resources)
	decisionsByResource := make(map[string]AccessDecision, len(uniqueResources))
	uncached := make([]string, 0, len(uniqueResources))

	for _, resource := range uniqueResources {
		decision, ok := cacheLookup(ctx, accessDecisionKey{
			token:    string(token),
			action:   action,
			resource: resource,
		})
		if ok {
			decisionsByResource[resource] = decision
			continue
		}
		uncached = append(uncached, resource)
	}

	if len(uncached) > 0 {
		rows, err := pc.q.ExplainUserTokenAccessMany(ctx, postgres.ExplainUserTokenAccessManyParams{
			Token:     token,
			Action:    postgres.PolicyAction(action),
			Resources: uncached,
		})
		if err != nil {
			return nil, err
		}

		fresh := make([]AccessDecision, 0, len(rows))
		for _, row := range rows {
			decision := AccessDecision{
				Resource: row.Resource,
				Access:   row.Access,
				Action:   action,
			}
			if row.Matched {
				id := row.PolicyUuid
				decision.PolicyUUID = &id
				groupID := row.GroupUuid
				decision.GroupUUID = &groupID
				p := row.Priority
				decision.Priority = &p
				e := string(row.Effect)
				decision.Effect = &e
				pattern := row.PolicyResource
				decision.MatchedPattern = &pattern
			}
			if row.Matched && row.GroupUuid != uuid.Nil {
				id := row.GroupUuid
				decision.GroupUUID = &id
			}

			decisionsByResource[row.Resource] = decision
			fresh = append(fresh, decision)
		}

		cacheStoreMany(ctx, token, action, fresh)
	}

	ordered := make([]AccessDecision, 0, len(resources))
	for _, resource := range resources {
		decision, ok := decisionsByResource[resource]
		if !ok {
			decision = AccessDecision{
				Resource: resource,
				Access:   false,
				Action:   action,
			}
		}
		ordered = append(ordered, decision)
	}

	return ordered, nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
