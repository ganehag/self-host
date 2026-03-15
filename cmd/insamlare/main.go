// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"context"

	"go.uber.org/zap"
)

func init() {
	initLogger()
	initConfig()
}

func main() {
	app, err := buildApp()
	if err != nil {
		logger.Fatal("Fatal error while building ingester", zap.Error(err))
	}

	if err := app.Run(context.Background()); err != nil {
		logger.Fatal("Fatal error while running ingester", zap.Error(err))
	}
}
