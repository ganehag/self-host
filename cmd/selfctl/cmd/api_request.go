package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var apiRequestBody string

var apiRequestCmd = &cobra.Command{
	Use:   "request METHOD PATH",
	Short: "Perform a raw HTTP request with the configured API credentials",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAPIRequest(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	apiCmd.AddCommand(apiRequestCmd)
	apiRequestCmd.Flags().StringVar(&apiRequestBody, "body", "", "Raw request body to send")
}

func runAPIRequest(method, requestPath string) error {
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	url := strings.TrimRight(cfg.Server, "/") + "/" + strings.TrimLeft(requestPath, "/")
	req, err := http.NewRequestWithContext(context.Background(), strings.ToUpper(method), url, bytes.NewBufferString(apiRequestBody))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.Domain, cfg.Token)
	if apiRequestBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %s", resp.Status)
	}
	return nil
}
