// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestExplainUserAccessViaTokenReturnsWinningPolicy(t *testing.T) {
	ctx := context.Background()
	users := NewUserService(db)
	groups := NewGroupService(db)
	policies := NewPolicyService(db)
	checks := NewPolicyCheckService(db)

	user, err := users.AddUser(ctx, "policy-explain-user")
	if err != nil {
		t.Fatal(err)
	}

	group, err := groups.AddGroup(ctx, "policy-explain-group")
	if err != nil {
		t.Fatal(err)
	}

	groupID := uuid.MustParse(group.Uuid)
	userID := uuid.MustParse(user.Uuid)
	if err := users.AddUserToGroups(ctx, userID, []uuid.UUID{groupID}); err != nil {
		t.Fatal(err)
	}

	token, err := users.AddTokenToUser(ctx, userID, "policy-explain-token")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  50,
		Effect:    "allow",
		Action:    "read",
		Resource:  "timeseries/%",
	}); err != nil {
		t.Fatal(err)
	}

	denyPolicy, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  10,
		Effect:    "deny",
		Action:    "read",
		Resource:  "timeseries/11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, err := checks.ExplainUserAccessViaToken(ctx, []byte(token.Secret), "read", "timeseries/11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}

	if decision.Access {
		t.Fatal("expected deny policy to win")
	}
	if decision.PolicyUUID == nil || decision.PolicyUUID.String() != denyPolicy.Uuid {
		t.Fatal("expected winning policy UUID to be reported")
	}
	if decision.GroupUUID == nil || decision.GroupUUID.String() != group.Uuid {
		t.Fatal("expected winning group UUID to be reported")
	}
	if decision.Priority == nil || *decision.Priority != 10 {
		t.Fatal("expected winning priority to be reported")
	}
	if decision.Effect == nil || *decision.Effect != "deny" {
		t.Fatal("expected winning effect to be reported")
	}
	if decision.MatchedPattern == nil || *decision.MatchedPattern != "timeseries/11111111-1111-1111-1111-111111111111" {
		t.Fatal("expected matched policy pattern to be reported")
	}
}

func TestPolicyCheckCacheIsRequestLocal(t *testing.T) {
	ctx := context.Background()
	users := NewUserService(db)
	groups := NewGroupService(db)
	policies := NewPolicyService(db)
	checks := NewPolicyCheckService(db)

	user, err := users.AddUser(ctx, "policy-cache-user")
	if err != nil {
		t.Fatal(err)
	}

	group, err := groups.AddGroup(ctx, "policy-cache-group")
	if err != nil {
		t.Fatal(err)
	}

	groupID := uuid.MustParse(group.Uuid)
	userID := uuid.MustParse(user.Uuid)
	if err := users.AddUserToGroups(ctx, userID, []uuid.UUID{groupID}); err != nil {
		t.Fatal(err)
	}

	token, err := users.AddTokenToUser(ctx, userID, "policy-cache-token")
	if err != nil {
		t.Fatal(err)
	}

	policy, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  10,
		Effect:    "allow",
		Action:    "read",
		Resource:  "things/%",
	})
	if err != nil {
		t.Fatal(err)
	}

	requestCtx := WithPolicyCheckCache(ctx)
	allowed, err := checks.UserHasAccessViaToken(requestCtx, []byte(token.Secret), "read", "things/abc")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected policy to allow access")
	}

	if _, err := policies.Delete(ctx, uuid.MustParse(policy.Uuid)); err != nil {
		t.Fatal(err)
	}

	allowed, err = checks.UserHasAccessViaToken(requestCtx, []byte(token.Secret), "read", "things/abc")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected cached decision to survive within one request context")
	}

	allowed, err = checks.UserHasAccessViaToken(context.Background(), []byte(token.Secret), "read", "things/abc")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected fresh context to observe policy deletion")
	}
}
