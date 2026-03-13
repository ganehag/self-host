// Copyright 2021 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/self-host/self-host/pkg/configdir"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func initConfig() {
	var err error

	viper.SetConfigName(os.Getenv("CONFIG_FILENAME"))
	viper.SetConfigType("yaml")
	for _, p := range configdir.SystemConfig("selfhost") {
		viper.AddConfigPath(p)
	}
	for _, p := range configdir.LocalConfig("selfhost") {
		viper.AddConfigPath(p)
	}
	viper.AddConfigPath(".")

	// Default settings
	viper.SetDefault("rate_control.req_per_hour", 600)
	viper.SetDefault("rate_control.maxburst", 10)
	viper.SetDefault("rate_control.cleanup", 3*time.Minute)
	viper.SetDefault("request_logging.enabled", true)
	viper.SetDefault("openapi_validation.enabled", true)
	viper.SetDefault("db_pool.max_open_conns", 25)
	viper.SetDefault("db_pool.max_idle_conns", 10)
	viper.SetDefault("db_pool.conn_max_lifetime", 30*time.Minute)
	viper.SetDefault("db_pool.conn_max_idle_time", 5*time.Minute)
	viper.SetDefault("timeseries_rollups.enabled", true)
	viper.SetDefault("timeseries_queries.max_series", 32)
	viper.SetDefault("timeseries_queries.max_points_per_series", 10000)
	viper.SetDefault("timeseries_queries.max_total_points", 100000)
	viper.SetDefault("dataset_storage.backend", "inline")
	viper.SetDefault("dataset_storage.s3.region", "us-east-1")
	viper.SetDefault("dataset_storage.s3.use_ssl", false)
	viper.SetDefault("dataset_storage.s3.force_path_style", true)
	viper.SetDefault("dataset_storage.s3.key_prefix", "datasets")
	viper.SetDefault("dataset_uploads.root_dir", filepath.Join(os.TempDir(), "selfhost-dataset-uploads"))
	viper.SetDefault("dataset_uploads.max_part_size", 16*1024*1024)
	viper.SetDefault("dataset_uploads.max_total_size", 128*1024*1024)

	// CORS default settings
	viper.SetDefault("cors.allowed_origins", []string{"https://*", "http://*"})
	viper.SetDefault("cors.allowed_methods", []string{"POST", "GET", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.allowed_headers", []string{"Accept", "Authorization", "Content-Type", "Content-MD5", "If-None-Match"})
	viper.SetDefault("cors.exposed_headers", []string{"Link"})
	viper.SetDefault("cors.allow_credentials", true)
	viper.SetDefault("cors.max_age", 300) // Maximum value not ignored by any of major browsers

	err = viper.ReadInConfig()
	if err != nil {
		logger.Fatal("Fatal error unable to load config file", zap.Error(err))
	}
}
