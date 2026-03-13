// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

	ie "github.com/self-host/self-host/internal/errors"
)

// Validate a request against the OpenAPI specification
func OapiRequestValidator(swagger *openapi3.T) func(http.Handler) http.Handler {
	return OapiRequestValidatorWithOptions(swagger, nil)
}

// Validate a request against the OpenAPI specification (with options)
func OapiRequestValidatorWithOptions(swagger *openapi3.T, options *Options) func(http.Handler) http.Handler {
	if swagger == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	router, err := legacyrouter.NewRouter(swagger)
	if err != nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if router == nil {
				next.ServeHTTP(w, r)
				return
			}

			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				ie.SendHTTPError(w, ie.NewInvalidRequestError(err))
				return
			}

			filterOptions := &openapi3filter.Options{
				AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
					return nil
				},
			}
			if options != nil {
				filterOptions = &options.Options
				if filterOptions.AuthenticationFunc == nil {
					filterOptions.AuthenticationFunc = func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
						return nil
					}
				}
			}

			requestValidationInput := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    filterOptions,
			}
			if err := openapi3filter.ValidateRequest(r.Context(), requestValidationInput); err != nil {
				e, ok := err.(*openapi3filter.RequestError)
				if ok {
					nerr := &ie.HTTPError{
						Code:    400,
						Message: e.Error(),
					}
					ie.SendHTTPError(w, nerr)
				} else {
					ie.SendHTTPError(w, ie.NewInvalidRequestError(err))
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
