// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 --config=rest/types.cfg.yaml rest/openapiv3.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 --config=rest/server.cfg.yaml rest/openapiv3.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 --config=rest/client.cfg.yaml rest/openapiv3.yaml

package aapije

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/self-host/self-host/internal/services"
)

// Error struct
type Error struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

// RestApi is the main REST API structure
type RestApi struct{}

const defaultJSONBodyLimit = 1 << 20

// NewRestApi creates a new instance of the REST API
func NewRestApi() *RestApi {
	return &RestApi{}
}

// GetDB gets the DB handle from the request context
func (ra *RestApi) GetDB(r *http.Request) (*sql.DB, error) {
	db, ok := r.Context().Value("db").(*sql.DB)
	if ok == false {
		return nil, errors.New("database handle missing from context")
	}
	return db, nil
}

func (ra *RestApi) GetUserUUID(r *http.Request) (uuid.UUID, error) {
	userUUID, ok := r.Context().Value("user_uuid").(uuid.UUID)
	if ok == false {
		return uuid.Nil, errors.New("authenticated user uuid missing from context")
	}

	return userUUID, nil
}

func (ra *RestApi) GetDomainToken(r *http.Request) (*services.DomainToken, error) {
	domaintoken, ok := r.Context().Value("domaintoken").(*services.DomainToken)
	if ok == false {
		return nil, errors.New("domain token missing from context")
	}

	return domaintoken, nil
}

func (ra *RestApi) decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	return ra.decodeJSONBodyWithLimit(w, r, dst, defaultJSONBodyLimit)
}

func (ra *RestApi) decodeJSONBodyWithLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func uuidFromParam(id rest.UuidParam) uuid.UUID {
	return uuid.UUID(id)
}

func uuidSliceFromParams(ids []rest.UUID) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out
}
