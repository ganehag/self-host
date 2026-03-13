# SeaweedFS Dataset Testing

To verify that the dataset backend works against a real SeaweedFS S3 gateway locally, use:

```bash
./test/seaweedfs/run-local.sh
```

This workflow starts:

- PostgreSQL
- SeaweedFS with the S3 gateway enabled
- `aapije` configured with `dataset_storage.backend: s3`

It then runs an end-to-end smoke test that:

1. creates a dataset with direct content upload
2. downloads the dataset and verifies the payload
3. initializes a multipart dataset upload
4. uploads two parts
5. assembles the upload
6. downloads the assembled dataset and verifies the payload
7. checks that the dataset object exists in SeaweedFS
8. deletes the dataset
9. checks that the dataset object is removed from SeaweedFS

The local stack uses:

- API base URL: `http://127.0.0.1:18080`
- SeaweedFS S3 URL: `http://127.0.0.1:8333`
- PostgreSQL: `127.0.0.1:55432`
- API auth: `test` / `root`
- SeaweedFS S3 credentials: `selfhost` / `selfhost-secret`

Teardown:

```bash
./test/seaweedfs/down-local.sh
```

The compose files and API config used by the smoke test are under `test/seaweedfs/docker/`.
