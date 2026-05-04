package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"runtime/debug"
	"sort"
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

const (
	cmpEq byte = iota
	cmpGT
	cmpLT
	cmpGTE
	cmpLTE
)

var (
	keyTime    = []byte("time")
	keyLevel   = []byte("level")
	keyService = []byte("service")
	keyMsg     = []byte("msg")
	keyMessage = []byte("message")
	keyTraceID = []byte("trace_id")
)

type filterCond struct {
	op  byte
	val []byte
}

type filterEntry struct {
	key      string
	keyBytes []byte
	conds    []filterCond
}

type options struct {
	noColor       bool
	forceColor    bool
	pretty        bool
	filters       []filterEntry
	exclusions    []filterEntry
	hasFields     [][]byte
	fieldMap      map[string]string
	sampleRate    int
	contextBefore int
	contextAfter  int
	highlights    []string
	input         string
	output        string
}

func parseFilterSpec(spec string) (key string, conds []filterCond, ok bool) {
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		if c != '=' && c != '>' && c != '<' {
			continue
		}
		key = spec[:i]
		op := cmpEq
		valStart := i + 1
		switch c {
		case '>':
			op = cmpGT
			if i+1 < len(spec) && spec[i+1] == '=' {
				op = cmpGTE
				valStart = i + 2
			}
		case '<':
			op = cmpLT
			if i+1 < len(spec) && spec[i+1] == '=' {
				op = cmpLTE
				valStart = i + 2
			}
		}
		rest := spec[valStart:]
		for _, p := range strings.Split(rest, ",") {
			cop, cv := op, p

			if len(p) > 0 && (p[0] == '>' || p[0] == '<' || p[0] == '=') {
				switch p[0] {
				case '=':
					cop = cmpEq
					cv = p[1:]
				case '>':
					if len(p) > 1 && p[1] == '=' {
						cop = cmpGTE
						cv = p[2:]
					} else {
						cop = cmpGT
						cv = p[1:]
					}
				case '<':
					if len(p) > 1 && p[1] == '=' {
						cop = cmpLTE
						cv = p[2:]
					} else {
						cop = cmpLT
						cv = p[1:]
					}
				}
			}
			conds = append(conds, filterCond{op: cop, val: []byte(cv)})
		}
		return key, conds, true
	}
	return "", nil, false
}

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

func parsePositiveInt(s string) int {
	v := 0
	for j := 0; j < len(s); j++ {
		if s[j] < '0' || s[j] > '9' {
			break
		}
		v = v*10 + int(s[j]-'0')
	}
	return v
}

func printHelp() {
	os.Stdout.WriteString("logprism " + resolveVersion() + " - high-performance log analyzer\n\n")
	os.Stdout.WriteString("Usage:\n")
	os.Stdout.WriteString("  cat app.log | logprism [flags]\n")
	os.Stdout.WriteString("  logprism -input app.log [flags]\n\n")
	os.Stdout.WriteString("Flags:\n")
	os.Stdout.WriteString("  -input <path>      Read from file instead of stdin\n")
	os.Stdout.WriteString("  -output <path>     Write to file instead of stdout\n")
	os.Stdout.WriteString("  -filter k=v        Include lines (repeatable, e.g. -filter level=ERROR)\n")
	os.Stdout.WriteString("  -exclude k=v       Exclude lines (repeatable, e.g. -exclude level=DEBUG)\n")
	os.Stdout.WriteString("  -has <key>         Show only lines containing this key (repeatable)\n")
	os.Stdout.WriteString("  -map k=v           Map internal key to JSON key (e.g. -map level=severity)\n")
	os.Stdout.WriteString("  -sample <rate>     Sample percentage of logs (1-100)\n")
	os.Stdout.WriteString("  -C <n>             Show <n> lines of context before and after match\n")
	os.Stdout.WriteString("  -before <n>        Show <n> lines of context before match\n")
	os.Stdout.WriteString("  -after <n>         Show <n> lines of context after match\n")
	os.Stdout.WriteString("  -highlight <str>   Highlight a string in the output (repeatable)\n")
	os.Stdout.WriteString("  -pretty            Indent nested JSON values\n")
	os.Stdout.WriteString("  -color             Force colorized output even if not a terminal\n")
	os.Stdout.WriteString("  -no-color          Disable ANSI color output\n")
	os.Stdout.WriteString("  -v, -version       Show version and exit\n")
	os.Stdout.WriteString("  -h, --help         Show this help message\n\n")
	os.Stdout.WriteString("Operators (filter/exclude): =, >, <, >=, <=. Comma-separated values are\n")
	os.Stdout.WriteString("OR-combined, e.g. -filter level=ERROR,WARN.\n\n")
}

func parseArgs(args []string) options {
	opts := options{
		fieldMap: make(map[string]string),
	}
	addCond := func(isExclude bool, spec, flagName string) {
		key, conds, ok := parseFilterSpec(spec)
		if !ok || key == "" {
			os.Stderr.WriteString("logprism: " + flagName + " expects key=value (got " + spec + ")\n")
			os.Exit(1)
		}
		target := &opts.filters
		if isExclude {
			target = &opts.exclusions
		}
		found := false
		for i := range *target {
			if (*target)[i].key == key {
				(*target)[i].conds = append((*target)[i].conds, conds...)
				found = true
				break
			}
		}
		if !found {
			*target = append(*target, filterEntry{
				key:      key,
				keyBytes: []byte(key),
				conds:    conds,
			})
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-no-color", "--no-color":
			opts.noColor = true
		case "-color", "--color":
			opts.forceColor = true
		case "-pretty", "--pretty":
			opts.pretty = true
		case "-v", "-version", "--version":
			os.Stdout.WriteString("logprism " + resolveVersion() + "\n")
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
				addCond(false, args[i+1], "-filter")
				i++
			}
		case "-exclude", "--exclude":
			if i+1 < len(args) {
				addCond(true, args[i+1], "-exclude")
				i++
			}
		case "-has", "--has":
			if i+1 < len(args) {
				opts.hasFields = append(opts.hasFields, []byte(args[i+1]))
				i++
			}
		case "-sample", "--sample":
			if i+1 < len(args) {
				opts.sampleRate = parsePositiveInt(args[i+1])
				i++
			}
		case "-map", "--map":
			if i+1 < len(args) {
				f := args[i+1]
				idx := strings.Index(f, "=")
				if idx > 0 {
					opts.fieldMap[f[:idx]] = f[idx+1:]
				}
				i++
			}
		case "-C":
			if i+1 < len(args) {
				v := parsePositiveInt(args[i+1])
				opts.contextBefore = v
				opts.contextAfter = v
				i++
			}
		case "-before":
			if i+1 < len(args) {
				opts.contextBefore = parsePositiveInt(args[i+1])
				i++
			}
		case "-after":
			if i+1 < len(args) {
				opts.contextAfter = parsePositiveInt(args[i+1])
				i++
			}
		case "-highlight", "--highlight":
			if i+1 < len(args) {
				opts.highlights = append(opts.highlights, args[i+1])
				i++
			}
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		default:
			os.Stderr.WriteString("logprism: unknown argument: " + arg + "\n")
			os.Stderr.WriteString("Run 'logprism --help' for usage.\n")
			os.Exit(2)
		}
	}
	return opts
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

type extraField struct {
	key   []byte
	val   []byte
	isStr bool
}
type extraFields []extraField

func (f extraFields) Len() int           { return len(f) }
func (f extraFields) Less(i, j int) bool { return bytes.Compare(f[i].key, f[j].key) < 0 }
func (f extraFields) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

type ringBuffer struct {
	lines [][]byte
	head  int
	full  bool
}

func (rb *ringBuffer) add(line []byte) {
	if len(rb.lines) == 0 {
		return
	}

	l := make([]byte, len(line))
	copy(l, line)
	rb.lines[rb.head] = l
	rb.head = (rb.head + 1) % len(rb.lines)
	if rb.head == 0 {
		rb.full = true
	}
}

type logRecord struct {
	time, level, service, msg, traceID                []byte
	hasTime, hasLevel, hasService, hasMsg, hasTraceID bool
	extras                                            extraFields
}

type jsonScanner struct {
	data []byte
	pos  int
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

func (s *jsonScanner) startObject() bool {
	s.skipWhitespace()
	if s.pos >= len(s.data) || s.data[s.pos] != '{' {
		return false
	}
	s.pos++
	return true
}

func resetRecord(rec *logRecord) {
	rec.time = nil
	rec.level = nil
	rec.service = nil
	rec.msg = nil
	rec.traceID = nil
	rec.hasTime = false
	rec.hasLevel = false
	rec.hasService = false
	rec.hasMsg = false
	rec.hasTraceID = false
	rec.extras = rec.extras[:0]
}

func (s *jsonScanner) readString() ([]byte, bool) {
	if s.pos >= len(s.data) || s.data[s.pos] != '"' {
		return nil, false
	}
	s.pos++
	start := s.pos
	for s.pos < len(s.data) {
		if s.data[s.pos] == '"' {
			esc := 0
			for p := s.pos - 1; p >= 0 && s.data[p] == '\\'; p-- {
				esc++
			}
			if esc%2 == 0 {
				res := s.data[start:s.pos]
				s.pos++
				return res, true
			}
		}
		s.pos++
	}
	return nil, false
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
		if c == '"' {
			esc := 0
			for p := s.pos - 1; p >= 0 && s.data[p] == '\\'; p-- {
				esc++
			}
			if esc%2 == 0 {
				inString = !inString
			}
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

func extractRecord(s *jsonScanner, opts options, rec *logRecord) {
	var bTime, bLevel, bService, bMsg []byte
	if k, ok := opts.fieldMap["time"]; ok {
		bTime = []byte(k)
	}
	if k, ok := opts.fieldMap["level"]; ok {
		bLevel = []byte(k)
	}
	if k, ok := opts.fieldMap["service"]; ok {
		bService = []byte(k)
	}
	if k, ok := opts.fieldMap["msg"]; ok {
		bMsg = []byte(k)
	}

	for {
		key, val, isStr, ok := s.nextField()
		if !ok {
			break
		}

		if bytes.Equal(key, keyTime) || (len(bTime) > 0 && bytes.Equal(key, bTime)) {
			rec.time, rec.hasTime = val, true
		} else if bytes.Equal(key, keyLevel) || (len(bLevel) > 0 && bytes.Equal(key, bLevel)) {
			rec.level, rec.hasLevel = val, true
		} else if bytes.Equal(key, keyService) || (len(bService) > 0 && bytes.Equal(key, bService)) {
			rec.service, rec.hasService = val, true
		} else if bytes.Equal(key, keyMsg) || bytes.Equal(key, keyMessage) || (len(bMsg) > 0 && bytes.Equal(key, bMsg)) {
			if !rec.hasMsg {
				rec.msg, rec.hasMsg = val, true
			}
		} else if bytes.Equal(key, keyTraceID) {
			rec.traceID, rec.hasTraceID = val, true
		} else {
			rec.extras = append(rec.extras, extraField{key, val, isStr})
		}
	}
}

func matchCond(actual []byte, c filterCond) bool {
	if c.op == cmpEq {
		return bytes.Equal(actual, c.val)
	}

	target := int64(0)
	neg := false
	start := 0
	if len(c.val) > 0 && c.val[0] == '-' {
		neg = true
		start = 1
	}
	for i := start; i < len(c.val); i++ {
		if c.val[i] < '0' || c.val[i] > '9' {
			return bytes.Equal(actual, c.val)
		}
		target = target*10 + int64(c.val[i]-'0')
	}
	if neg {
		target = -target
	}
	av := int64(0)
	aneg := false
	hasDigit := false
	for i := 0; i < len(actual); i++ {
		if actual[i] == '-' && !hasDigit {
			aneg = true
			continue
		}
		if actual[i] >= '0' && actual[i] <= '9' {
			av = av*10 + int64(actual[i]-'0')
			hasDigit = true
		} else if hasDigit {
			break
		}
	}
	if !hasDigit {
		return false
	}
	if aneg {
		av = -av
	}

	switch c.op {
	case cmpGT:
		return av > target
	case cmpLT:
		return av < target
	case cmpGTE:
		return av >= target
	case cmpLTE:
		return av <= target
	}
	return false
}

func matchAnyCond(actual []byte, conds []filterCond) bool {
	for i := range conds {
		if matchCond(actual, conds[i]) {
			return true
		}
	}
	return false
}

func matchFilters(rec *logRecord, opts options) bool {
	for _, hf := range opts.hasFields {
		found := false
		if bytes.Equal(hf, keyTime) && rec.hasTime {
			found = true
		} else if bytes.Equal(hf, keyLevel) && rec.hasLevel {
			found = true
		} else if bytes.Equal(hf, keyService) && rec.hasService {
			found = true
		} else if bytes.Equal(hf, keyMsg) && rec.hasMsg {
			found = true
		} else if bytes.Equal(hf, keyTraceID) && rec.hasTraceID {
			found = true
		}
		if !found {
			for _, e := range rec.extras {
				if bytes.Equal(e.key, hf) {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	for _, fe := range opts.filters {
		found := false
		if fe.key == "time" && rec.hasTime {
			found = matchAnyCond(rec.time, fe.conds)
		} else if fe.key == "level" && rec.hasLevel {
			found = matchAnyCond(rec.level, fe.conds)
		} else if fe.key == "service" && rec.hasService {
			found = matchAnyCond(rec.service, fe.conds)
		} else if fe.key == "msg" && rec.hasMsg {
			found = matchAnyCond(rec.msg, fe.conds)
		} else if fe.key == "trace_id" && rec.hasTraceID {
			found = matchAnyCond(rec.traceID, fe.conds)
		}

		if !found {
			for _, e := range rec.extras {
				if bytes.Equal(e.key, fe.keyBytes) {
					found = matchAnyCond(e.val, fe.conds)
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	for _, fe := range opts.exclusions {
		excluded := false
		if fe.key == "time" && rec.hasTime {
			excluded = matchAnyCond(rec.time, fe.conds)
		} else if fe.key == "level" && rec.hasLevel {
			excluded = matchAnyCond(rec.level, fe.conds)
		} else if fe.key == "service" && rec.hasService {
			excluded = matchAnyCond(rec.service, fe.conds)
		} else if fe.key == "msg" && rec.hasMsg {
			excluded = matchAnyCond(rec.msg, fe.conds)
		} else if fe.key == "trace_id" && rec.hasTraceID {
			excluded = matchAnyCond(rec.traceID, fe.conds)
		}

		if !excluded {
			for _, e := range rec.extras {
				if bytes.Equal(e.key, fe.keyBytes) {
					excluded = matchAnyCond(e.val, fe.conds)
					break
				}
			}
		}
		if excluded {
			return false
		}
	}

	return true
}

func writeHighlighted(b *strings.Builder, val []byte, highlights []string, noColor bool) {
	if noColor || len(highlights) == 0 {
		b.Write(val)
		return
	}
	i := 0
	for i < len(val) {
		bestStart, bestLen := -1, 0
		for _, h := range highlights {
			if h == "" {
				continue
			}
			if idx := bytes.Index(val[i:], []byte(h)); idx >= 0 {
				s := i + idx
				if bestStart == -1 || s < bestStart {
					bestStart, bestLen = s, len(h)
				}
			}
		}
		if bestStart == -1 {
			b.Write(val[i:])
			return
		}
		b.Write(val[i:bestStart])
		b.WriteString(colorYellow)
		b.Write(val[bestStart : bestStart+bestLen])
		b.WriteString(colorReset)
		i = bestStart + bestLen
	}
}

func writeIndent(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		b.WriteString("  ")
	}
}

func writePretty(b *strings.Builder, data []byte, indent int) {
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if c == '"' {

			esc := 0
			for p := i - 1; p >= 0 && data[p] == '\\'; p-- {
				esc++
			}
			if esc%2 == 0 {
				inString = !inString
			}
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

func formatRecord(rec *logRecord, opts options, b *strings.Builder) {
	if rec.hasTime {
		if !opts.noColor {
			b.WriteString(colorGray)
		}
		b.Write(rec.time)
		if !opts.noColor {
			b.WriteString(colorReset)
		}
		b.WriteString(" | ")
	}

	if !opts.noColor && len(rec.level) > 0 {

		switch rec.level[0] | 0x20 {
		case 'e', 'f', 'p':
			b.WriteString(colorRed)
		case 'w':
			b.WriteString(colorYellow)
		case 'i':
			b.WriteString(colorBlue)
		case 'd':
			b.WriteString(colorGray)
		}
	}
	b.WriteString("[")
	b.Write(rec.level)
	b.WriteString("]")
	if !opts.noColor {
		b.WriteString(colorReset)
	}

	if rec.hasService {
		b.WriteString(" ")
		b.Write(rec.service)
	}

	b.WriteString(" | ")
	if rec.hasTraceID {
		writeHighlighted(b, rec.traceID, opts.highlights, opts.noColor)
	} else {
		b.WriteString("00000000-0000-0000-0000-000000000000")
	}
	b.WriteString(" | ")
	writeHighlighted(b, rec.msg, opts.highlights, opts.noColor)

	sort.Sort(rec.extras)
	for _, e := range rec.extras {
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
			writeHighlighted(b, e.val, opts.highlights, opts.noColor)
		}
	}

	b.WriteByte('\n')
}

func formatLine(line []byte, opts options, b *strings.Builder) bool {
	s := &jsonScanner{data: line}
	if !s.startObject() {
		if len(opts.filters) > 0 || len(opts.hasFields) > 0 || len(opts.exclusions) > 0 {
			return false
		}
		b.Write(line)
		b.WriteByte('\n')
		return true
	}

	var rec logRecord
	rec.extras = make(extraFields, 0, 8)
	extractRecord(s, opts, &rec)
	if !matchFilters(&rec, opts) {
		return false
	}

	formatRecord(&rec, opts, b)
	return true
}

func writeLine(w io.Writer, line []byte, opts options, b *strings.Builder) (bool, error) {
	b.Reset()
	if !formatLine(line, opts, b) {
		return false, nil
	}
	_, err := io.WriteString(w, b.String())
	return true, err
}

func (rb *ringBuffer) flush(w io.Writer, opts options, b *strings.Builder) error {
	size := len(rb.lines)
	if size == 0 {
		return nil
	}
	start := 0
	if rb.full {
		start = rb.head
	}

	ctxOpts := opts
	ctxOpts.filters = nil
	ctxOpts.hasFields = nil
	ctxOpts.exclusions = nil

	for i := 0; i < size; i++ {
		idx := (start + i) % size
		if rb.lines[idx] != nil {
			if _, err := writeLine(w, rb.lines[idx], ctxOpts, b); err != nil {
				return err
			}
			rb.lines[idx] = nil
		}
	}
	rb.head = 0
	rb.full = false
	return nil
}

func run(r io.Reader, w io.Writer, opts options) error {
	scanner := bufio.NewScanner(r)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var b strings.Builder
	afterCount := 0
	var rb *ringBuffer
	if opts.contextBefore > 0 {
		rb = &ringBuffer{lines: make([][]byte, opts.contextBefore)}
	}

	ctxOpts := opts
	ctxOpts.filters = nil
	ctxOpts.hasFields = nil
	ctxOpts.exclusions = nil

	var rec logRecord
	rec.extras = make(extraFields, 0, 8)

	sampleAcc := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if opts.sampleRate > 0 && opts.sampleRate < 100 {
			sampleAcc += opts.sampleRate
			if sampleAcc < 100 {
				continue
			}
			sampleAcc -= 100
		}

		s := &jsonScanner{data: line}
		if s.startObject() {
			resetRecord(&rec)
			extractRecord(s, opts, &rec)
			if matchFilters(&rec, opts) {
				if rb != nil {
					if err := rb.flush(w, opts, &b); err != nil {
						return err
					}
				}
				b.Reset()
				formatRecord(&rec, opts, &b)
				if _, err := io.WriteString(w, b.String()); err != nil {
					return err
				}
				afterCount = opts.contextAfter
			} else if afterCount > 0 {
				b.Reset()
				formatRecord(&rec, ctxOpts, &b)
				if _, err := io.WriteString(w, b.String()); err != nil {
					return err
				}
				afterCount--
			} else if rb != nil {
				rb.add(line)
			}
		} else {
			matched := len(opts.filters) == 0 && len(opts.hasFields) == 0 && len(opts.exclusions) == 0
			if matched {
				if rb != nil {
					if err := rb.flush(w, opts, &b); err != nil {
						return err
					}
				}
				if _, err := w.Write(line); err != nil {
					return err
				}
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return err
				}
				afterCount = opts.contextAfter
			} else if afterCount > 0 {
				if _, err := w.Write(line); err != nil {
					return err
				}
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return err
				}
				afterCount--
			} else if rb != nil {
				rb.add(line)
			}
		}
	}

	return scanner.Err()
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
		if isFile && !opts.forceColor {
			opts.noColor = true
		} else if !opts.forceColor {
			if fi, err := os.Stdout.Stat(); err == nil {
				if (fi.Mode() & os.ModeCharDevice) == 0 {
					opts.noColor = true
				}
			}
		}
	}

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
