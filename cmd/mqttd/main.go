// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"go.uber.org/zap"
)

func main() {
	var (
		listenAddr     string
		inlineClient   bool
		writeBuffer    int
		readBuffer     int
		maxPending     int
		sysTopicPeriod int
	)

	flag.StringVar(&listenAddr, "listen", ":1883", "TCP listen address")
	flag.BoolVar(&inlineClient, "inline-client", false, "enable the broker inline client")
	flag.IntVar(&writeBuffer, "write-buffer", 4096, "per-client network write buffer size in bytes")
	flag.IntVar(&readBuffer, "read-buffer", 4096, "per-client network read buffer size in bytes")
	flag.IntVar(&maxPending, "max-pending", 8192, "maximum queued writes per client")
	flag.IntVar(&sysTopicPeriod, "sys-topic-seconds", 30, "system topic publish interval in seconds")
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	capabilities := mqtt.NewDefaultServerCapabilities()
	capabilities.MaximumClientWritesPending = int32(maxPending)

	server := mqtt.New(&mqtt.Options{
		InlineClient:             inlineClient,
		ClientNetWriteBufferSize: writeBuffer,
		ClientNetReadBufferSize:  readBuffer,
		SysTopicResendInterval:   int64(sysTopicPeriod),
		Capabilities:             capabilities,
	})

	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		logger.Fatal("add allow-all auth hook", zap.Error(err))
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: listenAddr,
	})
	if err := server.AddListener(tcp); err != nil {
		logger.Fatal("add tcp listener", zap.Error(err))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	logger.Info("starting mqttd",
		zap.String("listen", listenAddr),
		zap.Int("write_buffer", writeBuffer),
		zap.Int("read_buffer", readBuffer),
		zap.Int("max_pending", maxPending),
		zap.Bool("inline_client", inlineClient),
	)

	if err := server.Serve(); err != nil {
		logger.Fatal("serve mqttd", zap.Error(err))
	}

	sig := <-sigCh
	logger.Info("shutting down mqttd", zap.String("signal", sig.String()))
	if err := server.Close(); err != nil {
		logger.Error("close mqttd", zap.Error(err))
	}
}
