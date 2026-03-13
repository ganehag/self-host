package services

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type fakeS3Server struct {
	mu       sync.Mutex
	objects  map[string][]byte
	uploads  map[string]*fakeMultipartUpload
	nextID   int
	endpoint *httptest.Server
}

type fakeMultipartUpload struct {
	bucket string
	key    string
	parts  map[int][]byte
	etags  map[int]string
}

func newFakeS3Server(t *testing.T) *fakeS3Server {
	t.Helper()

	f := &fakeS3Server{
		objects: make(map[string][]byte),
		uploads: make(map[string]*fakeMultipartUpload),
	}

	f.endpoint = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.endpoint.Close)
	return f
}

func (f *fakeS3Server) storageOptions() DatasetStorageOptions {
	return DatasetStorageOptions{
		Backend: DatasetStorageBackendS3,
		Domain:  "test",
		S3: &DatasetS3Options{
			Endpoint:        f.endpoint.URL,
			Region:          "us-east-1",
			Bucket:          "datasets",
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			ForcePathStyle:  true,
			KeyPrefix:       "tests",
		},
	}
}

func TestDatasetS3ContentRoundTrip(t *testing.T) {
	s3srv := newFakeS3Server(t)
	svc := NewDatasetService(db, s3srv.storageOptions())

	content := []byte(`{"hello":"world"}`)
	ds, err := svc.AddDataset(context.Background(), &AddDatasetParams{
		Name:      "dataset-s3-roundtrip-" + uuid.NewString(),
		Format:    "json",
		Content:   content,
		CreatedBy: uuid.MustParse("00000000-0000-1000-8000-000000000000"),
	})
	if err != nil {
		t.Fatal(err)
	}

	dsID := uuid.MustParse(ds.Uuid)
	row, err := svc.q.GetDatasetObjectRefByUUID(context.Background(), dsID)
	if err != nil {
		t.Fatal(err)
	}
	if row.StorageBackend != DatasetStorageBackendS3 {
		t.Fatalf("expected s3 storage backend, got %q", row.StorageBackend)
	}
	if !row.StorageBucket.Valid || !row.StorageKey.Valid {
		t.Fatal("expected dataset object reference to be stored")
	}

	file, err := svc.GetDatasetContentByUuid(context.Background(), dsID)
	if err != nil {
		t.Fatal(err)
	}
	if file.Body == nil {
		t.Fatal("expected streamed S3 body")
	}
	defer file.Body.Close()

	got, err := io.ReadAll(file.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("dataset content mismatch: %q != %q", string(got), string(content))
	}
}

func TestDatasetS3MultipartAssemble(t *testing.T) {
	s3srv := newFakeS3Server(t)
	storageOpt := s3srv.storageOptions()
	dsSvc := NewDatasetService(db, storageOpt)
	store, err := NewDatasetObjectStore(context.Background(), storageOpt)
	if err != nil {
		t.Fatal(err)
	}
	uploadSvc := NewDatasetUploadService(db, DatasetUploadOptions{
		Domain:       "test",
		MaxPartSize:  16 * 1024 * 1024,
		MaxTotalSize: 128 * 1024 * 1024,
		Storage:      storageOpt,
		Store:        store,
	})

	ds, err := dsSvc.AddDataset(context.Background(), &AddDatasetParams{
		Name:      "dataset-s3-upload-" + uuid.NewString(),
		Format:    "json",
		Content:   []byte{},
		CreatedBy: uuid.MustParse("00000000-0000-1000-8000-000000000000"),
	})
	if err != nil {
		t.Fatal(err)
	}

	dsID := uuid.MustParse(ds.Uuid)
	session, err := uploadSvc.CreateUpload(context.Background(), dsID, uuid.MustParse("00000000-0000-1000-8000-000000000000"))
	if err != nil {
		t.Fatal(err)
	}

	part1 := strings.Repeat("a", minUploadPartSizeBytes)
	part2 := "tail-data"
	if err := uploadSvc.UploadPart(context.Background(), dsID, session.UploadID, 1, "", strings.NewReader(part1)); err != nil {
		t.Fatal(err)
	}
	if err := uploadSvc.UploadPart(context.Background(), dsID, session.UploadID, 2, "", strings.NewReader(part2)); err != nil {
		t.Fatal(err)
	}

	if err := uploadSvc.AssembleUpload(context.Background(), dsID, session.UploadID, ""); err != nil {
		t.Fatal(err)
	}

	file, err := dsSvc.GetDatasetContentByUuid(context.Background(), dsID)
	if err != nil {
		t.Fatal(err)
	}
	if file.Body == nil {
		t.Fatal("expected assembled dataset to be stored in S3")
	}
	defer file.Body.Close()

	got, err := io.ReadAll(file.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(part1 + part2)
	if !bytes.Equal(got, want) {
		t.Fatalf("assembled dataset mismatch: got %d bytes want %d", len(got), len(want))
	}
}

func (f *fakeS3Server) handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	bucket := parts[0]
	key := ""
	if len(parts) > 1 {
		key = parts[1]
	}

	if r.Method == http.MethodPut && key == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodPost && r.URL.Query().Has("uploads") {
		f.handleCreateMultipart(w, bucket, key)
		return
	}

	if r.Method == http.MethodPut && r.URL.Query().Get("uploadId") != "" {
		f.handleUploadPart(w, r, bucket, key)
		return
	}

	if r.Method == http.MethodPost && r.URL.Query().Get("uploadId") != "" {
		f.handleCompleteMultipart(w, r, bucket, key)
		return
	}

	if r.Method == http.MethodDelete && r.URL.Query().Get("uploadId") != "" {
		f.handleAbortMultipart(w, r)
		return
	}

	switch r.Method {
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.objects[bucket+"/"+key] = body
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		f.mu.Lock()
		body, ok := f.objects[bucket+"/"+key]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	case http.MethodDelete:
		f.mu.Lock()
		delete(f.objects, bucket+"/"+key)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeS3Server) handleCreateMultipart(w http.ResponseWriter, bucket, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	uploadID := "upload-" + strconv.Itoa(f.nextID)
	f.uploads[uploadID] = &fakeMultipartUpload{
		bucket: bucket,
		key:    key,
		parts:  make(map[int][]byte),
		etags:  make(map[int]string),
	}

	type result struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		UploadID string   `xml:"UploadId"`
	}
	_ = xml.NewEncoder(w).Encode(result{
		Bucket:   bucket,
		Key:      key,
		UploadID: uploadID,
	})
}

func (f *fakeS3Server) handleUploadPart(w http.ResponseWriter, r *http.Request, bucket, key string) {
	uploadID := r.URL.Query().Get("uploadId")
	partNumber, _ := strconv.Atoi(r.URL.Query().Get("partNumber"))
	body, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	defer f.mu.Unlock()
	up := f.uploads[uploadID]
	if up == nil || up.bucket != bucket || up.key != key {
		http.NotFound(w, r)
		return
	}
	etag := `"` + checksumHex(body) + `"`
	up.parts[partNumber] = body
	up.etags[partNumber] = etag
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

func (f *fakeS3Server) handleCompleteMultipart(w http.ResponseWriter, r *http.Request, bucket, key string) {
	uploadID := r.URL.Query().Get("uploadId")

	f.mu.Lock()
	defer f.mu.Unlock()
	up := f.uploads[uploadID]
	if up == nil || up.bucket != bucket || up.key != key {
		http.NotFound(w, r)
		return
	}

	var out []byte
	for i := 1; ; i++ {
		part, ok := up.parts[i]
		if !ok {
			break
		}
		out = append(out, part...)
	}
	f.objects[bucket+"/"+key] = out
	delete(f.uploads, uploadID)

	type result struct {
		XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
		Bucket  string   `xml:"Bucket"`
		Key     string   `xml:"Key"`
		ETag    string   `xml:"ETag"`
	}
	_ = xml.NewEncoder(w).Encode(result{
		Bucket: bucket,
		Key:    key,
		ETag:   `"` + checksumHex(out) + `"`,
	})
}

func (f *fakeS3Server) handleAbortMultipart(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("uploadId")
	f.mu.Lock()
	delete(f.uploads, uploadID)
	f.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
