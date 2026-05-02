package main

import (
	"bytes"
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

func TestFormatLineFilter(t *testing.T) {
	var b strings.Builder

	// Match on level.
	b.Reset()
	emit := formatLine(
		[]byte(`{"level":"ERROR","msg":"boom"}`),
		options{noColor: true, filters: map[string]string{"level": "ERROR"}},
		&b,
	)
	if !emit {
		t.Errorf("expected match for level=ERROR")
	}

	// Mismatch.
	b.Reset()
	emit = formatLine(
		[]byte(`{"level":"INFO","msg":"x"}`),
		options{noColor: true, filters: map[string]string{"level": "ERROR"}},
		&b,
	)
	if emit {
		t.Errorf("expected filter to reject INFO")
	}

	// Numeric match.
	b.Reset()
	emit = formatLine(
		[]byte(`{"level":"INFO","msg":"x","status":200}`),
		options{noColor: true, filters: map[string]string{"status": "200"}},
		&b,
	)
	if !emit {
		t.Errorf("expected numeric filter status=200 to match")
	}

	// Non-JSON line is filtered out when filters are set.
	b.Reset()
	emit = formatLine(
		[]byte(`raw line`),
		options{noColor: true, filters: map[string]string{"level": "ERROR"}},
		&b,
	)
	if emit {
		t.Errorf("expected non-JSON line to be filtered out under active filters")
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
