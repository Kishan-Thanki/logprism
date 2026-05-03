package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
)

var version = "v1.1.0"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

type options struct {
	noColor bool
	pretty  bool
	filters map[string]string
	input   string
	output  string
}

func main() {
	opts := parseArgs(os.Args[1:])

	in, closeIn, err := openInput(opts.input)
	if err != nil {
		os.Stderr.WriteString("logprism error: ")
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer closeIn()

	out, closeOut, isFile, err := openOutput(opts.output)
	if err != nil {
		os.Stderr.WriteString("logprism error: ")
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer closeOut()

	if !opts.noColor {
		if isFile {
			opts.noColor = true
		} else if fileInfo, err := os.Stdout.Stat(); err == nil {
			if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
				opts.noColor = true
			}
		}
	}

	if err := run(in, out, opts); err != nil {
		if err == syscall.EPIPE {
			return
		}
		os.Stderr.WriteString("logprism error: ")
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
}

func parseArgs(args []string) options {
	opts := options{filters: make(map[string]string)}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-no-color", "--no-color":
			opts.noColor = true
		case "-pretty", "--pretty":
			opts.pretty = true
		case "-version", "--version":
			os.Stdout.WriteString("logprism version ")
			os.Stdout.WriteString(version)
			os.Stdout.WriteString("\n")
			os.Exit(0)
		case "-input", "--input":
			if i+1 < len(args) {
				opts.input = args[i+1]
				i++
			}
		case "-output", "--output":
			if i+1 < len(args) {
				opts.output = args[i+1]
				i++
			}
		case "-filter", "--filter":
			if i+1 < len(args) {
				f := args[i+1]
				idx := strings.Index(f, "=")
				if idx > 0 {
					opts.filters[f[:idx]] = f[idx+1:]
				}
				i++
			}
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		}
	}
	return opts
}

func printHelp() {
	os.Stdout.WriteString("Usage: logprism [flags]\n\n")
	os.Stdout.WriteString("Flags:\n")
	os.Stdout.WriteString("  -input <path>      Read from file instead of stdin\n")
	os.Stdout.WriteString("  -output <path>     Write to file instead of stdout\n")
	os.Stdout.WriteString("  -filter k=v        Filter lines (repeatable)\n")
	os.Stdout.WriteString("  -pretty            Indent nested JSON\n")
	os.Stdout.WriteString("  -no-color          Disable color output\n")
	os.Stdout.WriteString("  -version           Show version\n")
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

	extras := make(extraFields, 0, 8)

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
		if strings.Contains(lvl, "ERROR") || strings.Contains(lvl, "FATAL") || strings.Contains(lvl, "PANIC") {
			b.WriteString(colorRed)
		} else if strings.Contains(lvl, "WARN") {
			b.WriteString(colorYellow)
		} else if strings.Contains(lvl, "INFO") {
			b.WriteString(colorBlue)
		} else if strings.Contains(lvl, "DEBUG") {
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

	sort.Sort(extras)

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

type extraField struct {
	key   []byte
	val   []byte
	isStr bool
}

type extraFields []extraField

func (f extraFields) Len() int           { return len(f) }
func (f extraFields) Less(i, j int) bool { return bytes.Compare(f[i].key, f[j].key) < 0 }
func (f extraFields) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

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
