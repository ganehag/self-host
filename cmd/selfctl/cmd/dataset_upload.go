/*
Copyright © 2021 Self-host Authors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	datasetUploadCmdLong = templates.LongDesc(`
		Upload a dataset file through the Self-host API using the multipart dataset workflow.
		The command reads API connection settings from ~/.selfctl/config.yaml by default.
	`)

	datasetUploadCmdExample = templates.Examples(`
		# Upload a new dataset using ~/.selfctl/config.yaml
		selfctl dataset upload ./big.bin --name big-upload-test

		# Override the API target explicitly
		selfctl dataset upload ./big.bin --name big-upload-test --server http://127.0.0.1:18080 --domain test --token root

		# Replace the content of an existing dataset
		selfctl dataset upload ./big.bin --dataset 11111111-1111-1111-1111-111111111111
	`)
)

var (
	datasetUploadServer     string
	datasetUploadDomain     string
	datasetUploadToken      string
	datasetUploadName       string
	datasetUploadFormat     string
	datasetUploadDatasetID  string
	datasetUploadPartSize   int64
	datasetUploadMaxRetries int
)

var datasetUploadCmd = &cobra.Command{
	Use:     "upload FILE",
	Short:   "Upload a dataset file through the Self-host API",
	Long:    datasetUploadCmdLong,
	Example: datasetUploadCmdExample,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetUpload(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetUploadCmd)
	datasetUploadCmd.Flags().StringVar(&datasetUploadName, "name", "", "Dataset name when creating a new dataset")
	datasetUploadCmd.Flags().StringVar(&datasetUploadFormat, "format", "", "Dataset format, defaults to the file extension or 'bin'")
	datasetUploadCmd.Flags().StringVar(&datasetUploadDatasetID, "dataset", "", "Existing dataset UUID to replace")
	datasetUploadCmd.Flags().Int64Var(&datasetUploadPartSize, "part-size", 16*1024*1024, "Multipart upload part size in bytes")
	datasetUploadCmd.Flags().IntVar(&datasetUploadMaxRetries, "retries", 3, "Maximum retries per uploaded part on 429/5xx")
}

func runDatasetUpload(filename string) error {
	cfg, err := resolveAPIConnection(datasetUploadServer, datasetUploadDomain, datasetUploadToken)
	if err != nil {
		return err
	}
	if datasetUploadPartSize <= 0 {
		return fmt.Errorf("part size must be greater than zero")
	}
	if datasetUploadMaxRetries < 0 {
		return fmt.Errorf("retries must be zero or greater")
	}

	finfo, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if finfo.IsDir() {
		return fmt.Errorf("%s is a directory", filename)
	}

	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	datasetID := datasetUploadDatasetID
	if datasetID == "" {
		datasetID, err = createDatasetForUpload(ctx, client, filename)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "created dataset %s\n", datasetID)
	} else if _, err := uuid.Parse(datasetID); err != nil {
		return fmt.Errorf("invalid dataset uuid %q", datasetID)
	}

	initResp, err := client.InitializeDatasetUploadByUuidWithResponse(ctx, rest.UuidParam(uuid.MustParse(datasetID)))
	if err != nil {
		return err
	}
	if initResp.StatusCode() != http.StatusOK || initResp.JSON200 == nil {
		return fmt.Errorf("initialize upload failed: %s", responseError(initResp.StatusCode(), initResp.Body))
	}

	uploadID := string(initResp.JSON200.UploadId)
	if uploadID == "" {
		return fmt.Errorf("initialize upload failed: empty upload id")
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, datasetUploadPartSize)
	partNumber := 1
	for {
		n, readErr := file.Read(buf)
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		if n == 0 {
			break
		}

		partBody := append([]byte(nil), buf[:n]...)
		if err := uploadDatasetPart(ctx, cfg, datasetID, uploadID, partNumber, partBody); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "uploaded part %d (%d bytes)\n", partNumber, n)
		partNumber++

		if readErr == io.EOF {
			break
		}
	}

	assembleResp, err := client.AssembleDatasetPartsByKeyWithResponse(ctx, rest.UuidParam(uuid.MustParse(datasetID)), &rest.AssembleDatasetPartsByKeyParams{
		UploadId: uploadID,
	})
	if err != nil {
		return err
	}
	if assembleResp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("assemble upload failed: %s", responseError(assembleResp.StatusCode(), assembleResp.Body))
	}

	fmt.Fprintf(os.Stdout, "assembled dataset %s\n", datasetID)
	return nil
}

func createDatasetForUpload(ctx context.Context, client *rest.ClientWithResponses, filename string) (string, error) {
	name := strings.TrimSpace(datasetUploadName)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}

	format := strings.TrimSpace(datasetUploadFormat)
	if format == "" {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
		if ext == "" {
			ext = "bin"
		}
		format = ext
	}

	body := rest.AddDatasetsJSONRequestBody{
		Name:   name,
		Format: datasetFormatForCreate(format),
		Content: func() *[]byte {
			b := []byte{}
			return &b
		}(),
	}

	resp, err := client.AddDatasetsWithResponse(ctx, body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusCreated || resp.JSON201 == nil {
		return "", fmt.Errorf("create dataset failed: %s", responseError(resp.StatusCode(), resp.Body))
	}

	return resp.JSON201.Uuid, nil
}

func uploadDatasetPart(ctx context.Context, cfg apiConnectionOptions, datasetID string, uploadID string, partNumber int, body []byte) error {
	baseReq, err := rest.NewUploadDatasetContentByKeyRequest(cfg.Server, rest.UuidParam(uuid.MustParse(datasetID)), &rest.UploadDatasetContentByKeyParams{
		UploadId:   uploadID,
		PartNumber: partNumber,
	})
	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	for attempt := 0; attempt <= datasetUploadMaxRetries; attempt++ {
		req := baseReq.Clone(ctx)
		req.SetBasicAuth(cfg.Domain, cfg.Token)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))

		resp, err := httpClient.Do(req)
		if err != nil {
			if attempt == datasetUploadMaxRetries {
				return err
			}
			time.Sleep(backoffDuration(attempt))
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if shouldRetryUpload(resp.StatusCode) && attempt < datasetUploadMaxRetries {
			time.Sleep(backoffDuration(attempt))
			continue
		}
		return fmt.Errorf("upload part %d failed: %s", partNumber, responseError(resp.StatusCode, respBody))
	}

	return nil
}

func shouldRetryUpload(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

func backoffDuration(attempt int) time.Duration {
	return time.Duration(attempt+1) * 500 * time.Millisecond
}

func responseError(code int, body []byte) string {
	if len(body) == 0 {
		return fmt.Sprintf("status %d", code)
	}

	var msg struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &msg) == nil && msg.Message != "" {
		return fmt.Sprintf("status %d: %s", code, msg.Message)
	}

	return fmt.Sprintf("status %d: %s", code, strings.TrimSpace(string(body)))
}

func datasetFormatForCreate(format string) rest.AddDatasetsJSONBodyFormat {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "csv":
		return rest.AddDatasetsJSONBodyFormatCsv
	case "ini":
		return rest.AddDatasetsJSONBodyFormatIni
	case "json":
		return rest.AddDatasetsJSONBodyFormatJson
	case "toml":
		return rest.AddDatasetsJSONBodyFormatToml
	case "xml":
		return rest.AddDatasetsJSONBodyFormatXml
	case "yaml", "yml":
		return rest.AddDatasetsJSONBodyFormatYaml
	default:
		return rest.AddDatasetsJSONBodyFormatMisc
	}
}
