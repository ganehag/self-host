// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/postgres"
)

// ProgramService represents the repository used for interacting with Program records.
type ProgramService struct {
	q  *postgres.Queries
	db *sql.DB
}

// NewDatasetService instantiates the ProgramService repository.
func NewProgramService(db *sql.DB) *ProgramService {
	if db == nil {
		return nil
	}

	return &ProgramService{
		q:  postgres.New(db),
		db: db,
	}
}

type AddProgramParams struct {
	Name      string
	Type      string
	State     string
	Schedule  string
	Deadline  int
	Language  string
	CreatedBy uuid.UUID
	Tags      []string
}

func (s *ProgramService) AddProgram(ctx context.Context, p AddProgramParams) (*rest.Program, error) {
	params := postgres.CreateProgramParams{
		Name:      p.Name,
		Type:      p.Type,
		State:     p.State,
		Schedule:  p.Schedule,
		Deadline:  int32(p.Deadline),
		Language:  p.Language,
		CreatedBy: p.CreatedBy,
		Tags:      p.Tags,
	}

	program, err := s.q.CreateProgram(ctx, params)
	if err != nil {
		return nil, err
	}

	v := &rest.Program{
		Uuid:     program.Uuid.String(),
		Name:     program.Name,
		Type:     rest.ProgramType(program.Type),
		State:    rest.ProgramState(program.State),
		Schedule: program.Schedule,
		Deadline: int(program.Deadline),
		Language: rest.ProgramLanguage(program.Language),
		Tags:     p.Tags,
	}

	return v, nil
}

type AddCodeRevisionParams struct {
	ProgramUuid uuid.UUID
	CreatedBy   uuid.UUID
	Code        []byte
}

func (s *ProgramService) AddCodeRevision(ctx context.Context, p AddCodeRevisionParams) (*rest.CodeRevision, error) {
	params := postgres.CreateCodeRevisionParams{
		ProgramUuid: p.ProgramUuid,
		CreatedBy:   nullableUUIDValue(p.CreatedBy),
		Code:        p.Code,
	}

	rev, err := s.q.CreateCodeRevision(ctx, params)
	if err != nil {
		return nil, err
	}

	v := &rest.CodeRevision{
		Revision:  int(rev.Revision),
		Created:   rev.Created,
		CreatedBy: nullableUUIDString(rev.CreatedBy),
		Checksum:  string(rev.Checksum),
	}

	if rev.Signed.Valid {
		v.Signed = &rev.Signed.Time
	}
	if rev.SignedBy.Valid {
		u := rev.SignedBy.UUID.String()
		v.SignedBy = &u
	}

	return v, nil
}

func (s *ProgramService) FindAll(ctx context.Context, p FindAllParams) ([]*rest.Program, error) {
	programs := make([]*rest.Program, 0)

	params := postgres.FindProgramsParams{
		Token: p.Token,
	}

	if p.Limit.Value != 0 {
		params.ArgLimit = p.Limit.Value
	}
	if p.Offset.Value != 0 {
		params.ArgOffset = p.Offset.Value
	}

	programsList, err := s.q.FindPrograms(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, t := range programsList {
		program := &rest.Program{
			Uuid:     t.Uuid.String(),
			Name:     t.Name,
			Type:     rest.ProgramType(t.Type),
			State:    rest.ProgramState(t.State),
			Schedule: t.Schedule,
			Deadline: int(t.Deadline),
			Language: rest.ProgramLanguage(t.Language),
			Tags:     t.Tags,
		}

		programs = append(programs, program)
	}

	return programs, nil
}

func (svc *ProgramService) FindByTags(ctx context.Context, p FindByTagsParams) ([]*rest.Program, error) {
	programs := make([]*rest.Program, 0)

	params := postgres.FindProgramsByTagsParams{
		Tags:  p.Tags,
		Token: p.Token,
	}
	if p.Limit.Value != 0 {
		params.ArgLimit = p.Limit.Value
	}
	if p.Offset.Value != 0 {
		params.ArgOffset = p.Offset.Value
	}

	progList, err := svc.q.FindProgramsByTags(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, t := range progList {
		program := &rest.Program{
			Uuid:     t.Uuid.String(),
			Name:     t.Name,
			Type:     rest.ProgramType(t.Type),
			State:    rest.ProgramState(t.State),
			Schedule: t.Schedule,
			Deadline: int(t.Deadline),
			Language: rest.ProgramLanguage(t.Language),
			Tags:     t.Tags,
		}

		programs = append(programs, program)
	}

	return programs, nil
}

func (s *ProgramService) FindProgramByUuid(ctx context.Context, id uuid.UUID) (*rest.Program, error) {
	program, err := s.q.FindProgramByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	v := &rest.Program{
		Uuid:     program.Uuid.String(),
		Name:     program.Name,
		Type:     rest.ProgramType(program.Type),
		State:    rest.ProgramState(program.State),
		Schedule: program.Schedule,
		Deadline: int(program.Deadline),
		Language: rest.ProgramLanguage(program.Language),
		Tags:     program.Tags,
	}

	return v, nil
}

func (s *ProgramService) FindAllCodeRevisions(ctx context.Context, id uuid.UUID) ([]*rest.CodeRevision, error) {
	revisions := make([]*rest.CodeRevision, 0)

	revList, err := s.q.FindProgramCodeRevisions(ctx, id)
	if err != nil {
		return nil, err
	}

	for _, t := range revList {
		rev := &rest.CodeRevision{
			Revision:  int(t.Revision),
			Created:   t.Created,
			CreatedBy: nullableUUIDString(t.CreatedBy),
			Checksum:  string(t.Checksum),
		}

		if t.Signed.Valid {
			v := t.Signed.Time
			rev.Signed = &v
		}
		if t.SignedBy.Valid {
			u := t.SignedBy.UUID.String()
			rev.SignedBy = &u
		}

		revisions = append(revisions, rev)
	}

	return revisions, nil
}

func (s *ProgramService) DiffProgramCodeAtRevisions(ctx context.Context, id uuid.UUID, revA int, revB int) (string, error) {
	var codeA, codeB string

	if revA == -1 {
		cA, err := s.q.GetProgramCodeAtHead(ctx, id)
		if err != nil {
			return "", err
		}
		codeA = string(cA.Code)
		revA = int(cA.Revision)
	} else {
		cA, err := s.q.GetProgramCodeAtRevision(ctx, postgres.GetProgramCodeAtRevisionParams{
			ProgramUuid: id,
			Revision:    int32(revA),
		})
		if err != nil {
			return "", err
		}
		codeA = string(cA)
	}

	if revB == -1 {
		cB, err := s.q.GetProgramCodeAtHead(ctx, id)
		if err != nil {
			return "", err
		}
		codeB = string(cB.Code)
		revB = int(cB.Revision)
	} else {
		cB, err := s.q.GetProgramCodeAtRevision(ctx, postgres.GetProgramCodeAtRevisionParams{
			ProgramUuid: id,
			Revision:    int32(revB),
		})
		if err != nil {
			return "", err
		}
		codeB = string(cB)
	}

	aName := fmt.Sprintf("%v@%v", id.String(), revA)
	bName := fmt.Sprintf("%v@%v", id.String(), revB)
	edits := myers.ComputeEdits(span.URIFromPath(id.String()), codeA, codeB)
	return fmt.Sprint(gotextdiff.ToUnified(aName, bName, codeA, edits)), nil
}

func (s *ProgramService) GetProgramCodeAtHead(ctx context.Context, id uuid.UUID) (string, error) {
	row, err := s.q.GetProgramCodeAtHead(ctx, id)
	if err != nil {
		return "", err
	}
	return string(row.Code), nil
}

func (s *ProgramService) GetSignedProgramCodeAtHead(ctx context.Context, id uuid.UUID) (string, error) {
	row, err := s.q.GetSignedProgramCodeAtHead(ctx, id)
	if err != nil {
		return "", err
	}
	return string(row.Code), nil
}

type UpdateProgramByUuidParams struct {
	Name     *string
	Type     *string
	State    *string
	Schedule *string
	Deadline *int
	Language *string
	Tags     *[]string
}

func (s *ProgramService) UpdateProgramByUuid(ctx context.Context, id uuid.UUID, p UpdateProgramByUuidParams) (int64, error) {
	setName := p.Name != nil
	setType := p.Type != nil
	setState := p.State != nil
	setSchedule := p.Schedule != nil
	setDeadline := p.Deadline != nil
	setLanguage := p.Language != nil
	setTags := p.Tags != nil

	if !(setName || setType || setState || setSchedule || setDeadline || setLanguage || setTags) {
		if _, err := s.q.FindProgramByUUID(ctx, id); err != nil {
			return 0, err
		}
		return 1, nil
	}

	params := postgres.UpdateProgramByUUIDParams{
		Uuid:        id,
		SetName:     setName,
		SetType:     setType,
		SetState:    setState,
		SetSchedule: setSchedule,
		SetDeadline: setDeadline,
		SetLanguage: setLanguage,
		SetTags:     setTags,
	}
	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Type != nil {
		params.Type = *p.Type
	}
	if p.State != nil {
		params.State = *p.State
	}
	if p.Schedule != nil {
		params.Schedule = *p.Schedule
	}
	if p.Deadline != nil {
		params.Deadline = int32(*p.Deadline)
	}
	if p.Language != nil {
		params.Language = *p.Language
	}
	if p.Tags != nil {
		params.Tags = *p.Tags
	}

	count, err := s.q.UpdateProgramByUUID(ctx, params)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, ie.ErrorNotFound
	}

	return count, nil
}

type SignCodeRevisionParams struct {
	ProgramUuid uuid.UUID
	Revision    int
	SignedBy    uuid.UUID
}

func (s *ProgramService) SignCodeRevision(ctx context.Context, p SignCodeRevisionParams) (int64, error) {
	count, err := s.q.SignProgramCodeRevision(ctx, postgres.SignProgramCodeRevisionParams{
		ProgramUuid: p.ProgramUuid,
		Revision:    int32(p.Revision),
		SignedBy:    nullableUUIDValue(p.SignedBy),
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *ProgramService) DeleteProgram(ctx context.Context, id uuid.UUID) (int64, error) {
	count, err := s.q.DeleteProgram(ctx, id)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *ProgramService) DeleteProgramCodeRevision(ctx context.Context, id uuid.UUID, revision int) (int64, error) {
	count, err := s.q.DeleteProgramCodeRevision(ctx, postgres.DeleteProgramCodeRevisionParams{
		ProgramUuid: id,
		Revision:    int32(revision),
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}
