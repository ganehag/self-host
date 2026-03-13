// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/postgres"
)

type DatasetFile struct {
	Format   string
	Content  []byte
	Checksum string
}

// DatasetService represents the repository used for interacting with Dataset records.
type DatasetService struct {
	q  *postgres.Queries
	db *sql.DB
}

// NewDatasetService instantiates the DatasetService repository.
func NewDatasetService(db *sql.DB) *DatasetService {
	if db == nil {
		return nil
	}

	return &DatasetService{
		q:  postgres.New(db),
		db: db,
	}
}

type AddDatasetParams struct {
	Name      string
	Format    string
	Content   []byte
	CreatedBy uuid.UUID
	ThingUuid uuid.UUID
	Tags      []string
}

func (svc *DatasetService) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := svc.q.ExistsDataset(ctx, id)
	if err != nil {
		return false, err
	}

	return found > 0, nil
}

func (svc *DatasetService) AddDataset(ctx context.Context, p *AddDatasetParams) (*rest.Dataset, error) {
	tags := make([]string, 0)
	if p.Tags != nil {
		for _, tag := range p.Tags {
			tags = append(tags, tag)
		}
	}

	params := postgres.CreateDatasetParams{
		Name:      p.Name,
		Content:   p.Content,
		Format:    p.Format,
		CreatedBy: p.CreatedBy,
		BelongsTo: p.ThingUuid,
		Tags:      tags,
	}

	dataset, err := svc.q.CreateDataset(ctx, params)
	if err != nil {
		return nil, err
	}

	v := &rest.Dataset{
		Uuid:      dataset.Uuid.String(),
		Name:      dataset.Name,
		Format:    rest.DatasetFormat(dataset.Format),
		Checksum:  dataset.Checksum,
		Size:      int64(dataset.Size),
		Created:   dataset.Created,
		Updated:   dataset.Updated,
		CreatedBy: nullableUUIDString(dataset.CreatedBy),
		UpdatedBy: nullableUUIDString(dataset.UpdatedBy),
		Tags:      dataset.Tags,
	}

	if dataset.BelongsTo.Valid {
		belongsTo := dataset.BelongsTo.UUID.String()
		v.ThingUuid = &belongsTo
	}

	return v, nil
}

func (svc *DatasetService) FindDatasetByUuid(ctx context.Context, id uuid.UUID) (*rest.Dataset, error) {
	dataset, err := svc.q.FindDatasetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	v := &rest.Dataset{
		Uuid:      dataset.Uuid.String(),
		Name:      dataset.Name,
		Format:    rest.DatasetFormat(dataset.Format),
		Checksum:  dataset.Checksum,
		Size:      int64(dataset.Size),
		Created:   dataset.Created,
		Updated:   dataset.Updated,
		CreatedBy: nullableUUIDString(dataset.CreatedBy),
		UpdatedBy: nullableUUIDString(dataset.UpdatedBy),
		Tags:      dataset.Tags,
	}

	if dataset.BelongsTo.Valid {
		belongsTo := dataset.BelongsTo.UUID.String()
		v.ThingUuid = &belongsTo
	}

	return v, nil
}

func (svc *DatasetService) FindByThing(ctx context.Context, id uuid.UUID) ([]*rest.Dataset, error) {
	datasets := make([]*rest.Dataset, 0)

	datasetsList, err := svc.q.FindDatasetByThing(ctx, nullableUUIDValue(id))
	if err != nil {
		return nil, err
	}

	if len(datasetsList) == 0 {
		count, err := svc.q.ExistsThing(ctx, id)
		if err != nil {
			return nil, err
		} else if count == 0 {
			return nil, ie.ErrorNotFound
		}
	}

	for _, t := range datasetsList {
		dataset := &rest.Dataset{
			Uuid:      t.Uuid.String(),
			Name:      t.Name,
			Format:    rest.DatasetFormat(t.Format),
			Checksum:  t.Checksum,
			Size:      int64(t.Size),
			Created:   t.Created,
			Updated:   t.Updated,
			CreatedBy: nullableUUIDString(t.CreatedBy),
			UpdatedBy: nullableUUIDString(t.UpdatedBy),
			Tags:      t.Tags,
		}

		if t.BelongsTo.Valid {
			v := t.BelongsTo.UUID.String()
			dataset.ThingUuid = &v
		}

		datasets = append(datasets, dataset)
	}

	return datasets, nil
}

func (svc *DatasetService) FindAll(ctx context.Context, p FindAllParams) ([]*rest.Dataset, error) {
	datasets := make([]*rest.Dataset, 0)

	params := postgres.FindDatasetsParams{
		Token: p.Token,
	}

	if p.Limit.Value != 0 {
		params.ArgLimit = p.Limit.Value
	}
	if p.Offset.Value != 0 {
		params.ArgOffset = p.Offset.Value
	}

	datasetsList, err := svc.q.FindDatasets(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, t := range datasetsList {
		dataset := &rest.Dataset{
			Uuid:      t.Uuid.String(),
			Name:      t.Name,
			Format:    rest.DatasetFormat(t.Format),
			Checksum:  t.Checksum,
			Size:      int64(t.Size),
			Created:   t.Created,
			Updated:   t.Updated,
			CreatedBy: nullableUUIDString(t.CreatedBy),
			UpdatedBy: nullableUUIDString(t.UpdatedBy),
			Tags:      t.Tags,
		}

		if t.BelongsTo.Valid {
			v := t.BelongsTo.UUID.String()
			dataset.ThingUuid = &v
		}

		datasets = append(datasets, dataset)
	}

	return datasets, nil
}

func (svc *DatasetService) FindByTags(ctx context.Context, p FindByTagsParams) ([]*rest.Dataset, error) {
	datasets := make([]*rest.Dataset, 0)

	params := postgres.FindDatasetsByTagsParams{
		Tags:  p.Tags,
		Token: p.Token,
	}
	if p.Limit.Value != 0 {
		params.ArgLimit = p.Limit.Value
	}
	if p.Offset.Value != 0 {
		params.ArgOffset = p.Offset.Value
	}

	dsList, err := svc.q.FindDatasetsByTags(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, t := range dsList {
		dataset := &rest.Dataset{
			Uuid:      t.Uuid.String(),
			Name:      t.Name,
			Format:    rest.DatasetFormat(t.Format),
			Checksum:  t.Checksum,
			Size:      int64(t.Size),
			Created:   t.Created,
			Updated:   t.Updated,
			CreatedBy: nullableUUIDString(t.CreatedBy),
			UpdatedBy: nullableUUIDString(t.UpdatedBy),
			Tags:      t.Tags,
		}

		if t.BelongsTo.Valid {
			v := t.BelongsTo.UUID.String()
			dataset.ThingUuid = &v
		}

		datasets = append(datasets, dataset)
	}

	return datasets, nil
}

func (svc *DatasetService) GetDatasetContentByUuid(ctx context.Context, id uuid.UUID) (*DatasetFile, error) {
	content, err := svc.q.GetDatasetContentByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &DatasetFile{
		Format:   content.Format,
		Content:  content.Content,
		Checksum: content.Checksum,
	}, nil
}

type UpdateDatasetByUuidParams struct {
	Content   *[]byte
	Format    *string
	Name      *string
	Tags      *[]string
	ThingUuid *uuid.UUID
}

func (svc *DatasetService) UpdateDatasetByUuid(ctx context.Context, id uuid.UUID, p UpdateDatasetByUuidParams) (int64, error) {
	setName := p.Name != nil
	setFormat := p.Format != nil
	setContent := p.Content != nil
	setTags := p.Tags != nil
	setThingUUID := p.ThingUuid != nil

	if !(setName || setFormat || setContent || setTags || setThingUUID) {
		if _, err := svc.q.FindDatasetByUUID(ctx, id); err != nil {
			return 0, err
		}
		return 1, nil
	}

	params := postgres.UpdateDatasetByUUIDParams{
		Uuid:         id,
		SetName:      setName,
		SetFormat:    setFormat,
		SetContent:   setContent,
		SetTags:      setTags,
		SetThingUuid: setThingUUID,
	}

	if p.Name != nil {
		params.Name = *p.Name
	}
	if p.Format != nil {
		params.Format = *p.Format
	}
	if p.Content != nil {
		params.Content = *p.Content
	}
	if p.Tags != nil {
		params.Tags = *p.Tags
	}
	if p.ThingUuid != nil {
		params.ThingUuid = nullableUUID(p.ThingUuid)
	}

	count, err := svc.q.UpdateDatasetByUUID(ctx, params)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, ie.ErrorNotFound
	}

	return count, nil
}

func (svc *DatasetService) DeleteDataset(ctx context.Context, id uuid.UUID) (int64, error) {
	count, err := svc.q.DeleteDataset(ctx, id)
	if err != nil {
		return 0, err
	}

	return count, nil
}
