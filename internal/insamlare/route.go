// Copyright 2021-2026 The Self-host Authors. All rights reserved.
// Use of this source code is governed by the GPLv3
// license that can be found in the LICENSE file.

package insamlare

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	Topic         string
	Payload       []byte
	ReceivedAt    time.Time
	TopicCaptures []string
}

type RoutedMessage struct {
	Route   *CompiledRoute
	Message Message
}

type Point struct {
	TimeseriesUUID uuid.UUID
	Timestamp      time.Time
	Value          float64
}

type Transformer interface {
	Transform(Message) ([]Point, error)
}

type CompiledRoute struct {
	Config            RouteConfig
	matcher           routeMatcher
	transformer       Transformer
	subscriptionTopic string
}

type topicMatch struct {
	captures []string
}

type routeMatcher interface {
	Match(topic string) (topicMatch, bool)
}

func CompileRoute(cfg RouteConfig, tcfg TransformConfig) (*CompiledRoute, error) {
	matcher, err := newRouteMatcher(cfg)
	if err != nil {
		return nil, err
	}
	subscriptionTopic, err := cfg.ResolveSubscriptionTopic()
	if err != nil {
		return nil, err
	}

	var transformer Transformer
	if strings.TrimSpace(cfg.ScriptPath) != "" {
		transformer, err = newTengoTransformer(cfg, tcfg)
		if err != nil {
			return nil, err
		}
	} else {
		transformer, err = newFixedTransformer(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &CompiledRoute{
		Config:            cfg,
		matcher:           matcher,
		transformer:       transformer,
		subscriptionTopic: subscriptionTopic,
	}, nil
}

func (r *CompiledRoute) Matches(topic string) bool {
	_, ok := r.matcher.Match(topic)
	return ok
}

func (r *CompiledRoute) Transform(msg Message) ([]Point, error) {
	match, ok := r.matcher.Match(msg.Topic)
	if !ok {
		return nil, fmt.Errorf("topic %q does not match route %q", msg.Topic, r.Config.Name)
	}
	msg.TopicCaptures = match.captures
	return r.transformer.Transform(msg)
}

func (r *CompiledRoute) FixedTimeseriesUUID() *uuid.UUID {
	if strings.TrimSpace(r.Config.ScriptPath) != "" || strings.TrimSpace(r.Config.TimeseriesUUID) == "" {
		return nil
	}
	if containsCaptureTemplate(r.Config.TimeseriesUUID) {
		return nil
	}

	id := uuid.MustParse(r.Config.TimeseriesUUID)
	return &id
}

func (r *CompiledRoute) SubscriptionTopic() string {
	return r.subscriptionTopic
}

type wildcardMatcher struct {
	pattern []string
}

func newRouteMatcher(cfg RouteConfig) (routeMatcher, error) {
	if strings.TrimSpace(cfg.TopicRegex) != "" {
		return newRegexMatcher(cfg.TopicRegex)
	}
	return newWildcardMatcher(cfg.Topic)
}

func newWildcardMatcher(topic string) (routeMatcher, error) {
	parts := strings.Split(topic, "/")
	for i, part := range parts {
		switch {
		case part == "#":
			if i != len(parts)-1 {
				return nil, fmt.Errorf("topic wildcard '#' must be the final segment")
			}
		case strings.Contains(part, "#"):
			return nil, fmt.Errorf("topic segment %q contains invalid '#'", part)
		case strings.Contains(part, "+") && part != "+":
			return nil, fmt.Errorf("topic segment %q contains invalid '+'", part)
		}
	}
	return wildcardMatcher{pattern: parts}, nil
}

func (m wildcardMatcher) Match(topic string) (topicMatch, bool) {
	parts := strings.Split(topic, "/")
	captures := make([]string, 0)

	for i, part := range m.pattern {
		if part == "#" {
			return topicMatch{captures: captures}, true
		}
		if i >= len(parts) {
			return topicMatch{}, false
		}
		if part == "+" {
			captures = append(captures, parts[i])
			continue
		}
		if part != parts[i] {
			return topicMatch{}, false
		}
	}

	if len(parts) != len(m.pattern) {
		return topicMatch{}, false
	}

	return topicMatch{captures: captures}, true
}

type regexMatcher struct {
	re *regexp.Regexp
}

func newRegexMatcher(expr string) (routeMatcher, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("compile topic_regex: %w", err)
	}
	return regexMatcher{re: re}, nil
}

func (m regexMatcher) Match(topic string) (topicMatch, bool) {
	matches := m.re.FindStringSubmatch(topic)
	if matches == nil {
		return topicMatch{}, false
	}
	return topicMatch{captures: matches[1:]}, true
}

func expandCaptureTemplate(template string, captures []string) string {
	if template == "" || len(captures) == 0 {
		return template
	}

	re := regexp.MustCompile(`\\([0-9]+)|\$([0-9]+)`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		idxStr := strings.TrimLeft(match, `\$`)
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx <= 0 || idx > len(captures) {
			return match
		}
		return captures[idx-1]
	})
}

func containsCaptureTemplate(value string) bool {
	return regexp.MustCompile(`\\[0-9]+|\$[0-9]+`).MatchString(value)
}
