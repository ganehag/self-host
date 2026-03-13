// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"database/sql"
	"strings"

	"github.com/self-host/self-host/postgres"
)

type PolicyCheckService struct {
	q *postgres.Queries
}

// NewPolicyCheck service
func NewPolicyCheckService(db *sql.DB) *PolicyCheckService {
	return &PolicyCheckService{
		q: postgres.New(db),
	}
}

func (pc *PolicyCheckService) UserHasAccessViaToken(ctx context.Context, token []byte, action string, resource string) (bool, error) {
	params := postgres.CheckUserTokenHasAccessParams{
		Action:   postgres.PolicyAction(action),
		Resource: resource,
		Token:    token,
	}

	hasAccess, err := pc.q.CheckUserTokenHasAccess(ctx, params)
	if err != nil {
		return false, err
	}

	return hasAccess, nil
}

func (pc *PolicyCheckService) UserHasManyAccessViaToken(ctx context.Context, token []byte, action string, resources []string) (bool, error) {
	policies, err := pc.q.FindPoliciesByToken(ctx, token)
	if err != nil {
		return false, err
	}

	if len(resources) == 0 {
		return false, nil
	}

	matchingPolicies := make([]postgres.GroupPolicy, 0, len(policies))
	for _, policy := range policies {
		if string(policy.Action) != action {
			continue
		}
		matchingPolicies = append(matchingPolicies, policy)
	}

	if len(matchingPolicies) == 0 {
		return false, nil
	}

	for _, resource := range resources {
		allowed := false
		denied := false

		for _, policy := range matchingPolicies {
			if likeMatch(resource, policy.Resource) == false {
				continue
			}

			if policy.Effect == postgres.PolicyEffectAllow {
				allowed = true
			}
			if policy.Effect == postgres.PolicyEffectDeny {
				denied = true
				break
			}
		}

		if allowed == false || denied {
			return false, nil
		}
	}

	return true, nil
}

func likeMatch(value, pattern string) bool {
	if pattern == "%" {
		return true
	}
	if strings.IndexByte(pattern, '_') == -1 && strings.IndexByte(pattern, '%') == -1 {
		return value == pattern
	}

	return likeMatchRunes([]rune(value), []rune(pattern))
}

func likeMatchRunes(value, pattern []rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '%':
			pattern = pattern[1:]
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(value); i++ {
				if likeMatchRunes(value[i:], pattern) {
					return true
				}
			}
			return false
		case '_':
			if len(value) == 0 {
				return false
			}
			value = value[1:]
			pattern = pattern[1:]
		default:
			if len(value) == 0 || value[0] != pattern[0] {
				return false
			}
			value = value[1:]
			pattern = pattern[1:]
		}
	}

	return len(value) == 0
}
