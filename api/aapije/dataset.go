// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package aapije

import (
	"context"
	"database/sql"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	ie "github.com/self-host/self-host/internal/errors"
	"github.com/self-host/self-host/internal/services"
	"github.com/spf13/viper"
)

// AddDatasets adds a new dataset
func (ra *RestApi) AddDatasets(w http.ResponseWriter, r *http.Request) {
	// We expect a NewDataset object in the request body.
	var n rest.NewDataset
	if err := ra.decodeJSONBody(w, r, &n); err != nil {
		ie.SendHTTPError(w, ie.ErrorMalformedRequest)
		return
	}

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	createdBy, err := ra.GetUserUUID(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	s, err := ra.newDatasetService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	params := &services.AddDatasetParams{
		Name:      n.Name,
		Format:    string(n.Format),
		CreatedBy: createdBy,
	}
	if n.Tags != nil {
		params.Tags = *n.Tags
	}

	if n.ThingUuid != nil {
		params.ThingUuid, err = uuid.Parse(*n.ThingUuid)
		if err != nil {
			ie.SendHTTPError(w, ie.ErrorMalformedRequest)
			return
		}
	}

	if n.Content != nil {
		params.Content = *n.Content
	}

	// Add the dataset
	dataset, err := s.AddDataset(r.Context(), params)

	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusCreated, dataset)
}

// FindDatasets lists all datasets
func (ra *RestApi) FindDatasets(w http.ResponseWriter, r *http.Request, p rest.FindDatasetsParams) {
	var err error
	var datasets []*rest.Dataset

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

	svc, err := ra.newDatasetService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	if p.Tags != nil {
		params := services.NewFindByTagsParams(
			[]byte(domaintoken.Token),
			*p.Tags,
			(*int64)(p.Limit),
			(*int64)(p.Offset))

		if params.Limit.Value == 0 {
			params.Limit.Value = 20
		}

		datasets, err = svc.FindByTags(r.Context(), params)
		if err != nil {
			ie.SendHTTPError(w, ie.ParseDBError(err))
			return
		}
	} else {
		params := services.NewFindAllParams(
			[]byte(domaintoken.Token),
			(*int64)(p.Limit),
			(*int64)(p.Offset))

		if params.Limit.Value == 0 {
			params.Limit.Value = 20
		}

		datasets, err = svc.FindAll(r.Context(), params)
		if err != nil {
			ie.SendHTTPError(w, ie.ParseDBError(err))
			return
		}
	}

	writeJSON(w, http.StatusOK, datasets)
}

// FindDatasetByUuid returns a specific dataset by its UUID
func (ra *RestApi) FindDatasetByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	svc, err := ra.newDatasetService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}
	datasets, err := svc.FindDatasetByUuid(r.Context(), datasetUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusOK, datasets)
}

// UpdateDatasetByUuid updates a dataset by its UUID
func (ra *RestApi) UpdateDatasetByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	// We expect a UpdateDataset object in the request body.
	var updDataset rest.UpdateDataset
	if err := ra.decodeJSONBody(w, r, &updDataset); err != nil {
		ie.SendHTTPError(w, ie.ErrorMalformedRequest)
		return
	}

	svc, err := ra.newDatasetService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}
	params := services.UpdateDatasetByUuidParams{
		Name:    updDataset.Name,
		Content: updDataset.Content,
		Tags:    updDataset.Tags,
	}

	if updDataset.ThingUuid != nil {
		thingUUID, err := uuid.Parse(*updDataset.ThingUuid)
		if err != nil {
			ie.SendHTTPError(w, ie.ErrorMalformedRequest)
			return
		}
		params.ThingUuid = &thingUUID
	}

	if updDataset.Format != nil {
		s := string(*updDataset.Format)
		params.Format = &s
	}

	_, err = svc.UpdateDatasetByUuid(r.Context(), datasetUUID, params)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetRawDatasetByUuid gets the "file" content from a dataset by its UUID
func (ra *RestApi) GetRawDatasetByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam, p rest.GetRawDatasetByUuidParams) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	svc, err := ra.newDatasetService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}
	f, err := svc.GetDatasetContentByUuid(r.Context(), datasetUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	if f == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Change Content-Type based on Dataset type
	switch string(f.Format) {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
	case "ini":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "json":
		w.Header().Set("Content-Type", "application/json")
	case "toml":
		w.Header().Set("Content-Type", "application/toml")
	case "xml":
		w.Header().Set("Content-Type", "application/xml")
	case "yaml":
		w.Header().Set("Content-Type", "application/yaml")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("ETag", f.Checksum)

	if p.IfNoneMatch != nil && (string)(*p.IfNoneMatch) == f.Checksum {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if f.Body == nil && len(f.Content) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusOK)
	if f.Body != nil {
		defer f.Body.Close()
		if _, err := io.Copy(w, f.Body); err != nil {
			return
		}
		return
	}
	w.Write(f.Content)
}

// InitializeDatasetUploadByUuid initiates the upload of a larger dataset
func (ra *RestApi) InitializeDatasetUploadByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	userUUID, err := ra.GetUserUUID(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	uploadSvc, err := ra.newDatasetUploadService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	session, err := uploadSvc.CreateUpload(r.Context(), datasetUUID, userUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"uploadId": session.UploadID,
	})
}

// DeleteDatasetUploadByKey cancels a partially completed upload
func (ra *RestApi) DeleteDatasetUploadByKey(w http.ResponseWriter, r *http.Request, id rest.UuidParam, p rest.DeleteDatasetUploadByKeyParams) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	uploadSvc, err := ra.newDatasetUploadService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	if err := uploadSvc.CancelUpload(r.Context(), datasetUUID, p.UploadId); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListDatasetPartsByKey lists all uploaded parts of the dataset
func (ra *RestApi) ListDatasetPartsByKey(w http.ResponseWriter, r *http.Request, id rest.UuidParam, p rest.ListDatasetPartsByKeyParams) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	uploadSvc, err := ra.newDatasetUploadService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	parts, err := uploadSvc.ListParts(r.Context(), datasetUUID, p.UploadId)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uploadId": p.UploadId,
		"parts":    parts,
	})
}

// AssembleDatasetPartsByKey combines all uploaded parts into a new dataset content
func (ra *RestApi) AssembleDatasetPartsByKey(w http.ResponseWriter, r *http.Request, id rest.UuidParam, p rest.AssembleDatasetPartsByKeyParams) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	uploadSvc, err := ra.newDatasetUploadService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	if err := uploadSvc.AssembleUpload(r.Context(), datasetUUID, p.UploadId, p.ContentMD5); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UploadDatasetContentByKey uploads a (max 5MB) part of a new content update to a dataset
func (ra *RestApi) UploadDatasetContentByKey(w http.ResponseWriter, r *http.Request, id rest.UuidParam, p rest.UploadDatasetContentByKeyParams) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	uploadSvc, err := ra.newDatasetUploadService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	maxPartSize := viper.GetInt64("dataset_uploads.max_part_size")
	if maxPartSize > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, maxPartSize)
	}

	if err := uploadSvc.UploadPart(r.Context(), datasetUUID, p.UploadId, p.PartNumber, p.ContentMD5, r.Body); err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "uploaded",
	})
}

// DeleteDatasetByUuid deletes a dataset by its UUID
func (ra *RestApi) DeleteDatasetByUuid(w http.ResponseWriter, r *http.Request, id rest.UuidParam) {
	datasetUUID := uuidFromParam(id)

	db, err := ra.GetDB(r)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	svc, err := ra.newDatasetService(r, db)
	if err != nil {
		ie.SendHTTPError(w, ie.ErrorUndefined)
		return
	}

	count, err := svc.DeleteDataset(r.Context(), datasetUUID)
	if err != nil {
		ie.SendHTTPError(w, ie.ParseDBError(err))
		return
	} else if count == 0 {
		ie.SendHTTPError(w, ie.ErrorNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ra *RestApi) newDatasetUploadService(r *http.Request, db *sql.DB) (*services.DatasetUploadService, error) {
	domaintoken, err := ra.GetDomainToken(r)
	if err != nil {
		return nil, err
	}

	storageOpt, err := ra.datasetStorageOptions(r.Context(), domaintoken.Domain)
	if err != nil {
		return nil, err
	}

	return services.NewDatasetUploadService(db, services.DatasetUploadOptions{
		Domain:       domaintoken.Domain,
		RootDir:      viper.GetString("dataset_uploads.root_dir"),
		MaxPartSize:  viper.GetInt64("dataset_uploads.max_part_size"),
		MaxTotalSize: viper.GetInt64("dataset_uploads.max_total_size"),
		Storage:      storageOpt,
		Store:        mustDatasetObjectStore(r.Context(), storageOpt),
	}), nil
}

func (ra *RestApi) newDatasetService(r *http.Request, db *sql.DB) (*services.DatasetService, error) {
	domaintoken, err := ra.GetDomainToken(r)
	if err != nil {
		return nil, err
	}

	storageOpt, err := ra.datasetStorageOptions(r.Context(), domaintoken.Domain)
	if err != nil {
		return nil, err
	}

	return services.NewDatasetService(db, storageOpt), nil
}

func (ra *RestApi) datasetStorageOptions(ctx context.Context, domain string) (services.DatasetStorageOptions, error) {
	opt := services.DatasetStorageOptions{
		Backend: viper.GetString("dataset_storage.backend"),
		Domain:  domain,
	}
	if opt.Backend != services.DatasetStorageBackendS3 {
		return opt, nil
	}

	opt.S3 = &services.DatasetS3Options{
		Endpoint:        viper.GetString("dataset_storage.s3.endpoint"),
		Region:          viper.GetString("dataset_storage.s3.region"),
		Bucket:          viper.GetString("dataset_storage.s3.bucket"),
		AccessKeyID:     viper.GetString("dataset_storage.s3.access_key_id"),
		SecretAccessKey: viper.GetString("dataset_storage.s3.secret_access_key"),
		UseSSL:          viper.GetBool("dataset_storage.s3.use_ssl"),
		ForcePathStyle:  viper.GetBool("dataset_storage.s3.force_path_style"),
		KeyPrefix:       viper.GetString("dataset_storage.s3.key_prefix"),
	}

	_, err := services.NewDatasetObjectStore(ctx, opt)
	if err != nil {
		return services.DatasetStorageOptions{}, err
	}
	return opt, nil
}

func mustDatasetObjectStore(ctx context.Context, opt services.DatasetStorageOptions) services.DatasetObjectStore {
	store, _ := services.NewDatasetObjectStore(ctx, opt)
	return store
}
