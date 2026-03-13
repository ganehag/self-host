// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

const rootUserUUID = "00000000-0000-1000-8000-000000000000"

func TestPolicyCheckRespectsPriorityForSingleAndMany(t *testing.T) {
	ctx := context.Background()
	users := NewUserService(db)
	groups := NewGroupService(db)
	policies := NewPolicyService(db)
	checks := NewPolicyCheckService(db)

	user, err := users.AddUser(ctx, "policy-priority-user")
	if err != nil {
		t.Fatal(err)
	}

	group, err := groups.AddGroup(ctx, "policy-priority-group")
	if err != nil {
		t.Fatal(err)
	}

	groupID := uuid.MustParse(group.Uuid)
	userID := uuid.MustParse(user.Uuid)
	if err := users.AddUserToGroups(ctx, userID, []uuid.UUID{groupID}); err != nil {
		t.Fatal(err)
	}

	token, err := users.AddTokenToUser(ctx, userID, "policy-priority-token")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  50,
		Effect:    "allow",
		Action:    "read",
		Resource:  "timeseries/%/data",
	}); err != nil {
		t.Fatal(err)
	}

	deniedResource := "timeseries/11111111-1111-1111-1111-111111111111/data"
	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: groupID,
		Priority:  10,
		Effect:    "deny",
		Action:    "read",
		Resource:  deniedResource,
	}); err != nil {
		t.Fatal(err)
	}

	allowed, err := checks.UserHasAccessViaToken(ctx, []byte(token.Secret), "read", deniedResource)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected higher-priority deny to win for single-resource access")
	}

	allowed, err = checks.UserHasManyAccessViaToken(ctx, []byte(token.Secret), "read", []string{deniedResource})
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected higher-priority deny to win for multi-resource access")
	}

	allowed, err = checks.UserHasManyAccessViaToken(ctx, []byte(token.Secret), "read", []string{
		"timeseries/22222222-2222-2222-2222-222222222222/data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected broad allow policy to grant unrelated timeseries read access")
	}
}
