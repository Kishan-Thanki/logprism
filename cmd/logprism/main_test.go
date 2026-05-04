package main

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"
)

func format(input string, noColor bool) string {
	var b strings.Builder
	formatLine([]byte(input), options{noColor: noColor}, &b)
	return b.String()
}

func TestFormatLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		noColor  bool
		expected string
	}{
		{
			name:     "Standard JSON",
			input:    `{"time":"2026-05-02", "level":"INFO", "msg":"hello"}`,
			noColor:  true,
			expected: "2026-05-02 | [INFO] | 00000000-0000-0000-0000-000000000000 | hello\n",
		},
		{
			name:     "Non-JSON Fallback",
			input:    "plain text",
			noColor:  true,
			expected: "plain text\n",
		},
		{
			name:     "Message Alias",
			input:    `{"level":"ERROR", "message":"failed"}`,
			noColor:  true,
			expected: "[ERROR] | 00000000-0000-0000-0000-000000000000 | failed\n",
		},
		{
			name:     "Field Sorting",
			input:    `{"level":"INFO", "msg":"hi", "b":2, "a":1}`,
			noColor:  true,
			expected: "[INFO] | 00000000-0000-0000-0000-000000000000 | hi | a=1 | b=2\n",
		},
		{
			name:     "Float Time",
			input:    `{"time":1714627200.5, "level":"INFO", "msg":"epoch"}`,
			noColor:  true,
			expected: "1714627200.5 | [INFO] | 00000000-0000-0000-0000-000000000000 | epoch\n",
		},
		{
			name:     "Nested Object Value",
			input:    `{"level":"INFO", "msg":"x", "ctx":{"k":"v"}}`,
			noColor:  true,
			expected: "[INFO] | 00000000-0000-0000-0000-000000000000 | x | ctx={\"k\":\"v\"}\n",
		},
		{
			name:     "Escaped Backslash at End",
			input:    `{"level":"INFO", "msg":"foo\\"}`,
			noColor:  true,
			expected: "[INFO] | 00000000-0000-0000-0000-000000000000 | foo\\\\\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format(tt.input, tt.noColor)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatLineColorized(t *testing.T) {
	out := format(`{"level":"ERROR", "msg":"boom"}`, false)
	if !strings.Contains(out, colorRed) {
		t.Errorf("expected red color in ERROR output, got %q", out)
	}
	if !strings.Contains(out, colorReset) {
		t.Errorf("expected color reset in output, got %q", out)
	}

	out = format(`{"level":"INFO", "msg":"hi", "k":"v"}`, false)
	if !strings.Contains(out, colorBlue) {
		t.Errorf("expected blue for INFO, got %q", out)
	}
	if !strings.Contains(out, colorGreen) {
		t.Errorf("expected green for extra key, got %q", out)
	}
}

func TestFormatLinePretty(t *testing.T) {
	var b strings.Builder
	formatLine(
		[]byte(`{"level":"INFO","msg":"x","ctx":{"k":"v","n":1}}`),
		options{noColor: true, pretty: true},
		&b,
	)
	if !strings.Contains(b.String(), "{\n  \"k\": \"v\"") {
		t.Errorf("expected pretty-printed nested object, got %q", b.String())
	}
}

func compile(spec string) (string, []filterCond) {
	k, c, _ := parseFilterSpec(spec)
	return k, c
}

func TestFormatLineFilter(t *testing.T) {
	var b strings.Builder

	mk := func(spec string) []filterEntry {
		k, c := compile(spec)
		return []filterEntry{{key: k, keyBytes: []byte(k), conds: c}}
	}

	b.Reset()
	emit := formatLine(
		[]byte(`{"level":"ERROR","msg":"boom"}`),
		options{noColor: true, filters: mk("level=ERROR")},
		&b,
	)
	if !emit {
		t.Errorf("expected match for level=ERROR")
	}

	b.Reset()
	emit = formatLine(
		[]byte(`{"level":"INFO","msg":"x"}`),
		options{noColor: true, filters: mk("level=ERROR")},
		&b,
	)
	if emit {
		t.Errorf("expected filter to reject INFO")
	}

	b.Reset()
	emit = formatLine(
		[]byte(`{"level":"INFO","msg":"x","status":200}`),
		options{noColor: true, filters: mk("status=200")},
		&b,
	)
	if !emit {
		t.Errorf("expected numeric filter status=200 to match")
	}

	b.Reset()
	emit = formatLine(
		[]byte(`raw line`),
		options{noColor: true, filters: mk("level=ERROR")},
		&b,
	)
	if emit {
		t.Errorf("expected non-JSON line to be filtered out under active filters")
	}
}

func TestMatchFiltersV130(t *testing.T) {
	mk := func(spec string) []filterEntry {
		k, c := compile(spec)
		return []filterEntry{{key: k, keyBytes: []byte(k), conds: c}}
	}

	tests := []struct {
		name     string
		record   logRecord
		opts     options
		expected bool
	}{
		{
			name:     "Numeric Match (Greater)",
			record:   logRecord{extras: extraFields{{key: []byte("status"), val: []byte("500")}}},
			opts:     options{filters: mk("status>400")},
			expected: true,
		},
		{
			name:     "Numeric Mismatch (Greater)",
			record:   logRecord{extras: extraFields{{key: []byte("status"), val: []byte("200")}}},
			opts:     options{filters: mk("status>400")},
			expected: false,
		},
		{
			name:     "Numeric Match (Lesser)",
			record:   logRecord{extras: extraFields{{key: []byte("latency"), val: []byte("50")}}},
			opts:     options{filters: mk("latency<100")},
			expected: true,
		},
		{
			name:     "Numeric Match (GreaterEqual Boundary)",
			record:   logRecord{extras: extraFields{{key: []byte("status"), val: []byte("400")}}},
			opts:     options{filters: mk("status>=400")},
			expected: true,
		},
		{
			name:     "Numeric Match (LessEqual Boundary)",
			record:   logRecord{extras: extraFields{{key: []byte("latency"), val: []byte("100")}}},
			opts:     options{filters: mk("latency<=100")},
			expected: true,
		},
		{
			name:     "Exclusion Filter (Hit)",
			record:   logRecord{hasLevel: true, level: []byte("DEBUG")},
			opts:     options{exclusions: mk("level=DEBUG")},
			expected: false,
		},
		{
			name:     "Exclusion Filter (Miss)",
			record:   logRecord{hasLevel: true, level: []byte("INFO")},
			opts:     options{exclusions: mk("level=DEBUG")},
			expected: true,
		},
		{
			name:     "OR Logic (Match First)",
			record:   logRecord{hasLevel: true, level: []byte("ERROR")},
			opts:     options{filters: mk("level=ERROR,WARN")},
			expected: true,
		},
		{
			name:     "OR Logic (Match Second)",
			record:   logRecord{hasLevel: true, level: []byte("WARN")},
			opts:     options{filters: mk("level=ERROR,WARN")},
			expected: true,
		},
		{
			name:     "OR Logic (Mismatch)",
			record:   logRecord{hasLevel: true, level: []byte("INFO")},
			opts:     options{filters: mk("level=ERROR,WARN")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchFilters(&tt.record, tt.opts)
			if got != tt.expected {
				t.Errorf("matchFilters() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOperatorBoundaries(t *testing.T) {
	mk := func(spec string) []filterEntry {
		k, c := compile(spec)
		return []filterEntry{{key: k, keyBytes: []byte(k), conds: c}}
	}

	tests := []struct {
		name     string
		record   logRecord
		opts     options
		expected bool
	}{
		{
			name:     "Negative Numbers (Match)",
			record:   logRecord{extras: extraFields{{key: []byte("val"), val: []byte("-50")}}},
			opts:     options{filters: mk("val<0")},
			expected: true,
		},
		{
			name:     "Negative Numbers (Mismatch)",
			record:   logRecord{extras: extraFields{{key: []byte("val"), val: []byte("-50")}}},
			opts:     options{filters: mk("val>0")},
			expected: false,
		},
		{
			name:     "Scientific Notation (Fallthrough to String)",
			record:   logRecord{extras: extraFields{{key: []byte("val"), val: []byte("1e5")}}},
			opts:     options{filters: mk("val=1e5")},
			expected: true,
		},
		{
			name:     "Non-Numeric String (Fallthrough to String)",
			record:   logRecord{extras: extraFields{{key: []byte("val"), val: []byte("abc")}}},
			opts:     options{filters: mk("val>abb")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchFilters(&tt.record, tt.opts)
			if got != tt.expected {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestStructuralEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "Empty Object",
			input:    `{}`,
			contains: []string{"00000000-0000-0000-0000-000000000000"},
		},
		{
			name:     "Empty Array Value",
			input:    `{"level":"INFO","msg":"x","tags":[]}`,
			contains: []string{"tags=[]"},
		},
		{
			name:     "Null Value",
			input:    `{"level":"INFO","msg":"x","user":null}`,
			contains: []string{"user=null"},
		},
		{
			name:     "UTF-8 Characters",
			input:    `{"level":"INFO","msg":"🚀 space","author":"Kishan 😊"}`,
			contains: []string{"🚀 space", "Kishan 😊"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			formatLine([]byte(tt.input), options{noColor: true}, &b)
			out := b.String()
			for _, s := range tt.contains {
				if !strings.Contains(out, s) {
					t.Errorf("%s: expected %q in %q", tt.name, s, out)
				}
			}
		})
	}
}

func TestMemoryEfficiency(t *testing.T) {
	line := []byte(`{"level":"INFO","msg":"x","status":200,"ctx":{"a":1}}` + "\n")
	r := &infiniteReader{line: line, limit: 100000}
	w := io.Discard

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	if err := run(r, w, options{noColor: true}); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)
	if m2.HeapObjects > m1.HeapObjects && m2.HeapObjects-m1.HeapObjects > 1000 {
		t.Errorf("excessive heap objects: %d -> %d", m1.HeapObjects, m2.HeapObjects)
	}
}

type infiniteReader struct {
	line  []byte
	limit int
	count int
}

func (r *infiniteReader) Read(p []byte) (int, error) {
	if r.count >= r.limit {
		return 0, io.EOF
	}
	n := copy(p, r.line)
	r.count++
	return n, nil
}

func TestFieldMapping(t *testing.T) {
	input := []byte(`{"severity":"ERROR","timestamp":"2026","msg":"x"}`)
	opts := options{
		noColor:  true,
		fieldMap: map[string]string{"level": "severity", "time": "timestamp"},
	}
	var sb strings.Builder
	formatLine(input, opts, &sb)
	out := sb.String()
	if !strings.Contains(out, "2026") || !strings.Contains(out, "[ERROR]") {
		t.Errorf("expected mapped fields to be resolved, got %q", out)
	}
}

func TestHighlighting(t *testing.T) {
	input := []byte(`{"level":"INFO","msg":"user-123 logged in"}`)
	opts := options{
		highlights: []string{"user-123"},
	}
	var sb strings.Builder
	formatLine(input, opts, &sb)
	out := sb.String()
	if !strings.Contains(out, colorYellow) || !strings.Contains(out, "user-123") {
		t.Errorf("expected highlight color for matched term, got %q", out)
	}
}

func TestRunLargeLine(t *testing.T) {
	big := strings.Repeat("x", 600*1024)
	input := `{"level":"INFO","msg":"big","data":"` + big + `"}` + "\n"

	var out bytes.Buffer
	if err := run(strings.NewReader(input), &out, options{noColor: true}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(out.String(), "data=") {
		t.Errorf("expected output to include data= field")
	}
	if !strings.Contains(out.String(), big[:128]) {
		t.Errorf("expected large value to be preserved in output")
	}
}

type errWriter struct{ err error }

func (e *errWriter) Write(p []byte) (int, error) { return 0, e.err }

func TestRunPropagatesWriteError(t *testing.T) {
	input := `{"level":"INFO","msg":"x"}` + "\n"
	sentinel := &errWriter{err: errSentinel}

	err := run(strings.NewReader(input), sentinel, options{noColor: true})
	if err != errSentinel {
		t.Errorf("expected write error to propagate, got %v", err)
	}
}

var errSentinel = &sentinelError{}

type sentinelError struct{}

func (*sentinelError) Error() string { return "sentinel" }
