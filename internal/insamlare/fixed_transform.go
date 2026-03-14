// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type fixedTransformer struct {
	seriesUUID      string
	payloadFormat   string
	valueKey        string
	timestampKey    string
	timestampFormat string
}

func newFixedTransformer(cfg RouteConfig) (*fixedTransformer, error) {
	if !containsCaptureTemplate(cfg.TimeseriesUUID) {
		if _, err := uuid.Parse(cfg.TimeseriesUUID); err != nil {
			return nil, err
		}
	}

	return &fixedTransformer{
		seriesUUID:      strings.TrimSpace(cfg.TimeseriesUUID),
		payloadFormat:   cfg.payloadFormatOrDefault(),
		valueKey:        strings.TrimSpace(cfg.ValueKey),
		timestampKey:    strings.TrimSpace(cfg.TimestampKey),
		timestampFormat: strings.TrimSpace(cfg.TimestampFormat),
	}, nil
}

func (t *fixedTransformer) Transform(msg Message) ([]Point, error) {
	values, err := t.extract(msg)
	if err != nil {
		return nil, err
	}
	seriesUUID, err := uuid.Parse(expandCaptureTemplate(t.seriesUUID, msg.TopicCaptures))
	if err != nil {
		return nil, fmt.Errorf("resolve timeseries_uuid: %w", err)
	}

	points := make([]Point, 0, len(values))
	for _, sample := range values {
		points = append(points, Point{
			TimeseriesUUID: seriesUUID,
			Timestamp:      sample.Timestamp,
			Value:          sample.Value,
		})
	}

	return points, nil
}

type fixedValue struct {
	Value     float64
	Timestamp time.Time
}

func (t *fixedTransformer) extract(msg Message) ([]fixedValue, error) {
	switch t.payloadFormat {
	case "number":
		value, err := parseNumericPayload(msg.Payload)
		if err != nil {
			return nil, err
		}
		return []fixedValue{{Value: value, Timestamp: msg.ReceivedAt}}, nil
	case "json":
		return t.extractJSON(msg)
	case "auto":
		if value, err := parseNumericPayload(msg.Payload); err == nil {
			return []fixedValue{{Value: value, Timestamp: msg.ReceivedAt}}, nil
		}
		return t.extractJSON(msg)
	default:
		return nil, fmt.Errorf("unsupported payload format %q", t.payloadFormat)
	}
}

func (t *fixedTransformer) extractJSON(msg Message) ([]fixedValue, error) {
	var raw any
	if err := json.Unmarshal(msg.Payload, &raw); err != nil {
		return nil, err
	}

	switch v := raw.(type) {
	case map[string]any:
		point, err := t.extractJSONObject(v, msg)
		if err != nil {
			return nil, err
		}
		return []fixedValue{point}, nil
	case []any:
		points := make([]fixedValue, 0, len(v))
		for i, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("points[%d] must be an object, got %T", i, item)
			}
			point, err := t.extractJSONObject(obj, msg)
			if err != nil {
				return nil, fmt.Errorf("points[%d]: %w", i, err)
			}
			points = append(points, point)
		}
		return points, nil
	default:
		return nil, fmt.Errorf("expected json object or array, got %T", raw)
	}
}

func (t *fixedTransformer) extractJSONObject(raw map[string]any, msg Message) (fixedValue, error) {
	valueField := "value"
	if t.valueKey != "" {
		valueField = expandCaptureTemplate(t.valueKey, msg.TopicCaptures)
	}
	rawValue, ok := raw[valueField]
	if !ok {
		return fixedValue{}, fmt.Errorf("missing value key %q", valueField)
	}

	value, err := parseNumericAny(rawValue)
	if err != nil {
		return fixedValue{}, fmt.Errorf("value key %q: %w", valueField, err)
	}

	timestampKey := expandCaptureTemplate(t.timestampKey, msg.TopicCaptures)
	if timestampKey == "" {
		return fixedValue{Value: value, Timestamp: msg.ReceivedAt}, nil
	}

	rawTimestamp, ok := raw[timestampKey]
	if !ok {
		return fixedValue{}, fmt.Errorf("missing timestamp key %q", timestampKey)
	}

	ts, err := parseTimestampAny(rawTimestamp, t.timestampFormat, msg.ReceivedAt.Location())
	if err != nil {
		return fixedValue{}, fmt.Errorf("timestamp key %q: %w", timestampKey, err)
	}

	return fixedValue{Value: value, Timestamp: ts}, nil
}

func parseNumericPayload(payload []byte) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(string(payload)), 64)
}

func parseNumericAny(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case int32:
		return float64(x), nil
	case json.Number:
		return x.Float64()
	case string:
		return strconv.ParseFloat(strings.TrimSpace(x), 64)
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", v)
	}
}

func parseTimestampAny(v any, format string, loc *time.Location) (time.Time, error) {
	switch format {
	case "", "rfc3339":
		s, ok := v.(string)
		if !ok {
			return time.Time{}, fmt.Errorf("expected rfc3339 string, got %T", v)
		}
		return time.Parse(time.RFC3339, s)
	case "unix":
		n, err := parseNumericAny(v)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(int64(n), 0).In(loc), nil
	case "unix_ms":
		n, err := parseNumericAny(v)
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(int64(n)).In(loc), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp format %q", format)
	}
}
