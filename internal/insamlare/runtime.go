// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

type Runtime struct {
	cfg    Config
	routes []*CompiledRoute
	db     *sql.DB
	writer *Writer
	msgCh  chan RoutedMessage

	receivedMessages  atomic.Uint64
	transformedPoints atomic.Uint64
	transformErrors   atomic.Uint64
	runtimeErrors     atomic.Uint64
}

type LoadSnapshot struct {
	MessageQueueLen   int
	MessageQueueCap   int
	ReceivedMessages  uint64
	TransformedPoints uint64
	TransformErrors   uint64
	RuntimeErrors     uint64
	Writer            WriterStats
}

func NewRuntime(cfg Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	runtime := &Runtime{
		cfg: cfg,
	}

	routes := make([]*CompiledRoute, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		compiled, err := CompileRoute(route, cfg.Transform)
		if err != nil {
			return nil, fmt.Errorf("compile route %q: %w", route.Name, err)
		}
		routes = append(routes, compiled)
	}
	runtime.routes = routes

	db, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	runtime.db = db

	writer, err := NewWriter(db, WriterConfig{
		CreatedByUUID: cfg.Postgres.CreatedByUUID,
		BatchSize:     cfg.Ingest.BatchSize,
		FlushInterval: cfg.Ingest.FlushInterval,
		QueueSize:     cfg.Ingest.QueueSize,
		OnError: func(err error) {
			runtime.runtimeErrors.Add(1)
			logger.Error("writer flush failed", zap.Error(err))
		},
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	runtime.writer = writer

	return runtime, nil
}

func (r *Runtime) Config() Config {
	return r.cfg
}

func (r *Runtime) Run(ctx context.Context) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	if err := r.validateRoutes(ctx); err != nil {
		return err
	}

	r.msgCh = make(chan RoutedMessage, r.cfg.Ingest.QueueSize)
	var workerWG sync.WaitGroup
	workerWG.Add(r.cfg.Ingest.Workers)
	for i := 0; i < r.cfg.Ingest.Workers; i++ {
		go func() {
			defer workerWG.Done()
			r.transformLoop(ctx, r.msgCh)
		}()
	}

	writerErrCh := make(chan error, 1)
	go func() {
		writerErrCh <- r.writer.Run(ctx)
	}()

	client, err := r.connectMQTT(ctx, r.msgCh)
	if err != nil {
		close(r.msgCh)
		workerWG.Wait()
		_ = r.db.Close()
		return err
	}

	defer func() {
		client.Disconnect(250)
		close(r.msgCh)
		workerWG.Wait()
		_ = r.db.Close()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-writerErrCh:
		return err
	}
}

func (r *Runtime) transformLoop(ctx context.Context, msgCh <-chan RoutedMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-msgCh:
			if !ok {
				return
			}

			points, err := item.Route.Transform(item.Message)
			if err != nil {
				r.transformErrors.Add(1)
				logger.Error(
					"message transform failed",
					zap.String("route", item.Route.Config.Name),
					zap.String("topic", item.Message.Topic),
					zap.Error(err),
				)
				continue
			}
			if len(points) == 0 {
				continue
			}
			r.transformedPoints.Add(uint64(len(points)))
			if err := r.writer.Enqueue(ctx, points); err != nil {
				return
			}
		}
	}
}

func (r *Runtime) connectMQTT(ctx context.Context, msgCh chan<- RoutedMessage) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(r.cfg.MQTT.Broker)
	opts.SetClientID(r.cfg.MQTT.ClientID)
	opts.SetUsername(r.cfg.MQTT.Username)
	opts.SetPassword(r.cfg.MQTT.Password)
	opts.SetCleanSession(r.cfg.MQTT.CleanSession)
	opts.SetAutoReconnect(true)
	opts.SetOrderMatters(false)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(r.cfg.Ingest.FlushInterval)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.Wait() {
		return nil, fmt.Errorf("mqtt connect did not complete")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}

	for _, route := range r.routes {
		compiledRoute := route
		subscriptionTopic := compiledRoute.SubscriptionTopic()
		subToken := client.Subscribe(subscriptionTopic, r.cfg.MQTT.QOS, func(_ mqtt.Client, msg mqtt.Message) {
			routed := RoutedMessage{
				Route: compiledRoute,
				Message: Message{
					Topic:      msg.Topic(),
					Payload:    append([]byte(nil), msg.Payload()...),
					ReceivedAt: nowUTC(),
				},
			}
			r.receivedMessages.Add(1)

			select {
			case <-ctx.Done():
				return
			case msgCh <- routed:
			}
		})

		if !subToken.Wait() {
			return nil, fmt.Errorf("mqtt subscribe for %q did not complete", subscriptionTopic)
		}
		if err := subToken.Error(); err != nil {
			return nil, fmt.Errorf("mqtt subscribe %q: %w", subscriptionTopic, err)
		}
	}

	return client, nil
}

func (r *Runtime) validateRoutes(ctx context.Context) error {
	for _, route := range r.routes {
		seriesID := route.FixedTimeseriesUUID()
		if seriesID == nil {
			continue
		}

		var exists bool
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM timeseries WHERE uuid = $1)`, *seriesID).Scan(&exists); err != nil {
			return fmt.Errorf("validate route %q timeseries %s: %w", route.Config.Name, seriesID.String(), err)
		}
		if !exists {
			return fmt.Errorf("route %q references unknown timeseries_uuid %s", route.Config.Name, seriesID.String())
		}
	}

	return nil
}

func (r *Runtime) LoadSnapshot() LoadSnapshot {
	messageQueueLen := 0
	messageQueueCap := 0
	if r.msgCh != nil {
		messageQueueLen = len(r.msgCh)
		messageQueueCap = cap(r.msgCh)
	}

	return LoadSnapshot{
		MessageQueueLen:   messageQueueLen,
		MessageQueueCap:   messageQueueCap,
		ReceivedMessages:  r.receivedMessages.Load(),
		TransformedPoints: r.transformedPoints.Load(),
		TransformErrors:   r.transformErrors.Load(),
		RuntimeErrors:     r.runtimeErrors.Load(),
		Writer:            r.writer.Stats(),
	}
}
