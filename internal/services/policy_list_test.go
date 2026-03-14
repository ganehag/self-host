package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestFindPoliciesUsesSamePrioritySemanticsAsRuntimeChecks(t *testing.T) {
	ctx := context.Background()
	users := NewUserService(db)
	groups := NewGroupService(db)
	policies := NewPolicyService(db)

	user, err := users.AddUser(ctx, "policy-list-user")
	if err != nil {
		t.Fatal(err)
	}

	viewerGroup, err := groups.AddGroup(ctx, "policy-list-viewer-group")
	if err != nil {
		t.Fatal(err)
	}

	targetGroup, err := groups.AddGroup(ctx, "policy-list-target-group")
	if err != nil {
		t.Fatal(err)
	}

	userID := uuid.MustParse(user.Uuid)
	viewerGroupID := uuid.MustParse(viewerGroup.Uuid)
	targetGroupID := uuid.MustParse(targetGroup.Uuid)

	if err := users.AddUserToGroups(ctx, userID, []uuid.UUID{viewerGroupID}); err != nil {
		t.Fatal(err)
	}

	token, err := users.AddTokenToUser(ctx, userID, "policy-list-token")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: viewerGroupID,
		Priority:  100,
		Effect:    "deny",
		Action:    "read",
		Resource:  "policies/%",
	}); err != nil {
		t.Fatal(err)
	}

	target, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: targetGroupID,
		Priority:  0,
		Effect:    "allow",
		Action:    "read",
		Resource:  "timeseries/%",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: viewerGroupID,
		Priority:  50,
		Effect:    "allow",
		Action:    "read",
		Resource:  "policies/" + target.Uuid,
	}); err != nil {
		t.Fatal(err)
	}

	list, err := policies.FindAll(ctx, FindAllPoliciesParams{
		Token: []byte(token.Secret),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, policy := range list {
		if policy.Uuid == target.Uuid {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected higher-priority specific allow to keep policy visible despite later broad deny")
	}

	if _, err := policies.Add(ctx, NewPolicyParams{
		GroupUuid: viewerGroupID,
		Priority:  10,
		Effect:    "deny",
		Action:    "read",
		Resource:  "policies/" + target.Uuid,
	}); err != nil {
		t.Fatal(err)
	}

	list, err = policies.FindAll(ctx, FindAllPoliciesParams{
		Token: []byte(token.Secret),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, policy := range list {
		if policy.Uuid == target.Uuid {
			t.Fatal("expected higher-priority deny to hide policy from listing")
		}
	}
}
