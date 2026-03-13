// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/postgres"
)

type DatasetFile struct {
	Format   string
	Content  []byte
	Body     io.ReadCloser
	Checksum string
	Size     int64
}

// DatasetService represents the repository used for interacting with Dataset records.
type DatasetService struct {
	q     *postgres.Queries
	db    *sql.DB
	store DatasetObjectStore
	opt   DatasetStorageOptions
}

// NewDatasetService instantiates the DatasetService repository.
func NewDatasetService(db *sql.DB, opts ...DatasetStorageOptions) *DatasetService {
	if db == nil {
		return nil
	}

	var opt DatasetStorageOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	store, _ := NewDatasetObjectStore(context.Background(), opt)

	return &DatasetService{
		q:     postgres.New(db),
		db:    db,
		store: store,
		opt:   opt,
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
	datasetUUID := uuid.New()
	tags := make([]string, 0)
	if p.Tags != nil {
		for _, tag := range p.Tags {
			tags = append(tags, tag)
		}
	}

	content := p.Content
	checksum := checksumHex(content)
	size := len(content)
	storageBackend := DatasetStorageBackendInline
	storageBucket := ""
	storageKey := ""
	inlineContent := content

	if svc.store != nil && len(content) > 0 {
		ref := svc.opt.WriteRef(datasetUUID.String())
		meta, err := svc.store.PutObject(ctx, ref, content, p.Format)
		if err != nil {
			return nil, err
		}
		checksum = meta.Checksum
		size = int(meta.Size)
		storageBackend = meta.Backend
		storageBucket = meta.Bucket
		storageKey = meta.Key
		inlineContent = []byte{}
	}

	params := postgres.CreateDatasetParams{
		Uuid:           datasetUUID,
		Name:           p.Name,
		Content:        inlineContent,
		Checksum:       checksum,
		Size:           int32(size),
		Format:         p.Format,
		CreatedBy:      p.CreatedBy,
		BelongsTo:      p.ThingUuid,
		Tags:           tags,
		StorageBackend: storageBackend,
		StorageBucket:  storageBucket,
		StorageKey:     storageKey,
	}

	dataset, err := svc.q.CreateDataset(ctx, params)
	if err != nil {
		if svc.store != nil && storageBackend == DatasetStorageBackendS3 {
			_ = svc.store.DeleteObject(ctx, DatasetObjectRef{
				Backend: storageBackend,
				Bucket:  storageBucket,
				Key:     storageKey,
			})
		}
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

	if content.StorageBackend == DatasetStorageBackendS3 && content.StorageBucket.Valid && content.StorageKey.Valid {
		if svc.store == nil {
			return nil, ie.ErrorUndefined
		}
		body, err := svc.store.GetObject(ctx, DatasetObjectRef{
			Backend: content.StorageBackend,
			Bucket:  content.StorageBucket.String,
			Key:     content.StorageKey.String,
		})
		if err != nil {
			return nil, err
		}

		return &DatasetFile{
			Format:   content.Format,
			Body:     body,
			Checksum: content.Checksum,
			Size:     int64(content.Size),
		}, nil
	}

	return &DatasetFile{
		Format:   content.Format,
		Content:  content.Content,
		Checksum: content.Checksum,
		Size:     int64(content.Size),
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

	var previousRef postgres.GetDatasetObjectRefByUUIDRow
	if setContent {
		previousRef, _ = svc.q.GetDatasetObjectRefByUUID(ctx, id)
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
		content := *p.Content
		params.Content = content
		params.Checksum = checksumHex(content)
		params.Size = int32(len(content))
		params.StorageBackend = DatasetStorageBackendInline
		params.StorageBucket = ""
		params.StorageKey = ""

		if svc.store != nil && len(content) > 0 {
			format := params.Format
			if !setFormat {
				current, err := svc.q.FindDatasetByUUID(ctx, id)
				if err != nil {
					return 0, err
				}
				format = current.Format
			}

			ref := svc.opt.WriteRef(id.String())
			meta, err := svc.store.PutObject(ctx, ref, content, format)
			if err != nil {
				return 0, err
			}

			params.Content = []byte{}
			params.Checksum = meta.Checksum
			params.Size = int32(meta.Size)
			params.StorageBackend = meta.Backend
			params.StorageBucket = meta.Bucket
			params.StorageKey = meta.Key
		}
	}
	if p.Tags != nil {
		params.Tags = *p.Tags
	}
	if p.ThingUuid != nil {
		params.ThingUuid = nullableUUID(p.ThingUuid)
	}

	count, err := svc.q.UpdateDatasetByUUID(ctx, params)
	if err != nil {
		if svc.store != nil && params.StorageBackend == DatasetStorageBackendS3 && params.StorageBucket != "" && params.StorageKey != "" {
			_ = svc.store.DeleteObject(ctx, DatasetObjectRef{
				Backend: params.StorageBackend,
				Bucket:  params.StorageBucket,
				Key:     params.StorageKey,
			})
		}
		return 0, err
	}
	if count == 0 {
		if svc.store != nil && params.StorageBackend == DatasetStorageBackendS3 && params.StorageBucket != "" && params.StorageKey != "" {
			_ = svc.store.DeleteObject(ctx, DatasetObjectRef{
				Backend: params.StorageBackend,
				Bucket:  params.StorageBucket,
				Key:     params.StorageKey,
			})
		}
		return 0, ie.ErrorNotFound
	}

	if svc.store != nil &&
		previousRef.StorageBackend == DatasetStorageBackendS3 &&
		previousRef.StorageBucket.Valid &&
		previousRef.StorageKey.Valid &&
		(params.StorageBackend != DatasetStorageBackendS3 ||
			previousRef.StorageBucket.String != params.StorageBucket ||
			previousRef.StorageKey.String != params.StorageKey) {
		_ = svc.store.DeleteObject(ctx, DatasetObjectRef{
			Backend: previousRef.StorageBackend,
			Bucket:  previousRef.StorageBucket.String,
			Key:     previousRef.StorageKey.String,
		})
	}

	return count, nil
}

func (svc *DatasetService) DeleteDataset(ctx context.Context, id uuid.UUID) (int64, error) {
	refRow, err := svc.q.GetDatasetObjectRefByUUID(ctx, id)
	if err != nil {
		return 0, err
	}

	count, err := svc.q.DeleteDataset(ctx, id)
	if err != nil {
		return 0, err
	}

	if svc.store != nil && refRow.StorageBackend == DatasetStorageBackendS3 && refRow.StorageBucket.Valid && refRow.StorageKey.Valid {
		_ = svc.store.DeleteObject(ctx, DatasetObjectRef{
			Backend: refRow.StorageBackend,
			Bucket:  refRow.StorageBucket.String,
			Key:     refRow.StorageKey.String,
		})
	}

	return count, nil
}

func checksumHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
