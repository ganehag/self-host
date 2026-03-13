// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"database/sql"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

var r *rand.Rand // Rand for this package.

func init() {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
}

const (
	// FormatOfStringForUUIDOfRFC4122 is an optional predefined format for UUID v1-v5 as specified by RFC4122
	FormatOfStringForUUIDOfRFC4122 = `^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
)

type DomainToken struct {
	Domain string
	Token  string
}

type DomainInfo struct {
	Dom *DomainToken
	Db  *sql.DB
}

type DataPoint struct {
	Value     float64   `json:"v"`
	Timestamp time.Time `json:"ts"`
}

type PaginationLimit struct {
	Value int64
}

func (pl *PaginationLimit) Scan(x *int64) {
	if x == nil {
		pl.Value = 0
		return
	}
	pl.Value = *x
}

type PaginationOffset struct {
	Value int64
}

func (pl *PaginationOffset) Scan(x *int64) {
	if x == nil {
		pl.Value = 0
		return
	}
	pl.Value = *x
}

type PaginationParams struct {
	Limit  PaginationLimit
	Offset PaginationOffset
}

type FindByTagsParams struct {
	PaginationParams
	Token []byte
	Tags  []string
}

type FindByUuidParams struct {
	PaginationParams
	Token []byte
	Uuid  uuid.UUID
}

type FindAllParams struct {
	PaginationParams
	Token []byte
}

func NewFindByTagsParams(token []byte, tags []string, limit *int64, offset *int64) FindByTagsParams {
	p := FindByTagsParams{
		Token: token,
		PaginationParams: PaginationParams{
			Limit:  PaginationLimit{},
			Offset: PaginationOffset{},
		},
		Tags: tags,
	}

	p.Limit.Scan(limit)
	p.Offset.Scan(offset)

	return p
}

func NewFindByUuidParams(token []byte, id uuid.UUID, limit *int64, offset *int64) FindByUuidParams {
	p := FindByUuidParams{
		Token: token,
		PaginationParams: PaginationParams{
			Limit:  PaginationLimit{},
			Offset: PaginationOffset{},
		},
		Uuid: id,
	}

	p.Limit.Scan(limit)
	p.Offset.Scan(offset)

	return p
}

func NewFindAllParams(token []byte, limit *int64, offset *int64) FindAllParams {
	p := FindAllParams{
		Token: token,
		PaginationParams: PaginationParams{
			Limit:  PaginationLimit{},
			Offset: PaginationOffset{},
		},
	}

	p.Limit.Scan(limit)
	p.Offset.Scan(offset)

	return p
}

func RandomString(strlen int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return string(result)
}

func nullableUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}

	return uuid.NullUUID{UUID: *id, Valid: true}
}

func nullableUUIDValue(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: true}
}

func nullableUUIDString(id uuid.NullUUID) string {
	if !id.Valid {
		return ""
	}

	return id.UUID.String()
}
