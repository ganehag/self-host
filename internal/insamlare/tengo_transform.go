// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/google/uuid"
)

var allowedTengoModules = []string{
	"base64",
	"enum",
	"hex",
	"json",
	"math",
	"rand",
	"text",
	"times",
}

var tengoImportRegex = regexp.MustCompile(`import\("([^"]+)"\)`)

type tengoTransformer struct {
	program *tengo.Compiled
	timeout time.Duration
}

func newTengoTransformer(cfg RouteConfig, tcfg TransformConfig) (*tengoTransformer, error) {
	sourceCode, err := os.ReadFile(cfg.ScriptPath)
	if err != nil {
		return nil, err
	}
	if err := validateTengoImports(sourceCode); err != nil {
		return nil, err
	}

	script := tengo.NewScript(sourceCode)
	script.SetImports(stdlib.GetModuleMap(allowedTengoModules...))
	for _, name := range []string{"topic", "payload", "received_at_unix_ms", "received_at_rfc3339", "points"} {
		if err := script.Add(name, nil); err != nil {
			return nil, err
		}
	}

	compiled, err := script.Compile()
	if err != nil {
		return nil, err
	}

	return &tengoTransformer{
		program: compiled,
		timeout: tcfg.Timeout,
	}, nil
}

func (t *tengoTransformer) Transform(msg Message) ([]Point, error) {
	program := t.program.Clone()
	if err := program.Set("topic", msg.Topic); err != nil {
		return nil, err
	}
	if err := program.Set("payload", string(msg.Payload)); err != nil {
		return nil, err
	}
	if err := program.Set("received_at_unix_ms", msg.ReceivedAt.UnixMilli()); err != nil {
		return nil, err
	}
	if err := program.Set("received_at_rfc3339", msg.ReceivedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	if err := program.Set("points", []any{}); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	if err := program.RunContext(ctx); err != nil {
		return nil, err
	}

	return convertTengoPoints(program.Get("points").Array())
}

func validateTengoImports(sourceCode []byte) error {
	allowed := make(map[string]struct{}, len(allowedTengoModules))
	for _, name := range allowedTengoModules {
		allowed[name] = struct{}{}
	}

	var disallowed []string
	for _, match := range tengoImportRegex.FindAllSubmatch(sourceCode, -1) {
		if len(match) != 2 {
			continue
		}
		name := string(match[1])
		if _, ok := allowed[name]; ok {
			continue
		}
		disallowed = append(disallowed, name)
	}

	if len(disallowed) == 0 {
		return nil
	}

	sort.Strings(disallowed)
	return fmt.Errorf("disallowed tengo imports: %v", disallowed)
}

func convertTengoPoints(items []interface{}) ([]Point, error) {
	points := make([]Point, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("points[%d] must be a map, got %T", i, item)
		}

		rawUUID, ok := m["ts_uuid"].(string)
		if !ok {
			return nil, fmt.Errorf("points[%d].ts_uuid must be a string", i)
		}
		id, err := uuid.Parse(rawUUID)
		if err != nil {
			return nil, fmt.Errorf("points[%d].ts_uuid: %w", i, err)
		}

		rawValue, ok := m["value"]
		if !ok {
			return nil, fmt.Errorf("points[%d].value is required", i)
		}
		value, err := parseNumericAny(rawValue)
		if err != nil {
			return nil, fmt.Errorf("points[%d].value: %w", i, err)
		}

		timestamp := time.Now().UTC()
		if rawTimestamp, ok := m["ts"]; ok {
			timestamp, err = parseTimestampAny(rawTimestamp, "rfc3339", time.UTC)
			if err != nil {
				if n, nerr := parseNumericAny(rawTimestamp); nerr == nil {
					timestamp = time.UnixMilli(int64(n)).UTC()
				} else {
					return nil, fmt.Errorf("points[%d].ts: %w", i, err)
				}
			}
		}

		points = append(points, Point{
			TimeseriesUUID: id,
			Timestamp:      timestamp,
			Value:          value,
		})
	}

	return points, nil
}
