package config

import (
	"encoding/json"
	"strings"
	"testing"
)

const benchJSON = `{"name":"inventory-api","listen":"127.0.0.1:9090","timeout":"1m30s","max_conns":250,"tags":["prod","eu-west"],"debug":true}`

// BenchmarkLoad measures the strict, validating loader.
func BenchmarkLoad(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Load(strings.NewReader(benchJSON)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnmarshalOnly is the lower bound: plain json.Unmarshal, with no
// unknown-field check, no trailing-data check and no validation. The gap to
// BenchmarkLoad is the price of strictness: about a microsecond, once, at
// startup.
func BenchmarkUnmarshalOnly(b *testing.B) {
	data := []byte(benchJSON)
	b.ReportAllocs()
	for b.Loop() {
		cfg := Default()
		if err := json.Unmarshal(data, &cfg); err != nil {
			b.Fatal(err)
		}
	}
}
