package toml

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"fmt"
	"go/ast"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type encodeState struct {
	bytes.Buffer
}

type InvalidMarshalError struct {
	Type reflect.Type
}

func (e *InvalidMarshalError) Error() string {
	return "toml: Marshal(nil " + e.Type.String() + ")"
}

type InvalidUTF8Error struct {
	S string
}

func (e *InvalidUTF8Error) Error() string {
	return "toml: invalid UTF-8 in string: " + strconv.Quote(e.S)
}

type DuplicatedKeyError struct {
	Path string
	Key  string
}

func (e *DuplicatedKeyError) Error() string {
	return "toml: key[" + normalizeKey(e.Key) + "] exists in table[" + e.Path + "]"
}

type MarshalTypeError struct {
	Type reflect.Type
	As   string
}

func (e *MarshalTypeError) Error() string {
	return "toml: cannot marshal Go value of type " + e.Type.String() + " as toml " + e.As
}

type MarshalArrayError struct {
	Expected string
	Got      string
}

func (e *MarshalArrayError) Error() string {
	return "toml: expect array of element type: " + e.Expected + ", got: " + e.Got
}

type MarshalValueError struct {
	Value string
	Type  reflect.Type
}

func (e *MarshalValueError) Error() string {
	return "toml: cannot marshal `" + e.Value + "` of Go type " + e.Type.String()
}

func indirectPtr(v reflect.Value) (encoding.TextMarshaler, reflect.Value) {
	for (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && !v.IsNil() {
		v = v.Elem()
	}
	if v.CanInterface() {
		if i, ok := v.Interface().(encoding.TextMarshaler); ok {
			return i, v
		}
	}
	if v.Kind() != reflect.Ptr && v.CanAddr() {
		p := v.Addr()
		if i, ok := p.Interface().(encoding.TextMarshaler); ok {
			return i, v
		}
	}
	return nil, v
}

type field struct {
	key   string
	value reflect.Value
}

type table struct {
	Inline bool
	Path   string
	sep    string
	keys   map[string]struct{}
	tables []field // table or array of tables
}

func (t *table) fieldSep() string {
	sep := t.sep
	if sep == "" {
		t.sep = "\n"
	} else if sep == " " {
		t.sep = ", "
	}
	return sep
}

func (t *table) tableSep() string {
	sep := t.sep
	if sep == "" {
		t.sep = "\n"
	} else {
		return "\n\n"
	}
	return sep
}

func (t *table) recordKey(key string) {
	if t.keys == nil {
		return
	}
	if _, ok := t.keys[key]; ok {
		panic(&DuplicatedKeyError{Path: t.Path, Key: key})
	}
	t.keys[key] = struct{}{}
}

func (t *table) appendStructField(key string, value reflect.Value) {
	t.recordKey(key)
	t.tables = append(t.tables, field{key, value})
}

var (
	datetimeType = reflect.TypeOf((*time.Time)(nil)).Elem()
)

type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "TODO"
}

type stringValues []reflect.Value

func (sv stringValues) Len() int           { return len(sv) }
func (sv stringValues) Swap(i, j int)      { sv[i], sv[j] = sv[j], sv[i] }
func (sv stringValues) Less(i, j int) bool { return sv[i].String() < sv[j].String() }

func (e *encodeState) WriteSepKeyAssign(sep, key string) {
	e.WriteString(sep)
	e.WriteString(normalizeKey(key))
	e.WriteString(" = ")
}

func isASCIIString(s string) bool {
	for _, r := range s {
		if r >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func quoteBasic(s string, quotes string, ASCIIOnly bool) string {
	origin := s
	multiline := quotes == `"""`
	pendingQuotes := ""
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(s)/2)
	for width := 0; len(s) > 0; s = s[width:] {
		width = 1
		r := rune(s[0])
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if r == utf8.RuneError {
			panic(&InvalidUTF8Error{origin})
		}
		if r == '"' {
			if !multiline || len(pendingQuotes) == 2 {
				buf = append(buf, `\"`...)
				continue
			}
			pendingQuotes += "\""
			continue
		} else if pendingQuotes != "" {
			buf = append(buf, pendingQuotes...)
		}
		if r == '\\' {
			buf = append(buf, `\\`...)
			continue
		}
		if multiline && (r == '\r' || r == '\n') {
			buf = append(buf, byte(r))
			continue
		}
		if ASCIIOnly {
			if r < utf8.RuneSelf && strconv.IsPrint(r) {
				buf = append(buf, byte(r))
				continue
			}
		} else if strconv.IsPrint(r) {
			n := utf8.EncodeRune(runeTmp[:], r)
			buf = append(buf, runeTmp[:n]...)
			continue
		}
		switch {
		case r == '\b':
			buf = append(buf, `\b`...)
		case r == '\t':
			buf = append(buf, `\t`...)
		case r == '\f':
			buf = append(buf, `\f`...)
		case r == '\r':
			buf = append(buf, `\r`...)
		case r == '\n':
			buf = append(buf, `\n`...)
		case r < 0x10000:
			buf = append(buf, `\u`...)
			buf = append(buf, fmt.Sprintf("%04x", r)...)
		default:
			buf = append(buf, `\U`...)
			buf = append(buf, fmt.Sprintf("%08x", r)...)
		}
	}
	return string(buf)
}

func (e *encodeState) marshalBasicString(s string, options tagOptions) {
	e.WriteByte('"')
	e.WriteString(quoteBasic(s, "\"", options.Has("ascii")))
	e.WriteByte('"')
}

func (e *encodeState) marshalMultilineString(s string, options tagOptions) {
	e.WriteString(`"""`)
	e.WriteByte('\n')
	e.WriteString(quoteBasic(s, `"""`, options.Has("ascii")))
	e.WriteString(`"""`)
}

func (e *encodeState) marshalLiteralString(s string, options tagOptions) {
	if options.Has("ascii") && !isASCIIString(s) {
		e.marshalBasicString(s, options)
		return
	}
	e.WriteByte('\'')
	e.WriteString(s)
	e.WriteByte('\'')
}

func (e *encodeState) marshalMultilineLiteral(s string, options tagOptions) {
	if options.Has("ascii") && !isASCIIString(s) {
		e.marshalMultilineString(s, options)
		return
	}
	e.WriteString(`'''`)
	e.WriteByte('\n')
	e.WriteString(s)
	e.WriteString(`'''`)
}

func (e *encodeState) marshalStringValue(s string, options tagOptions) {
	if options.Has("multiline") {
		if options.Has("literal") {
			if strings.Index(s, `'''`) == -1 {
				e.marshalMultilineLiteral(s, options)
				return
			}
		}
		e.marshalMultilineString(s, options)
		return
	}
	if options.Has("literal") {
		if strings.IndexAny(s, "'\r\n") == -1 {
			e.marshalLiteralString(s, options)
			return
		}
	}
	e.marshalBasicString(s, options)
}

func stringQuote(options tagOptions) string {
	if options.Has("multiline") {
		if options.Has("literal") {
			return `'''`
		}
		return `"""`
	}
	if options.Has("literal") {
		return `'`
	}
	return `"`
}

func (e *encodeState) marshalBytesValue(b []byte, options tagOptions) {
	quote := stringQuote(options)
	e.WriteString(quote)
	enc := base64.NewEncoder(base64.StdEncoding, e)
	enc.Write(b)
	enc.Close()
	e.WriteString(quote)
}

func (e *encodeState) marshalTextValue(ti encoding.TextMarshaler, options tagOptions) {
	b, err := ti.MarshalText()
	if err != nil {
		panic(err)
	}
	e.marshalStringValue(string(b), options)
}

func (e *encodeState) marshalRawValue(v string, options tagOptions) {
	if options.Has("string") || options.Has("literal") {
		e.marshalStringValue(v, options)
		return
	}
	e.WriteString(v)
}

func (e *encodeState) marshalBoolValue(b bool, options tagOptions) {
	e.marshalRawValue(strconv.FormatBool(b), options)
}

func (e *encodeState) marshalIntValue(i int64, options tagOptions) {
	e.marshalRawValue(strconv.FormatInt(i, 10), options)
}

func (e *encodeState) marshalUintValue(u uint64, options tagOptions) {
	e.marshalRawValue(strconv.FormatUint(u, 10), options)
}

func (e *encodeState) marshalFloatValue(f float64, options tagOptions) {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if strings.IndexAny(s, ".e") == -1 {
		s += ".0"
	}
	e.marshalRawValue(s, options)
}

func (e *encodeState) marshalBoolField(t *table, key string, b bool, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalBoolValue(b, options)
}

func (e *encodeState) marshalIntField(t *table, key string, i int64, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalIntValue(i, options)
}

func (e *encodeState) marshalUintField(t *table, key string, u uint64, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalUintValue(u, options)
}

func (e *encodeState) marshalFloatField(t *table, key string, f float64, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalFloatValue(f, options)
}

func (e *encodeState) marshalStringField(t *table, key string, value string, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalStringValue(value, options)
}

func (e *encodeState) marshalDatetimeValue(value reflect.Value, options tagOptions) {
	t := value.Convert(datetimeType).Interface().(time.Time)
	s := t.Format(time.RFC3339Nano)
	e.marshalRawValue(s, options)
}

func (e *encodeState) marshalDatetimeField(t *table, key string, value reflect.Value, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalDatetimeValue(value, options)
}

func (e *encodeState) marshalTextField(t *table, key string, ti encoding.TextMarshaler, options tagOptions) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalTextValue(ti, options)
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Type().Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return v.Bool() == false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}

func checkArrayElemType(elemType string) func(newElemType string) {
	return func(newType string) {
		if elemType == "" {
			elemType = newType
		} else if elemType != newType {
			panic(&MarshalArrayError{Expected: elemType, Got: newType})
		}
	}
}

func isTableType(typ reflect.Type) bool {
	return typ.Kind() == reflect.Map || typ.Kind() == reflect.Struct
}

func (e *encodeState) marshalArrayValue(path string, v reflect.Value, options tagOptions) string {
	if v.Type().Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
		e.marshalBytesValue(v.Bytes(), options)
		return "string"
	}

	sep := " "
	e.WriteByte('[')
	check := checkArrayElemType("")
	for i, n := 0, v.Len(); i < n; i++ {
		e.WriteString(sep)
		ti, elem := indirectPtr(v.Index(i))
		switch {
		case elem.Type() == datetimeType,
			elem.Type().ConvertibleTo(datetimeType) && options.Has("datetime"):
			check("datetime")
			e.marshalDatetimeValue(elem, options)
			continue
		case ti != nil:
			check("string")
			e.marshalTextValue(ti, options)
			continue
		}
		switch elem.Kind() {
		case reflect.Bool:
			check("boolean")
			e.marshalBoolValue(elem.Bool(), options)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			check("integer")
			e.marshalIntValue(elem.Int(), options)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			check("integer")
			e.marshalUintValue(elem.Uint(), options)
		case reflect.Float32, reflect.Float64:
			check("float")
			e.marshalFloatValue(elem.Float(), options)
		case reflect.String:
			check("string")
			e.marshalStringValue(elem.String(), options)
		case reflect.Array, reflect.Slice:
			check(e.marshalArrayValue(combineIndexPath(path, i), elem, options))
		case reflect.Map:
			check("table")
			e.marshalMapValue(combineIndexPath(path, i), elem, options)
		case reflect.Struct:
			check("table")
			e.marshalStructValue(combineIndexPath(path, i), elem, options)
		default:
			panic(&MarshalTypeError{Type: elem.Type(), As: "array element"})
		}
		sep = ", "
	}
	e.WriteString(" ]")
	return "array"
}

func (e *encodeState) marshalArrayField(t *table, key string, v reflect.Value, options tagOptions) {
	if v.Len() != 0 {
		ti, elem := indirectPtr(v.Index(0))
		if ti == nil && isTableType(elem.Type()) && !options.Has("inline") {
			t.appendStructField(key, v)
			return
		}
	}
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalArrayValue(combineKeyPath(t.Path, key), v, options)
}

func (e *encodeState) marshalTableField(t *table, key string, v reflect.Value, options tagOptions) {
	ti, v := indirectPtr(v)

	switch {
	case v.Type() == datetimeType,
		v.Type().ConvertibleTo(datetimeType) && options.Has("datetime"):
		e.marshalDatetimeField(t, key, v, options)
		return
	case ti != nil:
		e.marshalTextField(t, key, ti, options)
		return
	}

	if options.Has("omitempty") && isEmptyValue(v) {
		return
	}

	switch v.Kind() {
	case reflect.Bool:
		e.marshalBoolField(t, key, v.Bool(), options)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.marshalIntField(t, key, v.Int(), options)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.marshalUintField(t, key, v.Uint(), options)
	case reflect.Float32, reflect.Float64:
		e.marshalFloatField(t, key, v.Float(), options)
	case reflect.String:
		e.marshalStringField(t, key, v.String(), options)
	case reflect.Array, reflect.Slice:
		e.marshalArrayField(t, key, v, options)
	case reflect.Map:
		if t.Inline || options.Has("inline") {
			e.marshalMapField(t, key, v)
		} else {
			t.appendStructField(key, v)
		}
	case reflect.Struct:
		if t.Inline || options.Has("inline") {
			e.marshalStructField(t, key, v)
		} else {
			t.appendStructField(key, v)
		}
	case reflect.Ptr, reflect.Interface:
		// nil pointer/interface are ignored.
	default:
		panic(&MarshalTypeError{Type: v.Type(), As: "value"})
	}
}

func (e *encodeState) marshalMapValue(path string, v reflect.Value, options tagOptions) {
	if v.Type().Key().Kind() != reflect.String {
		panic(&MarshalTypeError{Type: v.Type(), As: "table key"})
	}
	e.WriteByte('{')
	var keys stringValues = v.MapKeys()
	t := &table{Inline: true, sep: " "}
	for _, k := range keys {
		e.marshalTableField(t, k.String(), v.MapIndex(k), nil)
	}
	e.WriteByte('}')
}

func (e *encodeState) marshalMapField(t *table, key string, v reflect.Value) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalMapValue(combineKeyPath(t.Path, key), v, nil)
}

func (e *encodeState) marshalStructValue(path string, v reflect.Value, options tagOptions) {
	t := &table{Inline: true, Path: path, sep: " ", keys: make(map[string]struct{})}
	e.marshalStructTable(t, v)
}

func (e *encodeState) marshalStructField(t *table, key string, v reflect.Value) {
	t.recordKey(key)
	e.WriteSepKeyAssign(t.fieldSep(), key)
	e.marshalStructValue(combineKeyPath(t.Path, key), v, nil)
}

func (e *encodeState) marshalStructTable(t *table, v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		sf := v.Type().Field(i)
		name, options := parseTag(sf.Tag.Get("toml"))
		if name == "-" {
			continue
		}
		if sf.Anonymous && name == "" {
			fieldValue := v.Field(i)
			switch sf.Type.Kind() {
			case reflect.Struct:
				e.marshalStructTable(t, fieldValue)
				continue
			case reflect.Ptr:
				if sf.Type.Elem().Kind() != reflect.Struct {
					break
				}
				if !fieldValue.IsNil() {
					e.marshalStructTable(t, fieldValue.Elem())
				}
				continue
			}
		}
		if !ast.IsExported(sf.Name) {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		e.marshalTableField(t, name, v.Field(i), options)
	}
}

func (e *encodeState) marshalTables(sup *table, tables []field) {
	for _, f := range tables {
		v := f.value
		path := combineKeyPath(sup.Path, f.key)
		switch v.Type().Kind() {
		case reflect.Map:
			e.WriteString(fmt.Sprintf("%s[%s]", sup.tableSep(), path))
			e.marshalMap(path, v)
		case reflect.Struct:
			e.WriteString(fmt.Sprintf("%s[%s]", sup.tableSep(), path))
			e.marshalStruct(path, v)
		case reflect.Array, reflect.Slice:
			for i, n := 0, v.Len(); i < n; i++ {
				e.WriteString(fmt.Sprintf("%s[[%s]]", sup.tableSep(), path))
				ti, elem := indirectPtr(v.Index(i))
				if ti != nil {
					panic(&MarshalTypeError{Type: elem.Type(), As: "table"})
				}
				switch elem.Type().Kind() {
				case reflect.Map:
					e.marshalMap(path, elem)
				case reflect.Struct:
					e.marshalStruct(path, elem)
				default:
					panic(&MarshalTypeError{Type: elem.Type(), As: "table"})
				}
			}
		default:
			panic("toml: unexpected postponed field")
		}
	}
}

func (e *encodeState) marshalMap(path string, v reflect.Value) {
	if v.Type().Key().Kind() != reflect.String {
		panic(&MarshalTypeError{Type: v.Type(), As: "table key"})
	}
	t := &table{Path: path, sep: "\n"}
	if path == "" {
		t.sep = ""
	}
	var keys stringValues = v.MapKeys()
	for _, k := range keys {
		e.marshalTableField(t, k.String(), v.MapIndex(k), nil)
	}
	e.marshalTables(t, t.tables)
}

func (e *encodeState) marshalStruct(path string, v reflect.Value) {
	t := &table{Path: path, sep: "\n", keys: make(map[string]struct{})}
	if path == "" {
		t.sep = ""
	}
	e.marshalStructTable(t, v)
	e.marshalTables(t, t.tables)
}

func validMarshal(v interface{}) (reflect.Value, error) {
	ti, rv := indirectPtr(reflect.ValueOf(v))
	if ti != nil {
		return reflect.Value{}, &MarshalTypeError{Type: reflect.TypeOf(v), As: "table"}
	}
	switch rv.Kind() {
	case reflect.Struct, reflect.Map:
	case reflect.Ptr, reflect.Interface:
		return reflect.Value{}, &InvalidMarshalError{reflect.TypeOf(v)}
	default:
		return reflect.Value{}, &MarshalTypeError{Type: reflect.TypeOf(v), As: "table"}
	}
	return rv, nil
}

// Marshal returns TOML encoding of v.
//
// Argument v must be of type struct/map or pointer to these types
// and must not implement encoding.TextMarshaler.
//
// Values implementing encoding.TextMarshaler are encoded as strings.
//
// Fields with nil pointer/interface value in struct or map are ignored.
// Error is raised when nil pointer/interface is encountered in array or
// slice.
//
// Slice of byte is encoded as base64-encoded string.
//
// time.Time and types with "datetime" tagged and convertible to
// time.Time are encoded as TOML Datetime.
//
// Any value that will be encoded as string can have "literal",
// "multiline" and/or "ascii" tagged.
//
// Struct or map fields tagged with "inline" are encoded as inline table.
//
// Tag options specified for array or slice fields are inherited by their
// elements.
func Marshal(v interface{}) (b []byte, err error) {
	rv, err := validMarshal(v)
	if err != nil {
		return nil, err
	}

	defer catchError(&err)

	var e encodeState
	switch rv.Kind() {
	case reflect.Map:
		e.marshalMap("", rv)
	case reflect.Struct:
		e.marshalStruct("", rv)
	}
	e.WriteByte('\n')
	return e.Bytes(), nil
}

// Encoder writes TOML document to an output stream.
type Encoder struct {
	w   io.Writer
	err error
}

// NewEncoder creates a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes TOML document of v to the underlying stream.
func (enc *Encoder) Encode(v interface{}) error {
	if enc.err != nil {
		return enc.err
	}

	b, err := Marshal(v)
	if err != nil {
		return err
	}

	_, enc.err = enc.w.Write(b)
	return enc.err
}
