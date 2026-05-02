package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var version = "dev"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

type filterFlag map[string]string

func (f filterFlag) String() string {
	parts := make([]string, 0, len(f))
	for k, v := range f {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (f filterFlag) Set(s string) error {
	idx := strings.Index(s, "=")
	if idx <= 0 {
		return fmt.Errorf("filter must be in key=value form, got %q", s)
	}
	f[s[:idx]] = s[idx+1:]
	return nil
}

type options struct {
	noColor bool
	pretty  bool
	filters map[string]string
}

func main() {
	noColor := flag.Bool("no-color", false, "Disable ANSI color output")
	pretty := flag.Bool("pretty", false, "Indent nested JSON values across multiple lines")
	showVersion := flag.Bool("version", false, "Display version information")
	inputPath := flag.String("input", "", "Read from file instead of stdin (use \"-\" for stdin)")
	outputPath := flag.String("output", "", "Write to file instead of stdout (use \"-\" for stdout)")
	filters := filterFlag{}
	flag.Var(filters, "filter", "Only emit lines where key=value (repeatable, e.g. -filter level=ERROR -filter service=api)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("logprism version %s\n", version)
		return
	}

	in, closeIn, err := openInput(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logprism error: %v\n", err)
		os.Exit(1)
	}
	defer closeIn()

	out, closeOut, isFile, err := openOutput(*outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logprism error: %v\n", err)
		os.Exit(1)
	}
	defer closeOut()

	if !*noColor {
		if isFile {
			*noColor = true
		} else if fileInfo, err := os.Stdout.Stat(); err == nil {
			if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
				*noColor = true
			}
		}
	}

	opts := options{noColor: *noColor, pretty: *pretty, filters: filters}
	if err := run(in, out, opts); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			return
		}
		fmt.Fprintf(os.Stderr, "logprism error: %v\n", err)
		os.Exit(1)
	}
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "" || path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func openOutput(path string) (io.Writer, func(), bool, error) {
	if path == "" || path == "-" {
		return os.Stdout, func() {}, false, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, false, err
	}
	return f, func() { f.Close() }, true, nil
}

func run(r io.Reader, w io.Writer, opts options) error {
	scanner := bufio.NewScanner(r)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var b strings.Builder

	for scanner.Scan() {
		if err := writeLine(w, scanner.Bytes(), opts, &b); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func writeLine(w io.Writer, line []byte, opts options, b *strings.Builder) error {
	b.Reset()
	if !formatLine(line, opts, b) {
		return nil
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func formatLine(line []byte, opts options, b *strings.Builder) bool {
	var m map[string]any

	if err := json.Unmarshal(line, &m); err != nil {
		if len(opts.filters) > 0 {
			return false
		}
		b.Write(line)
		b.WriteByte('\n')
		return true
	}

	if !matchesFilters(m, opts.filters) {
		return false
	}

	var timeStr string
	switch v := m["time"].(type) {
	case string:
		timeStr = v
	case float64:
		timeStr = strconv.FormatFloat(v, 'f', -1, 64)
	}

	level, _ := m["level"].(string)
	service, _ := m["service"].(string)

	msg, ok := m["msg"].(string)
	if !ok {
		msg, _ = m["message"].(string)
	}

	traceID, _ := m["trace_id"].(string)
	if traceID == "" {
		traceID = "00000000-0000-0000-0000-000000000000"
	}

	delete(m, "time")
	delete(m, "level")
	delete(m, "msg")
	delete(m, "message")
	delete(m, "trace_id")
	delete(m, "service")

	if timeStr != "" {
		if !opts.noColor {
			b.WriteString(colorGray)
		}
		b.WriteString(timeStr)
		if !opts.noColor {
			b.WriteString(colorReset)
		}
		b.WriteString(" | ")
	}

	if !opts.noColor {
		switch strings.ToUpper(level) {
		case "ERROR", "FATAL", "PANIC":
			b.WriteString(colorRed)
		case "WARN", "WARNING":
			b.WriteString(colorYellow)
		case "INFO":
			b.WriteString(colorBlue)
		case "DEBUG":
			b.WriteString(colorGray)
		}
	}
	b.WriteString("[")
	b.WriteString(level)
	b.WriteString("]")
	if !opts.noColor {
		b.WriteString(colorReset)
	}

	if service != "" {
		b.WriteString(" ")
		b.WriteString(service)
	}

	b.WriteString(" | ")
	b.WriteString(traceID)
	b.WriteString(" | ")
	b.WriteString(msg)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		b.WriteString(" | ")
		if !opts.noColor {
			b.WriteString(colorGreen)
		}
		b.WriteString(k)
		if !opts.noColor {
			b.WriteString(colorReset)
		}
		b.WriteString("=")

		switch val := v.(type) {
		case string:
			b.WriteString(val)
		case float64:
			b.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			b.WriteString(strconv.FormatBool(val))
		case nil:
			b.WriteString("null")
		default:
			if opts.pretty {
				raw, _ := json.MarshalIndent(val, "", "  ")
				b.Write(raw)
			} else {
				raw, _ := json.Marshal(val)
				b.Write(raw)
			}
		}
	}

	b.WriteByte('\n')
	return true
}

func matchesFilters(m map[string]any, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	for k, want := range filters {
		got, ok := m[k]
		if !ok {
			return false
		}
		if !valueEquals(got, want) {
			return false
		}
	}
	return true
}

func valueEquals(v any, want string) bool {
	switch val := v.(type) {
	case string:
		return val == want
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64) == want
	case bool:
		return strconv.FormatBool(val) == want
	case nil:
		return want == "null"
	default:
		raw, _ := json.Marshal(val)
		return string(raw) == want
	}
}
