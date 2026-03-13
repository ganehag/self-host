package main

import "time"

type Manifest struct {
	GeneratedAt  time.Time        `json:"generated_at" yaml:"generated_at"`
	Domain       string           `json:"domain" yaml:"domain"`
	RootUserUUID string           `json:"root_user_uuid" yaml:"root_user_uuid"`
	SeriesPrefix string           `json:"series_prefix" yaml:"series_prefix"`
	Step         string           `json:"step" yaml:"step"`
	Series       []ManifestSeries `json:"series" yaml:"series"`
	Windows      []ManifestWindow `json:"windows" yaml:"windows"`
}

type ManifestSeries struct {
	UUID   string   `json:"uuid" yaml:"uuid"`
	Name   string   `json:"name" yaml:"name"`
	Unit   string   `json:"unit" yaml:"unit"`
	Tags   []string `json:"tags" yaml:"tags"`
	Points int      `json:"points" yaml:"points"`
	Start  string   `json:"start" yaml:"start"`
	End    string   `json:"end" yaml:"end"`
}

type ManifestWindow struct {
	Name  string `json:"name" yaml:"name"`
	Start string `json:"start" yaml:"start"`
	End   string `json:"end" yaml:"end"`
}

type RunConfig struct {
	Name        string            `yaml:"name"`
	BaseURL     string            `yaml:"base_url"`
	Domain      string            `yaml:"domain"`
	Token       string            `yaml:"token"`
	Duration    time.Duration     `yaml:"duration"`
	Warmup      time.Duration     `yaml:"warmup"`
	Concurrency int               `yaml:"concurrency"`
	Timeout     time.Duration     `yaml:"timeout"`
	Headers     map[string]string `yaml:"headers"`
	Operations  []OperationConfig `yaml:"operations"`
}

type OperationConfig struct {
	Name             string            `yaml:"name"`
	Weight           int               `yaml:"weight"`
	Method           string            `yaml:"method"`
	Path             string            `yaml:"path"`
	Body             string            `yaml:"body"`
	Headers          map[string]string `yaml:"headers"`
	ExpectedStatuses []int             `yaml:"expected_statuses"`
}
