// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"time"

	"github.com/self-host/self-host/pkg/configdir"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func initConfig() {
	viper.SetConfigName(os.Getenv("CONFIG_FILENAME"))
	viper.SetConfigType("yaml")
	for _, p := range configdir.SystemConfig("selfhost") {
		viper.AddConfigPath(p)
	}
	for _, p := range configdir.LocalConfig("selfhost") {
		viper.AddConfigPath(p)
	}
	viper.AddConfigPath(".")

	viper.SetDefault("mqtt.client_id", "selfhost-insamlare")
	viper.SetDefault("mqtt.clean_session", false)
	viper.SetDefault("mqtt.qos", 1)
	viper.SetDefault("ingest.batch_size", 5000)
	viper.SetDefault("ingest.flush_interval", 2*time.Second)
	viper.SetDefault("ingest.workers", 4)
	viper.SetDefault("ingest.queue_size", 10000)
	viper.SetDefault("load_log.interval", time.Minute)
	viper.SetDefault("transform.timeout", 250*time.Millisecond)
	viper.SetDefault("postgres.created_by_uuid", "00000000-0000-1000-8000-000000000000")

	if err := viper.ReadInConfig(); err != nil {
		logger.Fatal("Fatal error unable to load config file", zap.Error(err))
	}
}
