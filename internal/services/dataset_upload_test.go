package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestDatasetUploadCreateCleansLocalDirOnDBFailure(t *testing.T) {
	ctx := context.Background()
	datasets, err := NewDatasetService(db)
	if err != nil {
		t.Fatal(err)
	}

	rootDir := t.TempDir()
	uploadSvc := NewDatasetUploadService(db, DatasetUploadOptions{
		Domain:       "test",
		RootDir:      rootDir,
		MaxPartSize:  16 * 1024 * 1024,
		MaxTotalSize: 128 * 1024 * 1024,
	})

	ds, err := datasets.AddDataset(ctx, &AddDatasetParams{
		Name:      "dataset-upload-cleanup-" + uuid.NewString(),
		Format:    "json",
		Content:   []byte("{}"),
		CreatedBy: uuid.MustParse("00000000-0000-1000-8000-000000000000"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = uploadSvc.CreateUpload(ctx, uuid.MustParse(ds.Uuid), uuid.Nil)
	if err == nil {
		t.Fatal("expected upload creation to fail for nil created_by")
	}

	entries, err := os.ReadDir(filepath.Join(rootDir, "test"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected upload root to be cleaned up, found %d leftover entries", len(entries))
	}
}
