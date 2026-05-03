package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
)

var version = "dev"

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev != "" {
		short := rev
		if len(short) > 12 {
			short = short[:12]
		}
		if modified == "true" {
			short += "-dirty"
		}
		return short
	}
	return version
}

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
		return errors.New("filter must be in key=value form")
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
		os.Stdout.WriteString("logprism version ")
		os.Stdout.WriteString(resolveVersion())
		os.Stdout.WriteString("\n")
		return
	}

	in, closeIn, err := openInput(*inputPath)
	if err != nil {
		os.Stderr.WriteString("logprism error: ")
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer closeIn()

	out, closeOut, isFile, err := openOutput(*outputPath)
	if err != nil {
		os.Stderr.WriteString("logprism error: ")
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
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
		os.Stderr.WriteString("logprism error: ")
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
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
	s := &jsonScanner{data: line}
	if !s.startObject() {
		if len(opts.filters) > 0 {
			return false
		}
		b.Write(line)
		b.WriteByte('\n')
		return true
	}

	var (
		timeVal, levelVal, serviceVal, msgVal, traceIDVal []byte
		hasTime, hasLevel, hasService, hasMsg, hasTraceID bool
	)

	type extraField struct {
		key   []byte
		val   []byte
		isStr bool
	}
	extras := make([]extraField, 0, 8)

	for {
		key, val, isStr, ok := s.nextField()
		if !ok {
			break
		}

		if bytes.Equal(key, []byte("time")) {
			timeVal, hasTime = val, true
		} else if bytes.Equal(key, []byte("level")) {
			levelVal, hasLevel = val, true
		} else if bytes.Equal(key, []byte("service")) {
			serviceVal, hasService = val, true
		} else if bytes.Equal(key, []byte("msg")) || bytes.Equal(key, []byte("message")) {
			if !hasMsg {
				msgVal, hasMsg = val, true
			}
		} else if bytes.Equal(key, []byte("trace_id")) {
			traceIDVal, hasTraceID = val, true
		} else {
			extras = append(extras, extraField{key, val, isStr})
		}
	}

	if len(opts.filters) > 0 {
		for fk, fv := range opts.filters {
			found := false
			bfk := []byte(fk)
			if bytes.Equal(bfk, []byte("time")) {
				if hasTime && string(timeVal) == fv {
					found = true
				}
			} else if bytes.Equal(bfk, []byte("level")) {
				if hasLevel && string(levelVal) == fv {
					found = true
				}
			} else if bytes.Equal(bfk, []byte("service")) {
				if hasService && string(serviceVal) == fv {
					found = true
				}
			} else if bytes.Equal(bfk, []byte("msg")) || bytes.Equal(bfk, []byte("message")) {
				if hasMsg && string(msgVal) == fv {
					found = true
				}
			} else if bytes.Equal(bfk, []byte("trace_id")) {
				if hasTraceID && string(traceIDVal) == fv {
					found = true
				}
			}

			if !found {
				for _, e := range extras {
					if string(e.key) == fk && string(e.val) == fv {
						found = true
						break
					}
				}
			}
			if !found {
				return false
			}
		}
	}

	if hasTime {
		if !opts.noColor {
			b.WriteString(colorGray)
		}
		b.Write(timeVal)
		if !opts.noColor {
			b.WriteString(colorReset)
		}
		b.WriteString(" | ")
	}

	if !opts.noColor {
		lvl := strings.ToUpper(string(levelVal))
		switch lvl {
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
	b.Write(levelVal)
	b.WriteString("]")
	if !opts.noColor {
		b.WriteString(colorReset)
	}

	if hasService {
		b.WriteString(" ")
		b.Write(serviceVal)
	}

	b.WriteString(" | ")
	if hasTraceID {
		b.Write(traceIDVal)
	} else {
		b.WriteString("00000000-0000-0000-0000-000000000000")
	}
	b.WriteString(" | ")
	b.Write(msgVal)

	sort.Slice(extras, func(i, j int) bool {
		return string(extras[i].key) < string(extras[j].key)
	})

	for _, e := range extras {
		b.WriteString(" | ")
		if !opts.noColor {
			b.WriteString(colorGreen)
		}
		b.Write(e.key)
		if !opts.noColor {
			b.WriteString(colorReset)
		}
		b.WriteString("=")
		if opts.pretty && len(e.val) > 0 && (e.val[0] == '{' || e.val[0] == '[') {
			writePretty(b, e.val, 0)
		} else {
			b.Write(e.val)
		}
	}

	b.WriteByte('\n')
	return true
}

type jsonScanner struct {
	data []byte
	pos  int
}

func (s *jsonScanner) startObject() bool {
	s.skipWhitespace()
	if s.pos >= len(s.data) || s.data[s.pos] != '{' {
		return false
	}
	s.pos++
	return true
}

func (s *jsonScanner) nextField() (key, val []byte, isStr bool, ok bool) {
	for {
		s.skipWhitespace()
		if s.pos >= len(s.data) || s.data[s.pos] == '}' {
			return nil, nil, false, false
		}
		if s.data[s.pos] == ',' {
			s.pos++
			continue
		}
		break
	}

	s.skipWhitespace()
	key, ok = s.readString()
	if !ok {
		return nil, nil, false, false
	}

	s.skipWhitespace()
	if s.pos >= len(s.data) || s.data[s.pos] != ':' {
		return nil, nil, false, false
	}
	s.pos++

	s.skipWhitespace()
	val, isStr, ok = s.readValue()
	return key, val, isStr, ok
}

func (s *jsonScanner) skipWhitespace() {
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s.pos++
	}
}

func (s *jsonScanner) readString() ([]byte, bool) {
	if s.pos >= len(s.data) || s.data[s.pos] != '"' {
		return nil, false
	}
	s.pos++
	start := s.pos
	for s.pos < len(s.data) {
		if s.data[s.pos] == '"' && s.data[s.pos-1] != '\\' {
			res := s.data[start:s.pos]
			s.pos++
			return res, true
		}
		s.pos++
	}
	return nil, false
}

func (s *jsonScanner) readValue() (val []byte, isStr bool, ok bool) {
	if s.pos >= len(s.data) {
		return nil, false, false
	}
	c := s.data[s.pos]
	if c == '"' {
		v, ok := s.readString()
		return v, true, ok
	}
	if c == '{' || c == '[' {
		return s.readBlock(), false, true
	}
	start := s.pos
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		s.pos++
	}
	return s.data[start:s.pos], false, true
}

func (s *jsonScanner) readBlock() []byte {
	start := s.pos
	open := s.data[s.pos]
	var close byte
	if open == '{' {
		close = '}'
	} else {
		close = ']'
	}
	depth := 0
	inString := false
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == '"' && (s.pos == 0 || s.data[s.pos-1] != '\\') {
			inString = !inString
		} else if !inString {
			if c == open {
				depth++
			} else if c == close {
				depth--
				if depth == 0 {
					s.pos++
					return s.data[start:s.pos]
				}
			}
		}
		s.pos++
	}
	return s.data[start:s.pos]
}

func writePretty(b *strings.Builder, data []byte, indent int) {
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if c == '"' && (i == 0 || data[i-1] != '\\') {
			inString = !inString
			b.WriteByte(c)
			continue
		}
		if inString {
			b.WriteByte(c)
			continue
		}
		switch c {
		case '{', '[':
			b.WriteByte(c)
			b.WriteByte('\n')
			indent++
			writeIndent(b, indent)
		case '}', ']':
			b.WriteByte('\n')
			indent--
			writeIndent(b, indent)
			b.WriteByte(c)
		case ',':
			b.WriteByte(c)
			b.WriteByte('\n')
			writeIndent(b, indent)
		case ':':
			b.WriteString(": ")
		case ' ', '\t', '\n', '\r':
			continue
		default:
			b.WriteByte(c)
		}
	}
}

func writeIndent(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		b.WriteString("  ")
	}
}
