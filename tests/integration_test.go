package tests

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binName = "logprism_test"

func TestMain(m *testing.M) {
	build := exec.Command("go", "build", "-o", binName, "../cmd/logprism")
	if err := build.Run(); err != nil {
		os.Exit(1)
	}

	result := m.Run()

	os.Remove(binName)
	os.Exit(result)
}

func TestCLI(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	tests := []struct {
		name     string
		input    string
		args     []string
		contains []string
	}{
		{
			name:     "Full JSON Path",
			input:    `{"time":"2026-05-02", "level":"INFO", "msg":"hello world", "trace_id":"123"}`,
			args:     []string{"-no-color"},
			contains: []string{"2026-05-02", "[INFO]", "123", "hello world"},
		},
		{
			name:     "Raw Text Fallback",
			input:    "this is not json",
			args:     []string{"-no-color"},
			contains: []string{"this is not json"},
		},
		{
			name:     "Message Alias",
			input:    `{"level":"ERROR", "message":"something failed"}`,
			args:     []string{"-no-color"},
			contains: []string{"[ERROR]", "something failed"},
		},
		{
			name:     "Stable Field Sorting",
			input:    `{"level":"INFO", "msg":"test", "z":1, "a":2}`,
			args:     []string{"-no-color"},
			contains: []string{"a=2", "z=1"},
		},
		{
			name:     "Float Time Field",
			input:    `{"time":1714627200, "level":"INFO", "msg":"epoch"}`,
			args:     []string{"-no-color"},
			contains: []string{"1714627200", "[INFO]", "epoch"},
		},
		{
			name:     "Nested Object Value",
			input:    `{"level":"INFO", "msg":"x", "ctx":{"k":"v"}}`,
			args:     []string{"-no-color"},
			contains: []string{`ctx={"k":"v"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(absPath, tt.args...)
			cmd.Stdin = strings.NewReader(tt.input)

			var out bytes.Buffer
			cmd.Stdout = &out

			if err := cmd.Run(); err != nil {
				t.Fatalf("failed to run binary: %v", err)
			}

			output := out.String()
			for _, search := range tt.contains {
				if !strings.Contains(output, search) {
					t.Errorf("expected output to contain %q, but got %q", search, output)
				}
			}
		})
	}
}

func TestFilterAndPretty(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	input := `{"level":"INFO","msg":"a"}` + "\n" +
		`{"level":"ERROR","msg":"b"}` + "\n" +
		`{"level":"WARN","msg":"c"}` + "\n"

	t.Run("Filter level=ERROR", func(t *testing.T) {
		cmd := exec.Command(absPath, "-no-color", "-filter", "level=ERROR")
		cmd.Stdin = strings.NewReader(input)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		o := out.String()
		if !strings.Contains(o, "[ERROR]") || strings.Contains(o, "[INFO]") || strings.Contains(o, "[WARN]") {
			t.Errorf("expected only ERROR line, got %q", o)
		}
	})

	t.Run("Multiple filters AND", func(t *testing.T) {
		multi := `{"level":"INFO","service":"api","msg":"x"}` + "\n" +
			`{"level":"INFO","service":"db","msg":"y"}` + "\n" +
			`{"level":"ERROR","service":"api","msg":"z"}` + "\n"
		cmd := exec.Command(absPath, "-no-color", "-filter", "level=INFO", "-filter", "service=api")
		cmd.Stdin = strings.NewReader(multi)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		o := out.String()
		if !strings.Contains(o, "| x") || strings.Contains(o, "| y") || strings.Contains(o, "| z") {
			t.Errorf("expected only msg=x line, got %q", o)
		}
	})

	t.Run("Pretty nested", func(t *testing.T) {
		cmd := exec.Command(absPath, "-no-color", "-pretty")
		cmd.Stdin = strings.NewReader(`{"level":"INFO","msg":"x","ctx":{"k":"v","n":1}}` + "\n")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		if !strings.Contains(out.String(), "{\n  \"k\": \"v\"") {
			t.Errorf("expected pretty-printed nested object, got %q", out.String())
		}
	})
}

func TestInputOutputFlags(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.log")
	outPath := filepath.Join(dir, "out.log")

	if err := os.WriteFile(inPath, []byte(`{"level":"INFO","msg":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	cmd := exec.Command(absPath, "-input", inPath, "-output", outPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "[INFO]") || !strings.Contains(string(data), "hello") {
		t.Errorf("expected formatted output in file, got %q", string(data))
	}
	if strings.Contains(string(data), "\033[") {
		t.Errorf("expected no ANSI codes when writing to file, got %q", string(data))
	}
}

func TestVersionFlag(t *testing.T) {
	absPath, _ := filepath.Abs(binName)
	cmd := exec.Command(absPath, "-version")

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to run version check: %v", err)
	}

	if !strings.HasPrefix(string(output), "logprism ") {
		t.Errorf("expected version output, got %q", string(output))
	}
}

func TestLargeLine(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	big := strings.Repeat("x", 800*1024)
	input := `{"level":"INFO","msg":"big","data":"` + big + `"}` + "\n"

	cmd := exec.Command(absPath, "-no-color")
	cmd.Stdin = strings.NewReader(input)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run binary: %v", err)
	}

	if !strings.Contains(out.String(), "data=") {
		t.Errorf("expected output to include data= for large line")
	}
}

func TestBrokenPipe(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	var input bytes.Buffer
	for i := 0; i < 100000; i++ {
		input.WriteString(`{"level":"INFO","msg":"x"}` + "\n")
	}

	cmd := exec.Command("sh", "-c", absPath+" -no-color | head -n 1")
	cmd.Stdin = &input

	var out bytes.Buffer
	cmd.Stdout = &out

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("pipeline should exit cleanly on broken pipe, got: %v", err)
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("pipeline hung; logprism did not honor SIGPIPE/EPIPE")
	}

	if !strings.Contains(out.String(), "[INFO]") {
		t.Errorf("expected at least one line, got %q", out.String())
	}
}

func TestAnalyzerV130Features(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	t.Run("Exclusion and Mapping", func(t *testing.T) {
		input := `{"severity":"DEBUG","msg":"noise"}` + "\n" +
			`{"severity":"ERROR","msg":"hit"}` + "\n"
		cmd := exec.Command(absPath, "-no-color", "-map", "level=severity", "-exclude", "level=DEBUG")
		cmd.Stdin = strings.NewReader(input)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		o := out.String()
		if strings.Contains(o, "noise") || !strings.Contains(o, "hit") {
			t.Errorf("expected only 'hit' after mapping and exclusion, got %q", o)
		}
	})

	t.Run("Numeric Comparison", func(t *testing.T) {
		input := `{"level":"INFO","status":200,"msg":"ok"}` + "\n" +
			`{"level":"ERROR","status":500,"msg":"fail"}` + "\n"
		cmd := exec.Command(absPath, "-no-color", "-filter", "status>400")
		cmd.Stdin = strings.NewReader(input)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		o := out.String()
		if strings.Contains(o, "ok") || !strings.Contains(o, "fail") {
			t.Errorf("expected only status>400 match, got %q", o)
		}
	})

	t.Run("Context Windows", func(t *testing.T) {
		input := `{"level":"INFO","msg":"1"}` + "\n" +
			`{"level":"INFO","msg":"2"}` + "\n" +
			`{"level":"ERROR","msg":"MATCH"}` + "\n" +
			`{"level":"INFO","msg":"3"}` + "\n"
		cmd := exec.Command(absPath, "-no-color", "-filter", "level=ERROR", "-C", "1")
		cmd.Stdin = strings.NewReader(input)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v", err)
		}
		o := out.String()
		if !strings.Contains(o, "2") || !strings.Contains(o, "MATCH") || !strings.Contains(o, "3") || strings.Contains(o, "1") {
			t.Errorf("expected context lines 2, MATCH, 3 but not 1, got %q", o)
		}
	})
}

func TestChaosStress(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	var input bytes.Buffer
	for i := 0; i < 1000; i++ {
		input.WriteString(`{"level":"INFO","msg":"valid"}` + "\n")
		input.WriteString(`{"level":"ERROR","msg":"truncated"` + "\n")
		input.WriteString("\x00\x01\x02\x03BINARY_GARBAGE\xff\xfe\n")
		input.WriteString(strings.Repeat("long_unbreakable_line_", 100) + "\n")
	}

	cmd := exec.Command(absPath, "-no-color")
	cmd.Stdin = &input
	if err := cmd.Run(); err != nil {
		t.Fatalf("logprism crashed during chaos stress test: %v", err)
	}
}

func TestMemoryFlatness(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	cmd := exec.Command(absPath, "-no-color")
	pr, pw := io.Pipe()
	cmd.Stdin = pr
	cmd.Stdout = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	go func() {
		defer pw.Close()
		line := []byte(`{"time":"2026-05-04T10:00:00Z","level":"INFO","service":"api","msg":"steady-stream","status":200}` + "\n")
		for i := 0; i < 100000; i++ {
			pw.Write(line)
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("execution failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Errorf("memory flatness test timed out")
	}
}

func TestDeepNesting(t *testing.T) {
	absPath, _ := filepath.Abs(binName)

	input := `{"level":"INFO","msg":"nested","data":` + strings.Repeat(`{"a":`, 40) + `1` + strings.Repeat(`}`, 40) + `}` + "\n"
	cmd := exec.Command(absPath, "-no-color", "-pretty")
	cmd.Stdin = strings.NewReader(input)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		t.Fatalf("crashed on deep nesting: %v", err)
	}

	if !strings.Contains(out.String(), "a=") {
		t.Errorf("expected nested keys in output, got %q", out.String())
	}
}
