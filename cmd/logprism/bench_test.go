package main

import (
	"io"
	"strings"
	"testing"
)

var benchInputs = map[string]string{
	"Plain":   `{"time":"2026-05-02T10:00:00Z","level":"INFO","msg":"hello"}`,
	"Service": `{"time":"2026-05-02T10:00:00Z","level":"INFO","service":"api","msg":"request","trace_id":"abc-123"}`,
	"Extras":  `{"time":"2026-05-02T10:00:00Z","level":"WARN","service":"api","msg":"slow","trace_id":"abc","status":200,"path":"/health","latency_ms":42.5,"user_id":"u-1"}`,
	"Nested":  `{"level":"INFO","msg":"x","ctx":{"k":"v","n":1,"arr":[1,2,3]}}`,
	"NonJSON": "this is not json at all, just a raw line",
}

func BenchmarkFormatLine(b *testing.B) {
	for name, input := range benchInputs {
		input := input
		b.Run(name, func(b *testing.B) {
			var sb strings.Builder
			data := []byte(input)
			opts := options{noColor: true}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sb.Reset()
				formatLine(data, opts, &sb)
			}
		})
	}
}

func BenchmarkFormatLineColor(b *testing.B) {
	input := []byte(benchInputs["Extras"])
	var sb strings.Builder
	opts := options{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		formatLine(input, opts, &sb)
	}
}

func BenchmarkRun(b *testing.B) {
	var lines strings.Builder
	for _, in := range benchInputs {
		lines.WriteString(in)
		lines.WriteByte('\n')
	}
	payload := lines.String()
	opts := options{noColor: true}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := run(strings.NewReader(payload), io.Discard, opts); err != nil {
			b.Fatal(err)
		}
	}
}
