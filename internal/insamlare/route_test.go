// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTopicMatcher(t *testing.T) {
	t.Parallel()

	matcher, err := newWildcardMatcher("factory/+/temperature/#")
	if err != nil {
		t.Fatal(err)
	}

	match, ok := matcher.Match("factory/line1/temperature")
	if !ok {
		t.Fatal("expected matcher to match exact prefix")
	}
	if len(match.captures) != 1 || match.captures[0] != "line1" {
		t.Fatal("expected wildcard capture to contain line1")
	}
	_, ok = matcher.Match("factory/line1/temperature/raw/modbus")
	if !ok {
		t.Fatal("expected matcher to match trailing '#'")
	}
	_, ok = matcher.Match("factory/line1/humidity")
	if ok {
		t.Fatal("expected matcher not to match different branch")
	}
}

func TestRegexMatcherAndTemplateExpansion(t *testing.T) {
	t.Parallel()

	matcher, err := newRegexMatcher(`^sensors/([^/]+)/([^/]+)$`)
	if err != nil {
		t.Fatal(err)
	}

	match, ok := matcher.Match("sensors/47daa6eb-bd1c-49de-9782-1e9422a206f5/temperature")
	if !ok {
		t.Fatal("expected regex matcher to match")
	}
	if got := expandCaptureTemplate(`$1`, match.captures); got != "47daa6eb-bd1c-49de-9782-1e9422a206f5" {
		t.Fatalf("unexpected $1 expansion %q", got)
	}
	if got := expandCaptureTemplate(`\2`, match.captures); got != `temperature` {
		t.Fatalf("unexpected \\2 expansion %q", got)
	}
}

func TestResolveSubscriptionTopicFromRegex(t *testing.T) {
	t.Parallel()

	route := RouteConfig{
		Name:       "dynamic",
		TopicRegex: `^sensors/([^/]+)/([^/]+)$`,
	}

	subscription, err := route.ResolveSubscriptionTopic()
	if err != nil {
		t.Fatal(err)
	}
	if subscription != "sensors/+/+" {
		t.Fatalf("unexpected subscription topic %q", subscription)
	}
}

func TestResolveSubscriptionTopicRequiresExplicitOverrideForComplexRegex(t *testing.T) {
	t.Parallel()

	route := RouteConfig{
		Name:       "complex",
		TopicRegex: `^sensors/(.+)/([^/]+)$`,
	}

	if _, err := route.ResolveSubscriptionTopic(); err == nil {
		t.Fatal("expected complex regex to require explicit subscription_topic")
	}

	route.Subscription = "sensors/#"
	subscription, err := route.ResolveSubscriptionTopic()
	if err != nil {
		t.Fatal(err)
	}
	if subscription != "sensors/#" {
		t.Fatalf("unexpected explicit subscription topic %q", subscription)
	}
}

func TestFixedTransformerJSON(t *testing.T) {
	t.Parallel()

	tf, err := newFixedTransformer(RouteConfig{
		TimeseriesUUID:  "47daa6eb-bd1c-49de-9782-1e9422a206f5",
		PayloadFormat:   "json",
		ValueKey:        "value",
		TimestampKey:    "ts",
		TimestampFormat: "unix_ms",
	})
	if err != nil {
		t.Fatal(err)
	}

	points, err := tf.Transform(Message{
		Topic:      "sensors/boiler/temperature",
		Payload:    []byte(`{"value":12.5,"ts":1710000000123}`),
		ReceivedAt: time.Unix(0, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected one point, got %d", len(points))
	}
	if points[0].Value != 12.5 {
		t.Fatalf("unexpected value %v", points[0].Value)
	}
	if points[0].Timestamp.UnixMilli() != 1710000000123 {
		t.Fatalf("unexpected timestamp %v", points[0].Timestamp)
	}
}

func TestFixedTransformerJSONWithTopicCaptures(t *testing.T) {
	t.Parallel()

	tf, err := newFixedTransformer(RouteConfig{
		TimeseriesUUID:  "$1",
		PayloadFormat:   "json",
		ValueKey:        "$2",
		TimestampKey:    "ts",
		TimestampFormat: "unix_ms",
	})
	if err != nil {
		t.Fatal(err)
	}

	points, err := tf.Transform(Message{
		Topic:         "sensors/47daa6eb-bd1c-49de-9782-1e9422a206f5/temperature",
		Payload:       []byte(`{"temperature":12.5,"ts":1710000000123}`),
		ReceivedAt:    time.Unix(0, 0),
		TopicCaptures: []string{"47daa6eb-bd1c-49de-9782-1e9422a206f5", "temperature"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected one point, got %d", len(points))
	}
	if points[0].TimeseriesUUID.String() != "47daa6eb-bd1c-49de-9782-1e9422a206f5" {
		t.Fatalf("unexpected timeseries uuid %s", points[0].TimeseriesUUID)
	}
	if points[0].Value != 12.5 {
		t.Fatalf("unexpected value %v", points[0].Value)
	}
}

func TestFixedTransformerJSONBatch(t *testing.T) {
	t.Parallel()

	tf, err := newFixedTransformer(RouteConfig{
		TimeseriesUUID:  "47daa6eb-bd1c-49de-9782-1e9422a206f5",
		PayloadFormat:   "json",
		ValueKey:        "value",
		TimestampKey:    "ts",
		TimestampFormat: "unix_ms",
	})
	if err != nil {
		t.Fatal(err)
	}

	points, err := tf.Transform(Message{
		Topic: "sensors/boiler/temperature",
		Payload: []byte(`[
			{"value":12.5,"ts":1710000000123},
			{"value":13.5,"ts":1710000001123}
		]`),
		ReceivedAt: time.Unix(0, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 {
		t.Fatalf("expected two points, got %d", len(points))
	}
	if points[0].Value != 12.5 || points[1].Value != 13.5 {
		t.Fatalf("unexpected values %#v", points)
	}
	if points[0].Timestamp.UnixMilli() != 1710000000123 || points[1].Timestamp.UnixMilli() != 1710000001123 {
		t.Fatalf("unexpected timestamps %#v", points)
	}
}

func TestTengoTransformer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "transform.tengo")
	source := []byte(`
payload_map := import("json").decode(payload)
points = [{
  ts_uuid: "47daa6eb-bd1c-49de-9782-1e9422a206f5",
  value: payload_map["reading"],
  ts: payload_map["ts"]
}]
`)
	if err := os.WriteFile(scriptPath, source, 0o644); err != nil {
		t.Fatal(err)
	}

	tf, err := newTengoTransformer(RouteConfig{
		Name:       "scripted",
		Topic:      "factory/line1/#",
		ScriptPath: scriptPath,
	}, TransformConfig{Timeout: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	points, err := tf.Transform(Message{
		Topic:      "factory/line1/raw",
		Payload:    []byte(`{"reading":42.75,"ts":"2026-03-14T12:13:14Z"}`),
		ReceivedAt: time.Unix(0, 0).UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected one point, got %d", len(points))
	}
	if points[0].Value != 42.75 {
		t.Fatalf("unexpected value %v", points[0].Value)
	}
	if points[0].Timestamp.Format(time.RFC3339) != "2026-03-14T12:13:14Z" {
		t.Fatalf("unexpected timestamp %v", points[0].Timestamp)
	}
}

func TestTengoTransformerRejectsDisallowedImports(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "transform.tengo")
	source := []byte(`
http := import("http")
points = [{
  ts_uuid: "47daa6eb-bd1c-49de-9782-1e9422a206f5",
  value: 1,
  ts: "2026-03-14T12:13:14Z"
}]
_ = http
`)
	if err := os.WriteFile(scriptPath, source, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := newTengoTransformer(RouteConfig{
		Name:       "scripted",
		Topic:      "factory/line1/#",
		ScriptPath: scriptPath,
	}, TransformConfig{Timeout: 250 * time.Millisecond})
	if err == nil {
		t.Fatal("expected disallowed import to fail")
	}
	if got := err.Error(); got != `disallowed tengo imports: [http]` {
		t.Fatalf("unexpected error %q", got)
	}
}
