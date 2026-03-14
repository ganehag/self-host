// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	ie "github.com/self-host/self-host/internal/errors"
)

func TestPolicyValidationRejectsMalformedResources(t *testing.T) {
	ctx := context.Background()
	groups := NewGroupService(db)
	policies := NewPolicyService(db)

	group, err := groups.AddGroup(ctx, "policy-validation-group")
	if err != nil {
		t.Fatal(err)
	}

	groupID := uuid.MustParse(group.Uuid)
	cases := []struct {
		name     string
		priority int32
		resource string
	}{
		{name: "negative priority", priority: -1, resource: "timeseries/%"},
		{name: "leading whitespace", priority: 0, resource: " timeseries/%"},
		{name: "embedded whitespace", priority: 0, resource: "timeseries /%"},
		{name: "leading slash", priority: 0, resource: "/timeseries/%"},
		{name: "trailing slash", priority: 0, resource: "timeseries/%/"},
		{name: "empty segment", priority: 0, resource: "timeseries//data"},
		{name: "mixed wildcard segment", priority: 0, resource: "timeseries/%suffix"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := policies.Add(ctx, NewPolicyParams{
				GroupUuid: groupID,
				Priority:  tc.priority,
				Effect:    "allow",
				Action:    "read",
				Resource:  tc.resource,
			})
			if err == nil {
				t.Fatal("expected policy validation error")
			}

			httpErr, ok := err.(*ie.HTTPError)
			if !ok {
				t.Fatalf("expected HTTP error, got %T", err)
			}
			if httpErr.Code != 400 {
				t.Fatalf("expected bad request, got %d", httpErr.Code)
			}
		})
	}
}

func TestPolicyValidationRejectsShadowedPolicies(t *testing.T) {
	ctx := context.Background()
	groups := NewGroupService(db)
	policies := NewPolicyService(db)

	group, err := groups.AddGroup(ctx, "policy-shadowed-group")
	if err != nil {
		t.Fatal(err)
	}

	groupID := uuid.MustParse(group.Uuid)
	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  10,
		Effect:    "allow",
		Action:    "read",
		Resource:  "timeseries/%",
	}); err != nil {
		t.Fatal(err)
	}

	_, err = policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  20,
		Effect:    "deny",
		Action:    "read",
		Resource:  "timeseries/%/data",
	})
	if err == nil {
		t.Fatal("expected shadowed policy to be rejected")
	}

	httpErr, ok := err.(*ie.HTTPError)
	if !ok {
		t.Fatalf("expected HTTP error, got %T", err)
	}
	if httpErr.Code != 400 {
		t.Fatalf("expected bad request, got %d", httpErr.Code)
	}
}

func TestPolicyValidationAllowsSamePriorityDenyOverride(t *testing.T) {
	ctx := context.Background()
	groups := NewGroupService(db)
	policies := NewPolicyService(db)

	group, err := groups.AddGroup(ctx, "policy-deny-override-group")
	if err != nil {
		t.Fatal(err)
	}

	groupID := uuid.MustParse(group.Uuid)
	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  10,
		Effect:    "allow",
		Action:    "read",
		Resource:  "timeseries/%",
	}); err != nil {
		t.Fatal(err)
	}

	policy, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  10,
		Effect:    "deny",
		Action:    "read",
		Resource:  "timeseries/11111111-1111-1111-1111-111111111111/data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if policy == nil {
		t.Fatal("expected policy to be created")
	}
}
