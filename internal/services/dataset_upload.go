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
	if _, err := svc.q.FindDatasetByUUID(ctx, datasetUUID); err != nil {
		return nil, err
	}

	uploadID := uuid.NewString()
	row, err := svc.q.CreateDatasetUpload(ctx, postgres.CreateDatasetUploadParams{
		UploadID:    uploadID,
		DatasetUuid: datasetUUID,
		CreatedBy:   nullableUUIDValue(createdBy),
	})
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(svc.uploadDir(uploadID), 0o700); err != nil {
		_, _ = svc.q.DeleteDatasetUploadByID(ctx, uploadID)
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

	partPath := svc.partPath(uploadID, partNumber)
	if err := os.MkdirAll(filepath.Dir(partPath), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(partPath), "part-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	hasher := md5.New()
	reader := content
	if svc.opt.MaxPartSize > 0 {
		reader = io.LimitReader(content, svc.opt.MaxPartSize+1)
	}

	written, err := io.Copy(io.MultiWriter(tmp, hasher), reader)
	if err != nil {
		return err
	}
	if svc.opt.MaxPartSize > 0 && written > svc.opt.MaxPartSize {
		return ie.NewBadRequestError(fmt.Errorf("PartTooLarge"))
	}

	actualMD5 := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualMD5, expectedMD5) {
		return ie.NewBadRequestError(fmt.Errorf("BadDigest"))
	}

	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, partPath); err != nil {
		return err
	}

	_, err = svc.q.UpsertDatasetUploadPart(ctx, postgres.UpsertDatasetUploadPartParams{
		UploadID:    uploadID,
		PartNumber:  int32(partNumber),
		Size:        int32(written),
		ChecksumMd5: strings.ToLower(actualMD5),
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

	count, err := svc.q.DeleteDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if count == 0 {
		return ie.ErrorNotFound
	}

	return os.RemoveAll(svc.uploadDir(uploadID))
}

func (svc *DatasetUploadService) AssembleUpload(ctx context.Context, datasetUUID uuid.UUID, uploadID string, expectedMD5 string) error {
	upload, err := svc.q.FindDatasetUploadByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if upload.DatasetUuid != datasetUUID {
		return ie.ErrorNotFound
	}

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

	assembledFile, err := os.CreateTemp(svc.uploadDir(uploadID), "assembled-*.bin")
	if err != nil {
		return err
	}
	assembledPath := assembledFile.Name()
	defer func() {
		assembledFile.Close()
		_ = os.Remove(assembledPath)
	}()

	hasher := md5.New()
	var totalSize int64
	for _, part := range parts {
		partFile, err := os.Open(svc.partPath(uploadID, int(part.PartNumber)))
		if err != nil {
			if os.IsNotExist(err) {
				return ie.NewBadRequestError(fmt.Errorf("InvalidPart"))
			}
			return err
		}

		written, copyErr := io.Copy(io.MultiWriter(assembledFile, hasher), partFile)
		partFile.Close()
		if copyErr != nil {
			return copyErr
		}
		totalSize += written
		if svc.opt.MaxTotalSize > 0 && totalSize > svc.opt.MaxTotalSize {
			return ie.NewBadRequestError(fmt.Errorf("EntityTooLarge"))
		}
	}

	actualMD5 := hex.EncodeToString(hasher.Sum(nil))
	if expectedMD5 != "" && !strings.EqualFold(actualMD5, expectedMD5) {
		return ie.NewBadRequestError(fmt.Errorf("BadDigest"))
	}

	if err := assembledFile.Close(); err != nil {
		return err
	}

	content, err := os.ReadFile(assembledPath)
	if err != nil {
		return err
	}

	dsSvc := NewDatasetService(svc.db)
	if _, err := dsSvc.UpdateDatasetByUuid(ctx, datasetUUID, UpdateDatasetByUuidParams{
		Content: &content,
	}); err != nil {
		return err
	}

	if _, err := svc.q.DeleteDatasetUploadByID(ctx, uploadID); err != nil {
		return err
	}

	return os.RemoveAll(svc.uploadDir(uploadID))
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
