// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/postgres"
)

type policyRule struct {
	ID        *uuid.UUID
	GroupUUID uuid.UUID
	Priority  int32
	Effect    string
	Action    string
	Resource  string
}

func validatePolicyResource(resource string) error {
	if resource == "" {
		return ie.NewBadRequestError(fmt.Errorf("resource must not be empty"))
	}
	if resource != strings.TrimSpace(resource) {
		return ie.NewBadRequestError(fmt.Errorf("resource must not have leading or trailing whitespace"))
	}
	if strings.IndexFunc(resource, unicode.IsSpace) >= 0 {
		return ie.NewBadRequestError(fmt.Errorf("resource must not contain whitespace"))
	}
	if strings.HasPrefix(resource, "/") || strings.HasSuffix(resource, "/") {
		return ie.NewBadRequestError(fmt.Errorf("resource must not start or end with '/'"))
	}
	if strings.Contains(resource, "//") {
		return ie.NewBadRequestError(fmt.Errorf("resource must not contain empty path segments"))
	}

	for _, segment := range strings.Split(resource, "/") {
		if segment == "" {
			return ie.NewBadRequestError(fmt.Errorf("resource must not contain empty path segments"))
		}
		if strings.Contains(segment, "%") && segment != "%" {
			return ie.NewBadRequestError(fmt.Errorf("resource wildcards must occupy an entire path segment"))
		}
	}

	return nil
}

func validatePolicyRuleShape(rule policyRule) error {
	if rule.Priority < 0 {
		return ie.NewBadRequestError(fmt.Errorf("priority must be zero or greater"))
	}

	return validatePolicyResource(rule.Resource)
}

func (s *PolicyService) validatePolicyRule(ctx context.Context, q *postgres.Queries, rule policyRule) error {
	if err := validatePolicyRuleShape(rule); err != nil {
		return err
	}

	policies, err := q.FindPoliciesByGroup(ctx, rule.GroupUUID)
	if err != nil {
		return err
	}

	for _, existing := range policies {
		if rule.ID != nil && existing.Uuid == *rule.ID {
			continue
		}
		if string(existing.Action) != rule.Action {
			continue
		}
		if policyShadows(existing, rule) {
			return ie.NewBadRequestError(fmt.Errorf("policy is shadowed by existing policy %s", existing.Uuid))
		}
	}

	return nil
}

func policyShadows(existing postgres.GroupPolicy, candidate policyRule) bool {
	if !policyResourceCovers(existing.Resource, candidate.Resource) {
		return false
	}

	if existing.Priority < candidate.Priority {
		return true
	}
	if existing.Priority > candidate.Priority {
		return false
	}

	if existing.Effect == postgres.PolicyEffectDeny {
		return true
	}

	return string(existing.Effect) == candidate.Effect
}

func policyResourceCovers(existing, candidate string) bool {
	if existing == "%" || existing == candidate {
		return true
	}

	if !strings.Contains(existing, "%") {
		return false
	}

	if strings.Count(existing, "%") == 1 && strings.HasSuffix(existing, "%") {
		return strings.HasPrefix(candidate, strings.TrimSuffix(existing, "%"))
	}

	if !strings.Contains(candidate, "%") {
		return sqlLikeMatch(existing, candidate)
	}

	return false
}

func sqlLikeMatch(pattern, resource string) bool {
	parts := strings.Split(pattern, "%")
	if len(parts) == 1 {
		return pattern == resource
	}

	idx := 0
	for i, part := range parts {
		if part == "" {
			continue
		}

		pos := strings.Index(resource[idx:], part)
		if pos < 0 {
			return false
		}

		if i == 0 && !strings.HasPrefix(pattern, "%") && pos != 0 {
			return false
		}

		idx += pos + len(part)
	}

	if !strings.HasSuffix(pattern, "%") && idx != len(resource) {
		return false
	}

	return true
}
