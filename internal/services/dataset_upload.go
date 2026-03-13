// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package services

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/postgres"
)

const (
	minUploadPartSizeBytes = 5 * 1024 * 1024
)

var safePathPartRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type DatasetUploadOptions struct {
	Domain       string
	RootDir      string
	MaxPartSize  int64
	MaxTotalSize int64
	Storage      DatasetStorageOptions
	Store        DatasetObjectStore
}

type DatasetUploadPart struct {
	PartNumber  int       `json:"partNumber"`
	Size        int64     `json:"size"`
	ChecksumMD5 string    `json:"checksumMD5"`
	Created     time.Time `json:"created"`
}

type DatasetUploadSession struct {
	UploadID    string    `json:"uploadId"`
	DatasetUUID string    `json:"datasetUuid"`
	Created     time.Time `json:"created"`
}

type DatasetUploadService struct {
	q   *postgres.Queries
	db  *sql.DB
	opt DatasetUploadOptions
}

func NewDatasetUploadService(db *sql.DB, opt DatasetUploadOptions) *DatasetUploadService {
	if db == nil {
		return nil
	}

	return &DatasetUploadService{
		q:   postgres.New(db),
		db:  db,
		opt: opt,
	}
}

func (svc *DatasetUploadService) CreateUpload(ctx context.Context, datasetUUID uuid.UUID, createdBy uuid.UUID) (*DatasetUploadSession, error) {
	ds, err := svc.q.FindDatasetByUUID(ctx, datasetUUID)
	if err != nil {
		return nil, err
	}

	uploadID := uuid.NewString()
	params := postgres.CreateDatasetUploadParams{
		UploadID:       uploadID,
		DatasetUuid:    datasetUUID,
		CreatedBy:      nullableUUIDValue(createdBy),
		StorageBackend: DatasetStorageBackendInline,
	}

	if svc.opt.Store != nil {
		ref := svc.opt.Storage.UploadRef(datasetUUID.String(), uploadID)
		backendUploadID, err := svc.opt.Store.CreateMultipartUpload(ctx, ref, ds.Format)
		if err != nil {
			return nil, err
		}

		params.StorageBackend = DatasetStorageBackendS3
		params.StorageBucket = ref.Bucket
		params.StorageKey = ref.Key
		params.BackendUploadID = backendUploadID
	} else {
		if err := os.MkdirAll(svc.uploadDir(uploadID), 0o700); err != nil {
			return nil, err
		}
	}

	row, err := svc.q.CreateDatasetUpload(ctx, params)
	if err != nil {
		if svc.opt.Store != nil {
			_ = svc.opt.Store.AbortMultipartUpload(ctx, DatasetObjectRef{
				Backend: DatasetStorageBackendS3,
				Bucket:  params.StorageBucket,
				Key:     params.StorageKey,
			}, params.BackendUploadID)
		}
		return nil, err
	}

	return &DatasetUploadSession{
		UploadID:    row.UploadID,
		DatasetUUID: row.DatasetUuid.String(),
		Created:     row.Created,
	}, nil
}

func (svc *DatasetUploadService) UploadPart(ctx context.Context, datasetUUID uuid.UUID, uploadID string, partNumber int, expectedMD5 string, content io.Reader) error {
	upload, err := svc.q.FindDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if upload.DatasetUuid != datasetUUID {
		return ie.ErrorNotFound
	}

	body, actualMD5, err := svc.readUploadBody(content, expectedMD5)
	if err != nil {
		return err
	}

	if svc.opt.Store != nil && upload.StorageBackend == DatasetStorageBackendS3 {
		etag, size, err := svc.opt.Store.UploadPart(ctx, DatasetObjectRef{
			Backend: upload.StorageBackend,
			Bucket:  nullableSQLString(upload.StorageBucket),
			Key:     nullableSQLString(upload.StorageKey),
		}, nullableSQLString(upload.BackendUploadID), int32(partNumber), body, actualMD5)
		if err != nil {
			return err
		}

		_, err = svc.q.UpsertDatasetUploadPart(ctx, postgres.UpsertDatasetUploadPartParams{
			UploadID:    uploadID,
			PartNumber:  int32(partNumber),
			Size:        int32(size),
			ChecksumMd5: actualMD5,
			Etag:        etag,
		})
		return err
	}

	partPath := svc.partPath(uploadID, partNumber)
	if err := os.MkdirAll(filepath.Dir(partPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(partPath, body, 0o600); err != nil {
		return err
	}

	_, err = svc.q.UpsertDatasetUploadPart(ctx, postgres.UpsertDatasetUploadPartParams{
		UploadID:    uploadID,
		PartNumber:  int32(partNumber),
		Size:        int32(len(body)),
		ChecksumMd5: actualMD5,
		Etag:        "",
	})
	return err
}

func (svc *DatasetUploadService) ListParts(ctx context.Context, datasetUUID uuid.UUID, uploadID string) ([]DatasetUploadPart, error) {
	upload, err := svc.q.FindDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if upload.DatasetUuid != datasetUUID {
		return nil, ie.ErrorNotFound
	}

	rows, err := svc.q.FindDatasetUploadParts(ctx, uploadID)
	if err != nil {
		return nil, err
	}

	out := make([]DatasetUploadPart, 0, len(rows))
	for _, row := range rows {
		out = append(out, DatasetUploadPart{
			PartNumber:  int(row.PartNumber),
			Size:        int64(row.Size),
			ChecksumMD5: row.ChecksumMd5,
			Created:     row.Created,
		})
	}

	return out, nil
}

func (svc *DatasetUploadService) CancelUpload(ctx context.Context, datasetUUID uuid.UUID, uploadID string) error {
	upload, err := svc.q.FindDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if upload.DatasetUuid != datasetUUID {
		return ie.ErrorNotFound
	}

	if svc.opt.Store != nil && upload.StorageBackend == DatasetStorageBackendS3 {
		if err := svc.opt.Store.AbortMultipartUpload(ctx, DatasetObjectRef{
			Backend: upload.StorageBackend,
			Bucket:  nullableSQLString(upload.StorageBucket),
			Key:     nullableSQLString(upload.StorageKey),
		}, nullableSQLString(upload.BackendUploadID)); err != nil {
			return err
		}
	} else if err := os.RemoveAll(svc.uploadDir(uploadID)); err != nil {
		return err
	}

	count, err := svc.q.DeleteDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if count == 0 {
		return ie.ErrorNotFound
	}

	return nil
}

func (svc *DatasetUploadService) AssembleUpload(ctx context.Context, datasetUUID uuid.UUID, uploadID string, expectedMD5 string) error {
	upload, err := svc.q.FindDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if upload.DatasetUuid != datasetUUID {
		return ie.ErrorNotFound
	}

	previousRef, _ := svc.q.GetDatasetObjectRefByUUID(ctx, datasetUUID)

	parts, err := svc.q.FindDatasetUploadParts(ctx, uploadID)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return ie.NewBadRequestError(fmt.Errorf("InvalidPart"))
	}

	for i, part := range parts {
		expectedPartNumber := i + 1
		if int(part.PartNumber) != expectedPartNumber {
			return ie.NewBadRequestError(fmt.Errorf("InvalidPartOrder"))
		}
		if i != len(parts)-1 && int64(part.Size) < minUploadPartSizeBytes {
			return ie.NewBadRequestError(fmt.Errorf("EntityTooSmall"))
		}
	}

	var (
		metadata      DatasetStorageMetadata
		inlineContent = []byte{}
	)
	if svc.opt.Store != nil && upload.StorageBackend == DatasetStorageBackendS3 {
		completedParts := make([]DatasetMultipartPart, 0, len(parts))
		for _, part := range parts {
			if !part.Etag.Valid || part.Etag.String == "" {
				return ie.NewBadRequestError(fmt.Errorf("InvalidPart"))
			}
			completedParts = append(completedParts, DatasetMultipartPart{
				PartNumber: part.PartNumber,
				ETag:       part.Etag.String,
			})
		}

		metadata, err = svc.opt.Store.CompleteMultipartUpload(ctx, DatasetObjectRef{
			Backend: upload.StorageBackend,
			Bucket:  nullableSQLString(upload.StorageBucket),
			Key:     nullableSQLString(upload.StorageKey),
		}, nullableSQLString(upload.BackendUploadID), completedParts)
		if err != nil {
			return err
		}
		if expectedMD5 != "" {
			// Preserve the existing API contract, which validates the caller-provided digest of the full assembled payload.
			body, err := svc.opt.Store.GetObject(ctx, DatasetObjectRef{
				Backend: metadata.Backend,
				Bucket:  metadata.Bucket,
				Key:     metadata.Key,
			})
			if err != nil {
				return err
			}
			defer body.Close()

			hasher := md5.New()
			if _, err := io.Copy(hasher, body); err != nil {
				return err
			}
			if actual := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(actual, expectedMD5) {
				return ie.NewBadRequestError(fmt.Errorf("BadDigest"))
			}
		}
	} else {
		inlineContent, metadata, err = svc.assembleInlineUpload(uploadID, parts, expectedMD5)
		if err != nil {
			return err
		}
	}

	count, err := svc.q.UpdateDatasetByUUID(ctx, postgres.UpdateDatasetByUUIDParams{
		Uuid:           datasetUUID,
		SetContent:     true,
		Content:        inlineContent,
		Checksum:       metadata.Checksum,
		Size:           int32(metadata.Size),
		StorageBackend: metadata.Backend,
		StorageBucket:  metadata.Bucket,
		StorageKey:     metadata.Key,
	})
	if err != nil {
		if svc.opt.Store != nil && metadata.Backend == DatasetStorageBackendS3 && metadata.Bucket != "" && metadata.Key != "" {
			_ = svc.opt.Store.DeleteObject(ctx, DatasetObjectRef{
				Backend: metadata.Backend,
				Bucket:  metadata.Bucket,
				Key:     metadata.Key,
			})
		}
		return err
	}
	if count == 0 {
		if svc.opt.Store != nil && metadata.Backend == DatasetStorageBackendS3 && metadata.Bucket != "" && metadata.Key != "" {
			_ = svc.opt.Store.DeleteObject(ctx, DatasetObjectRef{
				Backend: metadata.Backend,
				Bucket:  metadata.Bucket,
				Key:     metadata.Key,
			})
		}
		return ie.ErrorNotFound
	}

	if svc.opt.Store != nil &&
		previousRef.StorageBackend == DatasetStorageBackendS3 &&
		previousRef.StorageBucket.Valid &&
		previousRef.StorageKey.Valid &&
		(previousRef.StorageBucket.String != metadata.Bucket || previousRef.StorageKey.String != metadata.Key) {
		_ = svc.opt.Store.DeleteObject(ctx, DatasetObjectRef{
			Backend: previousRef.StorageBackend,
			Bucket:  previousRef.StorageBucket.String,
			Key:     previousRef.StorageKey.String,
		})
	}

	if _, err := svc.q.DeleteDatasetUploadByID(ctx, uploadID); err != nil {
		return err
	}

	if svc.opt.Store == nil {
		return os.RemoveAll(svc.uploadDir(uploadID))
	}
	return nil
}

func (svc *DatasetUploadService) assembleInlineUpload(uploadID string, parts []postgres.DatasetUploadPart, expectedMD5 string) ([]byte, DatasetStorageMetadata, error) {
	assembledFile, err := os.CreateTemp(svc.uploadDir(uploadID), "assembled-*.bin")
	if err != nil {
		return nil, DatasetStorageMetadata{}, err
	}
	assembledPath := assembledFile.Name()
	defer func() {
		assembledFile.Close()
		_ = os.Remove(assembledPath)
	}()

	md5Hasher := md5.New()
	var totalSize int64
	for _, part := range parts {
		partFile, err := os.Open(svc.partPath(uploadID, int(part.PartNumber)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, DatasetStorageMetadata{}, ie.NewBadRequestError(fmt.Errorf("InvalidPart"))
			}
			return nil, DatasetStorageMetadata{}, err
		}

		written, copyErr := io.Copy(io.MultiWriter(assembledFile, md5Hasher), partFile)
		partFile.Close()
		if copyErr != nil {
			return nil, DatasetStorageMetadata{}, copyErr
		}
		totalSize += written
		if svc.opt.MaxTotalSize > 0 && totalSize > svc.opt.MaxTotalSize {
			return nil, DatasetStorageMetadata{}, ie.NewBadRequestError(fmt.Errorf("EntityTooLarge"))
		}
	}

	actualMD5 := hex.EncodeToString(md5Hasher.Sum(nil))
	if expectedMD5 != "" && !strings.EqualFold(actualMD5, expectedMD5) {
		return nil, DatasetStorageMetadata{}, ie.NewBadRequestError(fmt.Errorf("BadDigest"))
	}

	content, err := os.ReadFile(assembledPath)
	if err != nil {
		return nil, DatasetStorageMetadata{}, err
	}

	return content, DatasetStorageMetadata{
		Backend:  DatasetStorageBackendInline,
		Size:     int64(len(content)),
		Checksum: checksumHex(content),
	}, nil
}

func (svc *DatasetUploadService) readUploadBody(content io.Reader, expectedMD5 string) ([]byte, string, error) {
	reader := content
	if svc.opt.MaxPartSize > 0 {
		reader = io.LimitReader(content, svc.opt.MaxPartSize+1)
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	if svc.opt.MaxPartSize > 0 && int64(len(body)) > svc.opt.MaxPartSize {
		return nil, "", ie.NewBadRequestError(fmt.Errorf("PartTooLarge"))
	}

	sum := md5.Sum(body)
	actualMD5 := hex.EncodeToString(sum[:])
	if expectedMD5 != "" && !strings.EqualFold(actualMD5, expectedMD5) {
		return nil, "", ie.NewBadRequestError(fmt.Errorf("BadDigest"))
	}

	return body, strings.ToLower(actualMD5), nil
}

func (svc *DatasetUploadService) uploadDir(uploadID string) string {
	return filepath.Join(svc.opt.RootDir, safePathPart(svc.opt.Domain), safePathPart(uploadID))
}

func (svc *DatasetUploadService) partPath(uploadID string, partNumber int) string {
	return filepath.Join(svc.uploadDir(uploadID), fmt.Sprintf("%08d.part", partNumber))
}

func safePathPart(value string) string {
	if value == "" {
		return "_"
	}
	if safePathPartRegex.MatchString(value) {
		return value
	}

	return hex.EncodeToString([]byte(value))
}

func nullableString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func nullableSQLString(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
