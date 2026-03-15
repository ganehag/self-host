// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"time"

	"github.com/self-host/self-host/internal/insamlare"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type app struct {
	runtime *insamlare.Runtime
}

func buildApp() (*app, error) {
	var cfg insamlare.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	rt, err := insamlare.NewRuntime(cfg)
	if err != nil {
		return nil, err
	}

	return &app{runtime: rt}, nil
}

func (a *app) Run(ctx context.Context) error {
	logger.Info("Insamlare configuration loaded",
		zap.String("broker", a.runtime.Config().MQTT.Broker),
		zap.Int("routes", len(a.runtime.Config().Routes)),
		zap.Int("workers", a.runtime.Config().Ingest.Workers),
		zap.Int("batch_size", a.runtime.Config().Ingest.BatchSize),
	)

	for _, route := range a.runtime.Config().Routes {
		fields := []zap.Field{
			zap.String("name", route.Name),
			zap.String("mode", route.Mode()),
		}
		if route.Topic != "" {
			fields = append(fields, zap.String("topic", route.Topic))
		}
		if route.TopicRegex != "" {
			fields = append(fields, zap.String("topic_regex", route.TopicRegex))
		}
		if subscriptionTopic, err := route.ResolveSubscriptionTopic(); err == nil {
			fields = append(fields, zap.String("subscription_topic", subscriptionTopic))
		}
		logger.Info("Configured route", fields...)
	}

	if interval := a.runtime.Config().LoadLog.Interval; interval > 0 {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					load := a.runtime.LoadSnapshot()
					logger.Info("Current load",
						zap.Duration("interval", interval),
						zap.Int("message_queue_len", load.MessageQueueLen),
						zap.Int("message_queue_cap", load.MessageQueueCap),
						zap.Uint64("received_messages", load.ReceivedMessages),
						zap.Uint64("transformed_points", load.TransformedPoints),
						zap.Uint64("transform_errors", load.TransformErrors),
						zap.Uint64("runtime_errors", load.RuntimeErrors),
						zap.Int("writer_queue_len", load.Writer.QueueLen),
						zap.Int("writer_queue_cap", load.Writer.QueueCap),
						zap.Uint64("writer_enqueued_points", load.Writer.EnqueuedPoints),
						zap.Uint64("writer_inserted_points", load.Writer.InsertedPoints),
						zap.Uint64("writer_flushes", load.Writer.Flushes),
						zap.Uint64("writer_failed_points", load.Writer.FailedPoints),
						zap.Uint64("writer_failed_flushes", load.Writer.FailedFlushes),
					)
				}
			}
		}()
	}

	return a.runtime.Run(ctx)
}
