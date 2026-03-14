// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Config struct {
	MQTT      MQTTConfig      `mapstructure:"mqtt"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Ingest    IngestConfig    `mapstructure:"ingest"`
	LoadLog   LoadLogConfig   `mapstructure:"load_log"`
	Transform TransformConfig `mapstructure:"transform"`
	Routes    []RouteConfig   `mapstructure:"routes"`
}

type MQTTConfig struct {
	Broker       string `mapstructure:"broker"`
	ClientID     string `mapstructure:"client_id"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	CleanSession bool   `mapstructure:"clean_session"`
	QOS          byte   `mapstructure:"qos"`
}

type PostgresConfig struct {
	DSN           string `mapstructure:"dsn"`
	CreatedByUUID string `mapstructure:"created_by_uuid"`
}

type IngestConfig struct {
	BatchSize     int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	Workers       int           `mapstructure:"workers"`
	QueueSize     int           `mapstructure:"queue_size"`
}

type LoadLogConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}

type TransformConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type RouteConfig struct {
	Name            string `mapstructure:"name"`
	Topic           string `mapstructure:"topic"`
	TopicRegex      string `mapstructure:"topic_regex"`
	Subscription    string `mapstructure:"subscription_topic"`
	TimeseriesUUID  string `mapstructure:"timeseries_uuid"`
	ScriptPath      string `mapstructure:"script_path"`
	PayloadFormat   string `mapstructure:"payload_format"`
	ValueKey        string `mapstructure:"value_key"`
	TimestampKey    string `mapstructure:"timestamp_key"`
	TimestampFormat string `mapstructure:"timestamp_format"`
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.MQTT.Broker) == "" {
		return fmt.Errorf("mqtt.broker is required")
	}
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return fmt.Errorf("postgres.dsn is required")
	}
	if _, err := uuid.Parse(c.Postgres.CreatedByUUID); err != nil {
		return fmt.Errorf("postgres.created_by_uuid: %w", err)
	}
	if c.Ingest.BatchSize <= 0 {
		return fmt.Errorf("ingest.batch_size must be greater than zero")
	}
	if c.Ingest.FlushInterval <= 0 {
		return fmt.Errorf("ingest.flush_interval must be greater than zero")
	}
	if c.Ingest.Workers <= 0 {
		return fmt.Errorf("ingest.workers must be greater than zero")
	}
	if c.Ingest.QueueSize <= 0 {
		return fmt.Errorf("ingest.queue_size must be greater than zero")
	}
	if c.LoadLog.Interval < 0 {
		return fmt.Errorf("load_log.interval must be zero or greater")
	}
	if c.Transform.Timeout <= 0 {
		return fmt.Errorf("transform.timeout must be greater than zero")
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}

	for i, route := range c.Routes {
		if err := route.Validate(); err != nil {
			return fmt.Errorf("routes[%d]: %w", i, err)
		}
	}

	return nil
}

func (r RouteConfig) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	hasTopic := strings.TrimSpace(r.Topic) != ""
	hasTopicRegex := strings.TrimSpace(r.TopicRegex) != ""
	if !hasTopic && !hasTopicRegex {
		return fmt.Errorf("either topic or topic_regex is required")
	}
	if hasTopic && hasTopicRegex {
		return fmt.Errorf("topic and topic_regex are mutually exclusive")
	}
	if hasTopic {
		if err := validateSubscriptionTopic(r.Topic); err != nil {
			return fmt.Errorf("topic: %w", err)
		}
	}
	if strings.TrimSpace(r.Subscription) != "" {
		if err := validateSubscriptionTopic(r.Subscription); err != nil {
			return fmt.Errorf("subscription_topic: %w", err)
		}
	}
	if _, err := r.ResolveSubscriptionTopic(); err != nil {
		return err
	}

	hasUUID := strings.TrimSpace(r.TimeseriesUUID) != ""
	hasScript := strings.TrimSpace(r.ScriptPath) != ""
	if !hasUUID && !hasScript {
		return fmt.Errorf("either timeseries_uuid or script_path is required")
	}
	if hasUUID && !containsCaptureTemplate(r.TimeseriesUUID) {
		if _, err := uuid.Parse(r.TimeseriesUUID); err != nil {
			return fmt.Errorf("timeseries_uuid: %w", err)
		}
	}
	if hasScript && strings.TrimSpace(r.ScriptPath) == "" {
		return fmt.Errorf("script_path must not be empty")
	}

	format := r.payloadFormatOrDefault()
	switch format {
	case "auto", "number", "json":
	default:
		return fmt.Errorf("payload_format must be one of auto, number, json")
	}

	switch strings.TrimSpace(r.TimestampFormat) {
	case "", "rfc3339", "unix", "unix_ms":
	default:
		return fmt.Errorf("timestamp_format must be one of rfc3339, unix, unix_ms")
	}

	return nil
}

func (r RouteConfig) payloadFormatOrDefault() string {
	if strings.TrimSpace(r.PayloadFormat) == "" {
		return "auto"
	}
	return strings.TrimSpace(strings.ToLower(r.PayloadFormat))
}

func (r RouteConfig) Mode() string {
	if strings.TrimSpace(r.ScriptPath) != "" {
		return "script"
	}
	return "fixed"
}

func (r RouteConfig) ResolveSubscriptionTopic() (string, error) {
	if strings.TrimSpace(r.Subscription) != "" {
		return r.Subscription, nil
	}
	if strings.TrimSpace(r.Topic) != "" {
		return r.Topic, nil
	}
	if strings.TrimSpace(r.TopicRegex) == "" {
		return "", fmt.Errorf("either topic or topic_regex is required")
	}

	return deriveSubscriptionTopicFromRegex(r.TopicRegex)
}

func validateSubscriptionTopic(topic string) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.Contains(topic, "#") && !strings.HasSuffix(topic, "#") {
		return fmt.Errorf("wildcard '#' must be the final segment")
	}
	if strings.Count(topic, "#") > 1 {
		return fmt.Errorf("can contain at most one '#' wildcard")
	}
	return nil
}

func deriveSubscriptionTopicFromRegex(expr string) (string, error) {
	trimmed := strings.TrimSpace(expr)
	trimmed = strings.TrimPrefix(trimmed, "^")
	trimmed = strings.TrimSuffix(trimmed, "$")
	if trimmed == "" {
		return "", fmt.Errorf("topic_regex can not be empty")
	}

	segments, err := splitRegexPath(trimmed)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		switch {
		case segment == "":
			out = append(out, "")
		case !containsRegexMeta(segment):
			out = append(out, segment)
		case singleLevelCaptureRegex(segment):
			out = append(out, "+")
		default:
			return "", fmt.Errorf("topic_regex %q can not be mapped safely to an MQTT subscription; set subscription_topic explicitly", expr)
		}
	}

	subscription := strings.Join(out, "/")
	if err := validateSubscriptionTopic(subscription); err != nil {
		return "", err
	}
	return subscription, nil
}

func splitRegexPath(expr string) ([]string, error) {
	segments := make([]string, 0)
	var current strings.Builder
	inClass := false
	escaped := false

	for _, r := range expr {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			current.WriteRune(r)
			escaped = true
		case r == '[':
			current.WriteRune(r)
			inClass = true
		case r == ']':
			current.WriteRune(r)
			inClass = false
		case r == '/' && !inClass:
			segments = append(segments, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		return nil, fmt.Errorf("topic_regex ends with dangling escape")
	}
	if inClass {
		return nil, fmt.Errorf("topic_regex has unterminated character class")
	}

	segments = append(segments, current.String())
	return segments, nil
}

func containsRegexMeta(segment string) bool {
	return regexp.MustCompile(`[.^$*+?()[\]{}\\|]`).MatchString(segment)
}

func singleLevelCaptureRegex(segment string) bool {
	switch segment {
	case `[^/]+`, `[^/]*`, `([^/]+)`, `([^/]*)`, `(?:[^/]+)`, `(?:[^/]*)`:
		return true
	default:
		return false
	}
}
