package toml

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kezhuw/toml/internal/types"
)

const (
	eof = 0
)

type scanner func(*parser) scanner

type environment struct {
	env  types.Environment
	path string
}

type parser struct {
	mark int

	pos     int
	line    int
	input   string
	backups []int

	root *types.Table
	envs []environment
	keys []string

	names []string // table name parsing

	str strParser
	num numParser

	scanners []scanner

	err error
}

type numParser struct {
	sign      string
	e         string
	esign     string
	integers  []string
	fractions []string
	exponents []string
}

func (p *numParser) reset() {
	p.sign = ""
	p.e = ""
	p.esign = ""
	p.integers = p.integers[:0]
	p.fractions = p.fractions[:0]
	p.exponents = p.exponents[:0]
}

func (p *numParser) pushInteger(part string) {
	p.integers = append(p.integers, part)
}

func (p *numParser) pushFraction(part string) {
	p.fractions = append(p.fractions, part)
}

func (p *numParser) pushExponent(part string) {
	p.exponents = append(p.exponents, part)
}

func (p *numParser) join(sep string) string {
	s := p.sign + strings.Join(p.integers, sep)
	if len(p.fractions) != 0 {
		s += "." + strings.Join(p.fractions, sep)
	}
	if p.e != "" {
		s += p.e + p.esign + strings.Join(p.exponents, sep)
	}
	return s
}

func (p *numParser) Float() (float64, error) {
	defer p.reset()
	s := p.join("")
	return strconv.ParseFloat(s, 64)
}

func (p *numParser) Integer() (int64, error) {
	defer p.reset()
	s := strings.Join(p.integers, "")
	if s != "0" && s[0] == '0' {
		return 0, fmt.Errorf("leading zero in integer %q", s)
	}
	s = p.sign + s
	return strconv.ParseInt(s, 10, 64)
}

type strParser struct {
	parts []string
}

func (s *strParser) reset() {
	s.parts = s.parts[:0]
}

func (s *strParser) push(str string) {
	if str != "" {
		s.parts = append(s.parts, str)
	}
}

func (s *strParser) join() string {
	defer s.reset()
	return strings.Join(s.parts, "")
}

func (p *parser) pushScanner(s scanner) {
	p.scanners = append(p.scanners, s)
}

func (p *parser) seqScanner(seqs ...scanner) scanner {
	if len(seqs) == 0 {
		panic("toml: scanner sequences can't be empty")
	}
	for i := len(seqs) - 1; i >= 0; i-- {
		p.scanners = append(p.scanners, seqs[i])
	}
	return p.popScanner()
}

func (p *parser) popScanner() (s scanner) {
	i := len(p.scanners) - 1
	p.scanners, s = p.scanners[:i], p.scanners[i]
	return s
}

func (p *parser) record(offset int) {
	p.mark = p.pos + offset
}

func (p *parser) slice(offset int) string {
	s := p.input[p.mark : p.pos+offset]
	p.mark = -1
	return s
}

func (p *parser) stepN(n int) {
	p.pos += n
	p.backups = append(p.backups, n)
}

func (p *parser) readRune() (r rune, n int) {
	r, n = utf8.DecodeRuneInString(p.input[p.pos:])
	if r == utf8.RuneError {
		if n == 1 {
			panic(p.errorf("invalid utf8 rune %#.4x", p.input[p.pos:]))
		}
		if n == 0 {
			r = eof
		}
	}
	p.stepN(n)
	return r, n
}

func (p *parser) peekRune() (rune, int) {
	r, n := p.readRune()
	p.unread()
	return r, n
}

func (p *parser) readByte() rune {
	if p.pos >= len(p.input) {
		p.stepN(0)
		return eof
	}
	r := rune(p.input[p.pos])
	p.stepN(1)
	return r
}

func (p *parser) peekByte() rune {
	if p.pos >= len(p.input) {
		return eof
	}
	return rune(p.input[p.pos])
}

func (p *parser) tryReadByte(r rune) bool {
	if p.peekByte() == r {
		p.stepN(1)
		return true
	}
	return false
}

func (p *parser) tryReadPrefix(str string) bool {
	if strings.HasPrefix(p.input[p.pos:], str) {
		p.stepN(len(str))
		return true
	}
	return false
}

func (p *parser) tryReadNewline() bool {
	r := p.readByte()
	if p.skipNewline(r) {
		return true
	}
	p.unread()
	return false
}

func (p *parser) unread() {
	i := len(p.backups) - 1
	rd := p.backups[i]
	p.backups = p.backups[:i]
	p.pos -= rd
}

func (p *parser) clearBackups() {
	p.backups = p.backups[:0]
}

func (p *parser) pushTableKey(key string) scanner {
	env, path := p.topEnv()
	if value, ok := env.(*types.Table).Elems[key]; ok {
		return p.errorScanner("table %s has key %s defined as %s", path, normalizeKey(key), value.Type())
	}
	p.keys = append(p.keys, key)
	return p.popScanner()
}

func (p *parser) topTableKey() string {
	return p.keys[len(p.keys)-1]
}

func (p *parser) popTableKey() (key string) {
	i := len(p.keys) - 1
	p.keys, key = p.keys[0:i], p.keys[i]
	return key
}

func (p *parser) setValue(value types.Value) scanner {
	env, path := p.topEnv()
	switch env := env.(type) {
	case *types.Array:
		if len(env.Elems) != 0 {
			if first := env.Elems[0]; first.Type() != value.Type() {
				return p.errorScanner("array %s expects element type %s, but got %s", path, first.Type(), value.Type())
			}
		}
		env.Elems = append(env.Elems, value)
	case *types.Table:
		key := p.popTableKey()
		env.Elems[key] = value
	}
	return p.popScanner()
}

func (p *parser) resetEnv(env types.Environment, path string) {
	p.envs = p.envs[:1]
	p.envs[0] = environment{env, path}
}

func (p *parser) pushEnv(new types.Environment) {
	env, path := p.topEnv()
	switch env := env.(type) {
	case *types.Table:
		path = combineKeyPath(path, p.topTableKey())
	case *types.Array:
		path = combineIndexPath(path, len(env.Elems))
	}
	p.envs = append(p.envs, environment{new, path})
}

func (p *parser) popEnv() (env types.Environment, path string) {
	i := len(p.envs) - 1
	environment := p.envs[i]
	p.envs = p.envs[:i]
	return environment.env, environment.path
}

func (p *parser) topEnv() (types.Environment, string) {
	env := p.envs[len(p.envs)-1]
	return env.env, env.path
}

func scanByte(r rune) scanner {
	return func(p *parser) scanner {
		if r1 := p.readByte(); r1 != r {
			return p.expectRune(r)
		}
		return p.popScanner()
	}
}

func scanDigit(p *parser) scanner {
	r := p.readByte()
	if !isDigit(r) {
		return p.expectStr("digit")
	}
	return p.popScanner()
}

func scanConsumeByte(pred func(r rune) bool) scanner {
	var s scanner
	s = func(p *parser) scanner {
		if r := p.readByte(); !pred(r) {
			p.unread()
			return p.popScanner()
		}
		return s
	}
	return s
}

var scanColon = scanByte(':')
var scanHash = scanByte('-')

func scanRecord(offset int) scanner {
	return func(p *parser) scanner {
		p.record(offset)
		return p.popScanner()
	}
}

var scanRecord0 = scanRecord(0)

func scanReturnString(p *parser, s string) scanner {
	p.str.push(s)
	return p.popScanner()
}

func scanUnicodeRune(p *parser, n int) scanner {
	p.record(0)
	for i := 0; i < n; i++ {
		if r := p.readByte(); !isHex(r) {
			return p.expectStr("hexadecimal digit")
		}
	}
	s := p.slice(0)
	codepoint, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return p.setError(err)
	}
	r := rune(codepoint)
	if !utf8.ValidRune(r) {
		return p.errorScanner("%s is not a valid utf8 rune", s)
	}
	return scanReturnString(p, string(r))
}

func scanEscapedRune(p *parser) scanner {
	r := p.readByte()
	switch r {
	case 'b':
		return scanReturnString(p, "\b")
	case 't':
		return scanReturnString(p, "\t")
	case 'n':
		return scanReturnString(p, "\n")
	case 'f':
		return scanReturnString(p, "\f")
	case 'r':
		return scanReturnString(p, "\r")
	case '"':
		return scanReturnString(p, "\"")
	case '\\':
		return scanReturnString(p, "\\")
	case 'u':
		return scanUnicodeRune(p, 4)
	case 'U':
		return scanUnicodeRune(p, 8)
	default:
		return p.expectStr("escaped sequence")
	}
}

func scanLiteral(p *parser) scanner {
	r, _ := p.readRune()
	switch r {
	case '\'':
		p.str.push(p.slice(-1))
		return p.popScanner()
	case '\r', '\n':
		return p.errorScanner("newline is not allowed in oneline string")
	case eof:
		return p.errorScanner("string without ending")
	default:
		return scanLiteral
	}
}

func scanMultiLineLiteral(p *parser) scanner {
	r, _ := p.readRune()
	switch r {
	case eof:
		return p.errorScanner("no ending for multi-line literal string")
	case '\'':
		if p.tryReadPrefix(`''`) {
			p.str.push(p.slice(-3))
			return p.popScanner()
		}
		fallthrough
	default:
		return scanMultiLineLiteral
	}
}

func scanMultiLineString(p *parser) scanner {
	r, _ := p.readRune()
	switch r {
	case '\\':
		p.str.push(p.slice(-1))
		if p.tryReadNewline() {
			return p.seqScanner(scanConsumeByte(func(r rune) bool { return isSpace(r) || p.skipNewline(r) }), scanRecord0, scanMultiLineString)
		}
		return p.seqScanner(scanEscapedRune, scanRecord0, scanMultiLineString)
	case eof:
		return p.errorScanner("multi-line basic string without ending")
	case '"':
		if p.tryReadPrefix(`""`) {
			p.str.push(p.slice(-3))
			return p.popScanner()
		}
		fallthrough
	default:
		return scanMultiLineString
	}
}

func scanComment(p *parser) scanner {
	r, _ := p.readRune()
	if r == eof || p.skipNewline(r) {
		return p.popScanner()
	}
	return scanComment
}

func scanArrayTableEnd(p *parser) scanner {
	i := len(p.names) - 1
	env, path := p.locateTable(p.names[:i])
	if env == nil {
		return nil
	}

	env, path = p.createTableArray(env, path, p.names[i])
	if env == nil {
		return nil
	}
	p.resetEnv(env, path)
	return scanTopEnd
}

func scanTableEnd(p *parser) scanner {
	i := len(p.names) - 1
	env, path := p.locateTable(p.names[:i])
	if env == nil {
		return nil
	}

	env, path = p.createTable(env, path, p.names[i])
	if env == nil {
		return nil
	}

	p.resetEnv(env, path)
	return scanTopEnd
}

func scanTableStart(p *parser) scanner {
	p.names = p.names[:0]
	if p.tryReadByte('[') {
		return p.seqScanner(scanTableNameStart, scanByte(']'), scanArrayTableEnd)
	}
	return p.seqScanner(scanTableNameStart, scanTableEnd)
}

func scanString(p *parser) scanner {
	r, _ := p.readRune()
	switch r {
	case '"':
		p.str.push(p.slice(-1))
		return p.popScanner()
	case '\\':
		p.str.push(p.slice(-1))
		return p.seqScanner(scanEscapedRune, scanRecord0, scanString)
	case '\r', '\n':
		return p.errorScanner("newline is not allowed in oneline string")
	case eof:
		return p.errorScanner("string without ending")
	default:
		return scanString
	}
}

func scanTableNameString(p *parser) scanner {
	s := p.str.join()
	p.appendTableName(s)
	return scanTableNameEnd
}

func scanTableNameStart(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanTableNameStart
	case r == '.' || r == ']':
		return p.errorScanner("table name must be non-empty")
	case r == '"':
		return p.seqScanner(scanRecord0, scanString, scanTableNameString)
	case isBareKeyChar(r):
		p.record(-1)
		return scanTableNameInside
	default:
		return p.expectStr("table name")
	}
}

func (p *parser) appendTableName(name string) {
	p.names = append(p.names, name)
}

func scanTableNameInside(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		p.appendTableName(p.slice(-1))
		return scanTableNameEnd
	case isBareKeyChar(r):
		return scanTableNameInside
	case r == '.':
		p.appendTableName(p.slice(-1))
		return scanTableNameStart
	case r == ']':
		p.appendTableName(p.slice(-1))
		return p.popScanner()
	default:
		return p.expectStr("bare character")
	}
}

func scanTableNameEnd(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanTableNameEnd
	case r == '.':
		return scanTableNameStart
	case r == ']':
		return p.popScanner()
	default:
		return p.expectStr("'.' or ']'")
	}
}

func (p *parser) skipNewline(r rune) bool {
	switch r {
	case '\r':
		p.tryReadByte('\n')
		fallthrough
	case '\n':
		p.line++
		return true
	}
	return false
}

func scanTop(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanTop
	case p.skipNewline(r):
		return scanTop
	case r == '#':
		return p.seqScanner(scanComment, scanTop)
	case r == '[':
		return scanTableStart
	case r == eof:
		return nil
	default:
		p.unread()
		// Resumed after a whole a key/value pair was scanned.
		return p.seqScanner(scanTableField, scanTopEnd)
	}
}

func scanTopEnd(p *parser) scanner {
	r := p.readByte()
	switch {
	case r == eof:
		fallthrough
	case p.skipNewline(r):
		return scanTop
	case isSpace(r):
		return scanTopEnd
	case r == '#':
		return p.seqScanner(scanComment, scanTop)
	default:
		return p.expectStr("new line, comment or EOF")
	}
}

func scanInlineTableFieldEnd(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanInlineTableFieldEnd
	case r == ',':
		return p.seqScanner(scanTableField, scanInlineTableFieldEnd)
	case r == '}':
		t, _ := p.popEnv()
		return p.setValue(t)
	default:
		return p.expectStr("inline table separator ',' or terminator '}'")
	}
}

func scanInlineTableStart(p *parser) scanner {
	r := p.readByte()
	switch {
	case p.skipNewline(r):
		return p.errorScanner("newlines are not allowed in inline table")
	case isSpace(r):
		return scanInlineTableStart
	case r == ',':
		return p.errorScanner("unexpected ',' in inline table")
	case r == '}':
		t := &types.Table{Elems: make(map[string]types.Value)}
		return p.setValue(t)
	default:
		p.unread()
		p.pushEnv(&types.Table{Elems: make(map[string]types.Value)})
		return p.seqScanner(scanTableField, scanInlineTableFieldEnd)
	}
}

func scanFloatFraction(p *parser) scanner {
	r := p.readByte()
	switch {
	case isDigit(r):
		return scanFloatFraction
	case r == '_':
		p.num.pushFraction(p.slice(-1))
		return p.seqScanner(scanRecord0, scanDigit, scanFloatFraction)
	case r == '.':
		return p.errorScanner("decimal point already read")
	case r == 'e' || r == 'E':
		p.num.e = string(r)
		p.num.pushFraction(p.slice(-1))
		return scanFloatExponentSign
	default:
		p.unread()
		p.num.pushFraction(p.slice(0))
		return setFloatValue(p)
	}
}

func scanFloatExponent(p *parser) scanner {
	r := p.readByte()
	switch {
	case isDigit(r):
		return scanFloatExponent
	case r == '_':
		p.num.pushExponent(p.slice(-1))
		return p.seqScanner(scanRecord0, scanDigit, scanFloatExponent)
	default:
		p.unread()
		p.num.pushExponent(p.slice(0))
		return setFloatValue(p)
	}
}

func scanFloatExponentSign(p *parser) scanner {
	switch r := p.readByte(); r {
	case '+', '-':
		p.num.esign = string(r)
	default:
		p.unread()
	}
	return p.seqScanner(scanRecord0, scanDigit, scanFloatExponent)
}

func scanNumber(p *parser) scanner {
	r := p.readByte()
	switch {
	case isDigit(r):
		return scanNumber
	case r == '_':
		p.num.pushInteger(p.slice(-1))
		return p.seqScanner(scanRecord0, scanDigit, scanNumber)
	case r == '.':
		p.num.pushInteger(p.slice(-1))
		return p.seqScanner(scanRecord0, scanDigit, scanFloatFraction)
	case r == 'e' || r == 'E':
		p.num.e = string(r)
		p.num.pushInteger(p.slice(-1))
		return p.seqScanner(scanRecord0, scanDigit, scanFloatExponent)
	default:
		p.unread()
		p.num.pushInteger(p.slice(0))
		return setIntegerValue(p)
	}
}

func scanNumberStart(p *parser) scanner {
	return p.seqScanner(scanRecord0, scanDigit, scanNumber)
}

func setFloatValue(p *parser) scanner {
	f, err := p.num.Float()
	if err != nil {
		return p.setError(err)
	}
	return p.setValue(types.Float(f))
}

func setIntegerValue(p *parser) scanner {
	i, err := p.num.Integer()
	if err != nil {
		return p.setError(err)
	}
	return p.setValue(types.Integer(i))
}

func setStringValue(p *parser) scanner {
	s := p.str.join()
	return p.setValue(types.String(s))
}

func scanStringStart(p *parser) scanner {
	if p.tryReadPrefix(`""`) {
		p.tryReadNewline()
		return p.seqScanner(scanRecord0, scanMultiLineString, setStringValue)
	}
	return p.seqScanner(scanRecord0, scanString, setStringValue)
}

func scanLiteralStart(p *parser) scanner {
	if p.tryReadPrefix(`''`) {
		p.tryReadNewline()
		return p.seqScanner(scanRecord0, scanMultiLineLiteral, setStringValue)
	}
	return p.seqScanner(scanRecord0, scanLiteral, setStringValue)
}

func scanArrayValue(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r) || p.skipNewline(r):
		return scanArrayValue
	case r == '#':
		return p.seqScanner(scanComment, scanArrayValue)
	case r == ',':
		return p.errorScanner("no array element before separator")
	case r == ']':
		p.unread()
		return p.popScanner()
	default:
		p.unread()
		return scanValue
	}
}

func scanArrayStart(p *parser) scanner {
	p.pushEnv(&types.Array{Closed: true, Elems: make([]types.Value, 0)})
	return p.seqScanner(scanArrayValue, scanArrayEnd)
}

func scanArrayEnd(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanArrayEnd
	case r == '#':
		return p.seqScanner(scanComment, scanArrayEnd)
	case r == ',':
		return p.seqScanner(scanArrayValue, scanArrayEnd)
	case r == ']':
		env, _ := p.popEnv()
		return p.setValue(env)
	default:
		return p.expectStr("',' or ']'")
	}
}

func scanValue(p *parser) scanner {
	r := p.readByte()
	switch {
	case r == '[':
		return scanArrayStart
	case r == '{':
		return scanInlineTableStart
	case r == 't':
		if !p.tryReadPrefix("rue") {
			return p.expectStr("true")
		}
		return p.setValue(types.Boolean(true))
	case r == 'f':
		if !p.tryReadPrefix("alse") {
			return p.expectStr("false")
		}
		return p.setValue(types.Boolean(false))
	case r == '"':
		return scanStringStart
	case r == '\'':
		return scanLiteralStart
	case r == '+' || r == '-':
		p.num.sign = string(r)
		return scanNumberStart
	case isDigit(r):
		p.record(-1)
		return scanNumberOrDate
	case isSpace(r):
		return scanValue
	default:
		return p.expectStr("value")
	}
}

func scanDateValue(p *parser, suffix string) scanner {
	s := p.slice(0) + suffix
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return p.errorScanner(err.Error())
	}
	return p.setValue(types.Datetime(t))
}

func scanDateEnd(p *parser) scanner {
	return scanDateValue(p, "")
}

func scanDateTimeEnd(p *parser) scanner {
	r := p.readByte()
	switch r {
	case 'Z':
		return scanDateEnd(p)
	case '-':
		return p.seqScanner(scanDigit, scanDigit, scanColon, scanDigit, scanDigit, scanDateEnd)
	default:
		p.unread()
		return scanDateValue(p, "Z")
	}
}

func scanDateTimeFraction(p *parser) scanner {
	r := p.readByte()
	switch r {
	case '.':
		for isDigit(p.readByte()) {
		}
		fallthrough
	default:
		p.unread()
		return scanDateTimeEnd
	}
}

func scanDateTime(p *parser) scanner {
	r := p.readByte()
	switch r {
	case 'T':
		return p.seqScanner(scanDigit, scanDigit, scanColon, scanDigit, scanDigit, scanColon, scanDigit, scanDigit, scanDateTimeFraction)
	default:
		p.unread()
		return scanDateValue(p, "T00:00:00Z")
	}
}

func scanNumberOrDate(p *parser) scanner {
	r := p.readByte()
	switch {
	case r == '-':
		return p.seqScanner(scanDigit, scanDigit, scanHash, scanDigit, scanDigit, scanDateTime)
	case isDigit(r):
		return scanNumberOrDate
	default:
		p.unread()
		return scanNumber
	}
}

func scanFieldAssign(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanFieldAssign
	case r == '=':
		return scanValue
	default:
		return p.expectRune('=')
	}
}

func scanBareKey(p *parser) scanner {
	r := p.readByte()
	switch {
	case isBareKeyChar(r):
		return scanBareKey
	case isSpace(r):
		key := p.slice(-1)
		p.pushScanner(scanFieldAssign)
		return p.pushTableKey(key)
	case r == '=':
		key := p.slice(-1)
		p.pushScanner(scanValue)
		return p.pushTableKey(key)
	default:
		return p.expectStr("bare character")
	}
}

func scanKeyEnd(p *parser) scanner {
	key := p.str.join()
	p.pushScanner(scanFieldAssign)
	return p.pushTableKey(key)
}

func scanTableField(p *parser) scanner {
	r := p.readByte()
	switch {
	case isSpace(r):
		return scanTableField
	case isBareKeyChar(r):
		p.record(-1)
		return scanBareKey
	case r == '=':
		return p.errorScanner("key must be non-empty")
	case r == '"':
		return p.seqScanner(scanRecord0, scanString, scanKeyEnd)
	case r == '\'':
		return p.seqScanner(scanRecord0, scanLiteral, scanKeyEnd)
	default:
		return p.expectStr("table field")
	}
}

type char rune

func (c char) String() string {
	if c == eof {
		return "EOF"
	}
	return fmt.Sprintf("%q", rune(c))
}

func (p *parser) expectRune(r rune) scanner {
	p.unread()
	got, _ := p.peekRune()
	p.err = &ParseError{p.line, p.pos, fmt.Errorf("expect %q, got %s", r, char(got))}
	return nil
}

func (p *parser) expectStr(str string) scanner {
	p.unread()
	got, _ := p.peekRune()
	p.err = &ParseError{p.line, p.pos, fmt.Errorf("expect %s, got %s", str, char(got))}
	return nil
}

func (p *parser) errorf(format string, args ...interface{}) error {
	return &ParseError{p.line, p.pos, fmt.Errorf(format, args...)}
}

func (p *parser) errorScanner(format string, args ...interface{}) scanner {
	p.err = p.errorf(format, args...)
	return nil
}

func (p *parser) setError(err error) scanner {
	p.err = &ParseError{p.line, p.pos, err}
	return nil
}

func normalizeKey(key string) string {
	for _, r := range key {
		if !isBareKeyChar(r) {
			return strconv.Quote(key)
		}
	}
	return key
}

func combineKeyPath(path, key string) string {
	key = normalizeKey(key)
	if path == "" {
		return key
	}
	return path + "." + key
}

func combineIndexPath(path string, i int) string {
	return fmt.Sprintf("%s[%d]", path, i)
}

func (p *parser) locateTable(names []string) (t *types.Table, path string) {
	t = p.root
	for _, name := range names {
		path = combineKeyPath(path, name)
		switch v := t.Elems[name].(type) {
		case nil:
			ti := &types.Table{Implicit: true, Elems: make(map[string]types.Value)}
			t.Elems[name] = ti
			t = ti
		case *types.Table:
			t = v
		case *types.Array:
			if v.Closed {
				panic(p.errorf("%s was defined as array", path))
			}
			i := len(v.Elems) - 1
			t = v.Elems[i].(*types.Table)
			path = combineIndexPath(path, i)
		default:
			panic(p.errorf("%s was defined as %s", path, v.Type()))
		}
	}
	return t, path
}

func (p *parser) createTable(env *types.Table, path string, name string) (*types.Table, string) {
	path = combineKeyPath(path, name)
	switch v := env.Elems[name].(type) {
	case nil:
		t := &types.Table{Elems: make(map[string]types.Value)}
		env.Elems[name] = t
		return t, path
	case *types.Table:
		if !v.Implicit {
			panic(p.errorf("table %s was defined twice", path))
		}
		v.Implicit = false
		return v, path
	default:
		panic(p.errorf("%s was defined as %s", path, v.Type()))
	}
}

func (p *parser) createTableArray(env *types.Table, path string, name string) (*types.Table, string) {
	path = combineKeyPath(path, name)
	t := &types.Table{Elems: make(map[string]types.Value)}
	switch v := env.Elems[name].(type) {
	case nil:
		env.Elems[name] = &types.Array{Elems: []types.Value{t}}
	case *types.Array:
		if v.Closed {
			panic(p.errorf("%s was defined as array", path))
		}
		v.Elems = append(v.Elems, t)
	default:
		panic(p.errorf("%s was defined as %s", path, v.Type()))
	}
	return t, path
}

func (p *parser) errRecover(errp *error) {
	if r := recover(); r != nil {
		switch err := r.(type) {
		default:
			panic(r)
		case runtime.Error:
			panic(r)
		case *ParseError:
			*errp = err
		case error:
			*errp = &ParseError{p.line, p.pos, err}
		}
	}
}

func (p *parser) parse() (err error) {
	defer p.errRecover(&err)
	scanner := scanTop
	for scanner != nil {
		p.clearBackups()
		scanner = scanner(p)
	}
	return p.err
}

func newParser(t *types.Table, s string) *parser {
	return &parser{
		mark:  -1,
		line:  1,
		input: s,
		root:  t,
		envs:  []environment{{t, ""}},
	}
}

// Parse parses TOML document from data, and represents it in types.Table.
func parse(data []byte) (*types.Table, error) {
	root := &types.Table{Elems: make(map[string]types.Value)}
	p := newParser(root, string(data))
	err := p.parse()
	if err != nil {
		return nil, err
	}
	return root, nil
}
