package toml

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"go/ast"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kezhuw/toml/internal/types"
)

// An InvalidUnmarshalError describes that an invalid argment was passed
// to Unmarshal. The argument passed to Unmarshal must be non-nil pointer.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "toml: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Ptr {
		return "toml: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "toml: Unmarshal(nil " + e.Type.String() + ")"
}

// An UnmarshalTypeError describes that a TOML value is not appropriate
// to be stored in specified Go type.
type UnmarshalTypeError struct {
	Value string
	Type  reflect.Type
}

func (e *UnmarshalTypeError) Error() string {
	return "toml: cannot unmarshal " + e.Value + " to Go value of type " + e.Type.String()
}

// An UnmarshalOverflowError describes that a TOML number value overflows
// specified Go type.
type UnmarshalOverflowError struct {
	Value string
	Type  reflect.Type
}

func (e *UnmarshalOverflowError) Error() string {
	return "toml: " + e.Value + " overflow Go value of type " + e.Type.String()
}

func indirectType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func indirectValue(v reflect.Value) (encoding.TextUnmarshaler, reflect.Value) {
	if v.Kind() != reflect.Ptr && v.CanAddr() {
		v = v.Addr()
	}
	var u encoding.TextUnmarshaler
	for {
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Ptr && !e.IsNil() {
				v = e
				continue
			}
		}
		if v.Kind() != reflect.Ptr {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.NumMethod() > 0 {
			// TOML has native Datetime support, while time.Time implements
			// encoding.TextUnmarshaler. For native Datetime, we need settable
			// time.Time struct, so continue here.
			if i, ok := v.Interface().(encoding.TextUnmarshaler); ok {
				u = i
			}
		}
		v = v.Elem()
	}
	return u, v
}

func findField(t *types.Table, field *reflect.StructField, tagname string) (string, types.Value) {
	if tagname != "" {
		return tagname, t.Elems[tagname]
	}
	if value, ok := t.Elems[field.Name]; ok {
		return field.Name, value
	}
	lowerName := strings.ToLower(field.Name)
	return lowerName, t.Elems[lowerName]
}

func unmarshalBoolean(b bool, v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(b)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(b))
			return
		}
		fallthrough
	default:
		panic(&UnmarshalTypeError{"boolean " + strconv.FormatBool(b), v.Type()})
	}
}

func unmarshalQuoted(s string, v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			panic(err)
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			panic(err)
		}
		if v.OverflowInt(i) {
			goto overflowError
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			panic(err)
		}
		if v.OverflowUint(u) {
			goto overflowError
		}
		v.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic(err)
		}
		if v.OverflowFloat(f) {
			goto overflowError
		}
		v.SetFloat(f)
	default:
		panic("toml: unexpected type for quoted string")
	}
	return
overflowError:
	panic(&UnmarshalOverflowError{"string " + strconv.Quote(s), v.Type()})
}

func unmarshalString(s string, v reflect.Value, options tagOptions) {
	u, v := indirectValue(v)
	if u != nil {
		err := u.UnmarshalText([]byte(s))
		if err != nil {
			panic(err)
		}
		return
	}
	switch v.Kind() {
	case reflect.String:
		if options.Has("string") {
			t, err := strconv.Unquote(s)
			if err != nil {
				panic(err)
			}
			v.SetString(t)
			break
		}
		v.SetString(s)
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			goto typeError
		}
		b := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
		n, err := base64.StdEncoding.Decode(b, []byte(s))
		if err != nil {
			panic(err)
		}
		v.SetBytes(b[:n])
	case reflect.Bool,
		reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !options.Has("string") {
			goto typeError
		}
		unmarshalQuoted(s, v)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			goto typeError
		}
		v.Set(reflect.ValueOf(s))
	default:
		goto typeError
	}
	return
typeError:
	panic(&UnmarshalTypeError{fmt.Sprintf("string: %q", s), v.Type()})
}

func unmarshalDatetime(t time.Time, v reflect.Value) {
	if !reflect.TypeOf(t).ConvertibleTo(v.Type()) {
		panic(&UnmarshalTypeError{"datetime " + t.Format(time.RFC3339Nano), v.Type()})
	}
	v.Set(reflect.ValueOf(t).Convert(v.Type()))
}

func unmarshalFloat(f float64, v reflect.Value) {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if v.OverflowFloat(f) {
			panic(&UnmarshalOverflowError{"float " + strconv.FormatFloat(f, 'g', -1, 64), v.Type()})
		}
		v.SetFloat(f)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(f))
			return
		}
		fallthrough
	default:
		panic(&UnmarshalTypeError{"float " + strconv.FormatFloat(f, 'g', -1, 64), v.Type()})
	}
}

func unmarshalInteger(i int64, v reflect.Value) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.OverflowInt(i) {
			panic(&UnmarshalOverflowError{"integer " + strconv.FormatInt(i, 10), v.Type()})
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if i < 0 {
			panic(&UnmarshalOverflowError{"integer " + strconv.FormatInt(i, 10), v.Type()})
		}
		u := uint64(i)
		if v.OverflowUint(u) {
			panic(&UnmarshalOverflowError{"integer " + strconv.FormatUint(u, 10), v.Type()})
		}
		v.SetUint(u)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(i))
			return
		}
		fallthrough
	default:
		panic(&UnmarshalTypeError{"integer " + strconv.FormatInt(i, 10), v.Type()})
	}
}

var emptyInterfaceType = reflect.TypeOf((*interface{})(nil)).Elem()

func unmarshalMap(t *types.Table, v reflect.Value) {
	keyType := v.Type().Key()
	if keyType.Kind() != reflect.String {
		panic(&UnmarshalTypeError{"table", v.Type()})
	}
	m := reflect.MakeMap(v.Type())
	elemType := v.Type().Elem()
	elemZero := reflect.Zero(elemType)
	elemValue := reflect.New(elemType).Elem()
	for key, value := range t.Elems {
		elemValue.Set(elemZero)
		unmarshalValue(value, elemValue, nil)
		m.SetMapIndex(reflect.ValueOf(key).Convert(keyType), elemValue)
	}
	v.Set(m)
}

func unmarshalStructNested(t *types.Table, v reflect.Value, matchs map[string]struct{}) {
	_, v = indirectValue(v)
	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := vType.Field(i)
		// Unexported embedded struct fields may export internal
		// export fields to its parent.
		//
		// There is a bug which cause unexported embedded struct fields
		// settable in Go 1.5 and prior. See https://github.com/golang/go/issues/12367.
		// Here we use ast.IsExported to circumvent it.
		isExported := ast.IsExported(field.Name)
		if !isExported && !field.Anonymous {
			continue
		}
		var (
			name    string
			value   types.Value
			options tagOptions
		)
		name, options = parseTag(field.Tag.Get("toml"))
		if name == "-" {
			continue
		}
		if isExported {
			name, value = findField(t, &field, name)
		}
		if value == nil {
			if isExported && options.Has("omitempty") {
				v.Field(i).Set(reflect.Zero(field.Type))
			} else if field.Anonymous {
				fieldValue := v.Field(i)
				switch field.Type.Kind() {
				case reflect.Struct:
					unmarshalStructNested(t, v.Field(i), matchs)
				case reflect.Ptr:
					if field.Type.Elem().Kind() != reflect.Struct {
						break
					}
					if fieldValue.IsNil() {
						fieldNew := reflect.New(field.Type.Elem())
						n := len(matchs)
						unmarshalStructNested(t, fieldNew.Elem(), matchs)
						if n != len(matchs) {
							fieldValue.Set(fieldNew)
						}
					} else {
						unmarshalStructNested(t, fieldValue, matchs)
					}
				default:
				}
			}
			continue
		}
		if _, ok := matchs[name]; ok {
			continue
		}
		unmarshalValue(value, v.Field(i), options)
		matchs[name] = struct{}{}
	}
}

func unmarshalStruct(t *types.Table, v reflect.Value) {
	unmarshalStructNested(t, v, make(map[string]struct{}, len(t.Elems)))
}

func unmarshalTable(t *types.Table, v reflect.Value) {
	switch v.Kind() {
	case reflect.Map:
		unmarshalMap(t, v)
	case reflect.Struct:
		unmarshalStruct(t, v)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(t.Interface()))
			return
		}
		fallthrough
	default:
		panic(&UnmarshalTypeError{"table", v.Type()})
	}
}

func unmarshalSlice(a *types.Array, v reflect.Value) {
	n := len(a.Elems)
	slice := reflect.MakeSlice(v.Type(), n, n)
	for i, value := range a.Elems {
		unmarshalValue(value, slice.Index(i), nil)
	}
	v.Set(slice)
}

func unmarshalGoArray(a *types.Array, v reflect.Value) {
	if len(a.Elems) != v.Type().Len() {
		panic(&UnmarshalTypeError{fmt.Sprintf("[%d]array", len(a.Elems)), v.Type()})
	}
	if v.IsNil() {
		v.Set(reflect.Zero(v.Type()))
	}
	for i, value := range a.Elems {
		unmarshalValue(value, v.Index(i), nil)
	}
}

func unmarshalArray(a *types.Array, v reflect.Value) {
	switch v.Kind() {
	case reflect.Array:
		unmarshalGoArray(a, v)
	case reflect.Slice:
		unmarshalSlice(a, v)
	case reflect.Interface:
		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(a.Interface()))
			return
		}
		fallthrough
	default:
		panic(&UnmarshalTypeError{"array", v.Type()})
	}
}

func unmarshalValue(tv types.Value, rv reflect.Value, options tagOptions) {
	_, rv = indirectValue(rv)
	switch tv := tv.(type) {
	case types.Boolean:
		unmarshalBoolean(bool(tv), rv)
	case types.Float:
		unmarshalFloat(float64(tv), rv)
	case types.String:
		unmarshalString(string(tv), rv, options)
	case types.Integer:
		unmarshalInteger(int64(tv), rv)
	case types.Datetime:
		unmarshalDatetime(time.Time(tv), rv)
	case *types.Array:
		unmarshalArray(tv, rv)
	case *types.Table:
		unmarshalTable(tv, rv)
	}
}

func catchError(errp *error) {
	if r := recover(); r != nil {
		switch err := r.(type) {
		default:
			panic(r)
		case runtime.Error:
			panic(r)
		case error:
			*errp = err
		}
	}
}

// Unmarshal parses TOML data and stores the result in the value pointed
// by v.
//
// To unmarshal TOML into a struct, Unmarshal uses TOML tagged name to
// find matching item in TOML table. Field name and its lower case will
// got tried in sequence if TOML tagged name is absent. Options can be
// specified after tag name separated by comma. Examples:
//
//   // Field is ignored by this package.
//   Field int `toml:"-"`
//
//   // "Field" and "field" will be used to find key in TOML table.
//   Field int `toml:"myName"`
//
//   // "myName" will be used to find matching item in TOML table, and
//   // if it is absent, it will be set to zero value.
//   Field int `toml:"myName,omitempty"`
//
//   // "Field" and "field" will be used to find key in TOML table and
//   // this field can be unmarshalled from TOML string.
//   Field int `toml:",string"
//
// To unmarshal TOML into an interface value, Unmarshal stores TOML
// value in following types:
//
//   bool, for TOML Boolean
//   int64, for TOML Integer
//   float64, for TOML Float
//   string, for TOML String
//   time.Time, for TOML Datetime
//   []interface{}, for TOML Array
//   map[string]interface{}, for TOML Table
//
// There is no guarantee that origin data in Go value will be preserved
// after a failure or success Unmarshal().
func Unmarshal(data []byte, v interface{}) (err error) {
	defer catchError(&err)

	t, err := parse(data)
	if err != nil {
		return err
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	_, rv = indirectValue(rv)
	unmarshalTable(t, rv)
	return nil
}
