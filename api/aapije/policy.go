// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package aapije

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/internal/services"
)

func authorizePolicyGrant(ctx context.Context, pc *services.PolicyCheckService, token []byte, action, resource string) error {
	canGrant, err := pc.UserHasAccessViaToken(ctx, token, action, resource)
	if err != nil {
		return err
	}
	if !canGrant {
		return ie.ErrorForbidden
	}
	return nil
}

// AddPolicy adds a new policy
func (ra *RestApi) AddPolicy(w http.ResponseWriter, r *http.Request) {
	// We expect a NewPolicy object in the request body.
	var newPolicy rest.NewPolicy
	if err := ra.decodeJSONBody(w, r, &newPolicy); err != nil {
		ie.SendHTTPError(w, ie.ErrorMalformedRequest)
		return
	}

	groupUUID, err := uuid.Parse(newPolicy.GroupUuid)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorMalformedRequest)
		return
	}

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	// Ensure that the User has the right to create a policy with these access rules
	pc := services.NewPolicyCheckService(db)
	if err := authorizePolicyGrant(r.Context(), pc, []byte(domaintoken.Token), string(newPolicy.Action), newPolicy.Resource); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	srv := services.NewPolicyService(db)

	params := services.NewPolicyParams{
		GroupUuid: groupUUID,
		Priority:  int32(newPolicy.Priority),
		Effect:    string(newPolicy.Effect),
		Action:    string(newPolicy.Action),
		Resource:  newPolicy.Resource,
	}

	policy, err := srv.Add(r.Context(), params)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusCreated, policy)
}

// FindPolicies list all policies
func (ra *RestApi) FindPolicies(w http.ResponseWriter, r *http.Request, p rest.FindPoliciesParams) {
	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	srv := services.NewPolicyService(db)

	params := services.FindAllPoliciesParams{
		Token: []byte(domaintoken.Token),
	}
	if p.Limit != nil {
		i := int64(*p.Limit)
		params.Limit = &i
	}
	if p.Offset != nil {
		i := int64(*p.Offset)
		params.Offset = &i
	}

	if p.GroupUuids != nil {
		groupUUIDs := uuidSliceFromParams(*p.GroupUuids)
		params.GroupUuids = &groupUUIDs
	}

	policies, err := srv.FindAll(r.Context(), params)
	if err != nil {
		// FIXME: log
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusOK, policies)
}

func (ra *RestApi) FindPolicyByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	policyUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	s := services.NewPolicyService(db)
	policy, err := s.FindByUuid(r.Context(), policyUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

// ExplainPolicyDecision explains the winning policy decision for the current token.
func (ra *RestApi) ExplainPolicyDecision(w http.ResponseWriter, r *http.Request, p rest.ExplainPolicyDecisionParams) {
	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	checks := services.NewPolicyCheckService(db)
	decision, err := checks.ExplainUserAccessViaToken(r.Context(), []byte(domaintoken.Token), string(p.Action), p.Resource)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	reply := rest.AuthorizationDecision{
		Action:   rest.AuthorizationDecisionAction(decision.Action),
		Resource: decision.Resource,
		Access:   decision.Access,
	}
	if decision.PolicyUUID != nil {
		v := decision.PolicyUUID.String()
		reply.PolicyUuid = &v
	}
	if decision.GroupUUID != nil {
		v := decision.GroupUUID.String()
		reply.GroupUuid = &v
	}
	if decision.Priority != nil {
		v := *decision.Priority
		reply.Priority = &v
	}
	if decision.Effect != nil {
		v := rest.AuthorizationDecisionEffect(*decision.Effect)
		reply.Effect = &v
	}
	if decision.MatchedPattern != nil {
		v := *decision.MatchedPattern
		reply.MatchedPattern = &v
	}

	writeJSON(w, http.StatusOK, reply)
}

// UpdatePolicyByUuid updates a specific policy by its UUID
func (ra *RestApi) UpdatePolicyByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	// We expect a UpdatePolicy object in the request body.
	var updatePolicy rest.UpdatePolicy
	if err := ra.decodeJSONBody(w, r, &updatePolicy); err != nil {
		ie.SendHTTPError(w, ie.ErrorMalformedRequest)
		return
	}

	policyUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	srv := services.NewPolicyService(db)
	current, err := srv.FindByUuid(r.Context(), policyUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	nextAction := string(current.Action)
	if updatePolicy.Action != nil {
		nextAction = string(*updatePolicy.Action)
	}
	nextResource := current.Resource
	if updatePolicy.Resource != nil {
		nextResource = *updatePolicy.Resource
	}

	pc := services.NewPolicyCheckService(db)
	if err := authorizePolicyGrant(r.Context(), pc, []byte(domaintoken.Token), string(current.Action), current.Resource); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}
	if err := authorizePolicyGrant(r.Context(), pc, []byte(domaintoken.Token), nextAction, nextResource); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	params := services.UpdatePolicyParams{
		Priority: updatePolicy.Priority,
		Effect:   (*string)(updatePolicy.Effect),
		Action:   (*string)(updatePolicy.Action),
		Resource: updatePolicy.Resource,
	}

	if updatePolicy.GroupUuid != nil {
		groupUUID, err := uuid.Parse(*updatePolicy.GroupUuid)
		if err != nil {
			ie.SendHTTPError(w, ie.ErrorMalformedRequest)
			return
		}
		params.GroupUuid = &groupUUID
	}

	_, err = srv.Update(r.Context(), policyUUID, params)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeletePolicyByUuid deletes a specific policy by its UUID
func (ra *RestApi) DeletePolicyByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	policyUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	s := services.NewPolicyService(db)
	current, err := s.FindByUuid(r.Context(), policyUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}
	pc := services.NewPolicyCheckService(db)
	if err := authorizePolicyGrant(r.Context(), pc, []byte(domaintoken.Token), string(current.Action), current.Resource); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}
	_, err = s.Delete(r.Context(), policyUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
