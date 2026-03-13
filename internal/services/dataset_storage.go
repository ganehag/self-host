package services

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

const (
	DatasetStorageBackendInline = "inline"
	DatasetStorageBackendS3     = "s3"
)

type DatasetStorageOptions struct {
	Backend string
	Domain  string
	S3      *DatasetS3Options
}

type DatasetS3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	ForcePathStyle  bool
	KeyPrefix       string
}

type DatasetObjectRef struct {
	Backend string
	Bucket  string
	Key     string
}

type DatasetMultipartPart struct {
	PartNumber int32
	ETag       string
}

type DatasetStorageMetadata struct {
	Backend  string
	Bucket   string
	Key      string
	Size     int64
	Checksum string
}

type DatasetObjectStore interface {
	PutObject(ctx context.Context, ref DatasetObjectRef, body []byte, format string) (DatasetStorageMetadata, error)
	GetObject(ctx context.Context, ref DatasetObjectRef) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, ref DatasetObjectRef) error
	CreateMultipartUpload(ctx context.Context, ref DatasetObjectRef, format string) (string, error)
	UploadPart(ctx context.Context, ref DatasetObjectRef, uploadID string, partNumber int32, body []byte, checksumMD5 string) (string, int64, error)
	CompleteMultipartUpload(ctx context.Context, ref DatasetObjectRef, uploadID string, parts []DatasetMultipartPart) (DatasetStorageMetadata, error)
	AbortMultipartUpload(ctx context.Context, ref DatasetObjectRef, uploadID string) error
}

type DatasetS3Store struct {
	client *s3.Client
	opt    DatasetS3Options
}

func NewDatasetObjectStore(ctx context.Context, opt DatasetStorageOptions) (DatasetObjectStore, error) {
	if strings.TrimSpace(opt.Backend) == "" || opt.Backend == DatasetStorageBackendInline {
		return nil, nil
	}
	if opt.Backend != DatasetStorageBackendS3 {
		return nil, fmt.Errorf("unsupported dataset storage backend %q", opt.Backend)
	}
	if opt.S3 == nil {
		return nil, fmt.Errorf("dataset S3 configuration is required")
	}

	s3opt := *opt.S3
	if s3opt.Region == "" {
		s3opt.Region = "us-east-1"
	}
	if s3opt.Bucket == "" {
		return nil, fmt.Errorf("dataset_storage.s3.bucket is empty")
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s3opt.Region),
	}
	if s3opt.AccessKeyID != "" || s3opt.SecretAccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s3opt.AccessKeyID, s3opt.SecretAccessKey, ""),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = s3opt.ForcePathStyle
		if endpoint := normalizeS3Endpoint(s3opt.Endpoint, s3opt.UseSSL); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &DatasetS3Store{
		client: client,
		opt:    s3opt,
	}, nil
}

func (opt DatasetStorageOptions) ContentRef(datasetUUID string) DatasetObjectRef {
	if opt.Backend != DatasetStorageBackendS3 || opt.S3 == nil {
		return DatasetObjectRef{Backend: DatasetStorageBackendInline}
	}

	return DatasetObjectRef{
		Backend: DatasetStorageBackendS3,
		Bucket:  opt.S3.Bucket,
		Key:     objectKey(opt, datasetUUID, "content"),
	}
}

func (opt DatasetStorageOptions) WriteRef(datasetUUID string) DatasetObjectRef {
	if opt.Backend != DatasetStorageBackendS3 || opt.S3 == nil {
		return DatasetObjectRef{Backend: DatasetStorageBackendInline}
	}

	return DatasetObjectRef{
		Backend: DatasetStorageBackendS3,
		Bucket:  opt.S3.Bucket,
		Key:     objectKey(opt, datasetUUID, path.Join("objects", uuid.NewString())),
	}
}

func (opt DatasetStorageOptions) UploadRef(datasetUUID, uploadID string) DatasetObjectRef {
	if opt.Backend != DatasetStorageBackendS3 || opt.S3 == nil {
		return DatasetObjectRef{Backend: DatasetStorageBackendInline}
	}

	return DatasetObjectRef{
		Backend: DatasetStorageBackendS3,
		Bucket:  opt.S3.Bucket,
		Key:     objectKey(opt, datasetUUID, path.Join("objects", uploadID)),
	}
}

func objectKey(opt DatasetStorageOptions, datasetUUID, suffix string) string {
	prefix := "datasets"
	if opt.S3 != nil && strings.TrimSpace(opt.S3.KeyPrefix) != "" {
		prefix = strings.Trim(opt.S3.KeyPrefix, "/")
	}

	parts := []string{prefix}
	if opt.Domain != "" {
		parts = append(parts, safeObjectKeyPart(opt.Domain))
	}
	parts = append(parts, datasetUUID, suffix)
	return path.Join(parts...)
}

func (s *DatasetS3Store) PutObject(ctx context.Context, ref DatasetObjectRef, body []byte, format string) (DatasetStorageMetadata, error) {
	checksum := sha256.Sum256(body)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(ref.Bucket),
		Key:         aws.String(ref.Key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(datasetContentType(format)),
	})
	if err != nil {
		return DatasetStorageMetadata{}, err
	}

	return DatasetStorageMetadata{
		Backend:  DatasetStorageBackendS3,
		Bucket:   ref.Bucket,
		Key:      ref.Key,
		Size:     int64(len(body)),
		Checksum: hex.EncodeToString(checksum[:]),
	}, nil
}

func (s *DatasetS3Store) GetObject(ctx context.Context, ref DatasetObjectRef) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(ref.Bucket),
		Key:    aws.String(ref.Key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *DatasetS3Store) DeleteObject(ctx context.Context, ref DatasetObjectRef) error {
	if ref.Bucket == "" || ref.Key == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(ref.Bucket),
		Key:    aws.String(ref.Key),
	})
	return err
}

func (s *DatasetS3Store) CreateMultipartUpload(ctx context.Context, ref DatasetObjectRef, format string) (string, error) {
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(ref.Bucket),
		Key:         aws.String(ref.Key),
		ContentType: aws.String(datasetContentType(format)),
	})
	if err != nil {
		return "", err
	}
	if out.UploadId == nil {
		return "", fmt.Errorf("multipart upload did not return an upload ID")
	}
	return *out.UploadId, nil
}

func (s *DatasetS3Store) UploadPart(ctx context.Context, ref DatasetObjectRef, uploadID string, partNumber int32, body []byte, checksumMD5 string) (string, int64, error) {
	md5Hex := strings.ToLower(checksumMD5)
	if md5Hex == "" {
		sum := md5.Sum(body)
		md5Hex = hex.EncodeToString(sum[:])
	}

	md5Raw, err := hex.DecodeString(md5Hex)
	if err != nil {
		return "", 0, err
	}

	out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(ref.Bucket),
		Key:        aws.String(ref.Key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
		Body:       bytes.NewReader(body),
		ContentMD5: aws.String(base64.StdEncoding.EncodeToString(md5Raw)),
	})
	if err != nil {
		return "", 0, err
	}
	if out.ETag == nil {
		return "", 0, fmt.Errorf("multipart upload part did not return an ETag")
	}

	return aws.ToString(out.ETag), int64(len(body)), nil
}

func (s *DatasetS3Store) CompleteMultipartUpload(ctx context.Context, ref DatasetObjectRef, uploadID string, parts []DatasetMultipartPart) (DatasetStorageMetadata, error) {
	completedParts := make([]s3types.CompletedPart, 0, len(parts))
	for _, part := range parts {
		etag := part.ETag
		completedParts = append(completedParts, s3types.CompletedPart{
			ETag:       aws.String(etag),
			PartNumber: aws.Int32(part.PartNumber),
		})
	}

	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(ref.Bucket),
		Key:      aws.String(ref.Key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return DatasetStorageMetadata{}, err
	}

	body, err := s.GetObject(ctx, ref)
	if err != nil {
		return DatasetStorageMetadata{}, err
	}
	defer body.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, body)
	if err != nil {
		return DatasetStorageMetadata{}, err
	}

	return DatasetStorageMetadata{
		Backend:  DatasetStorageBackendS3,
		Bucket:   ref.Bucket,
		Key:      ref.Key,
		Size:     size,
		Checksum: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (s *DatasetS3Store) AbortMultipartUpload(ctx context.Context, ref DatasetObjectRef, uploadID string) error {
	if uploadID == "" {
		return nil
	}
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(ref.Bucket),
		Key:      aws.String(ref.Key),
		UploadId: aws.String(uploadID),
	})
	return err
}

func datasetContentType(format string) string {
	switch strings.ToLower(format) {
	case "csv":
		return "text/csv"
	case "ini":
		return "text/plain; charset=utf-8"
	case "json":
		return "application/json"
	case "toml":
		return "application/toml"
	case "xml":
		return "application/xml"
	case "yaml":
		return "application/yaml"
	default:
		return "application/octet-stream"
	}
}

func normalizeS3Endpoint(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return scheme + "://" + endpoint
}

func safeObjectKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	value = strings.ReplaceAll(value, "/", "_")
	return url.PathEscape(value)
}
