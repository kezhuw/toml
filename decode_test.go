package toml_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kezhuw/toml"
)

type embed0 struct {
	E0 int `toml:"embed0"`
}

type Embed1 struct {
	embed0
}

type Embeda struct {
	Ea string
}

type Embedb struct {
	*Embeda
}

type Struct struct {
	STRING  string `toml:"sTrInG"`
	Integer uint16
	Pointer *string
	Nested  Embeda
}

type Unicode struct {
	Welcome string `toml:"初次见面"`
}

type Ignore struct {
	Key string `toml:"-"`
}

type String struct {
	Integer int64 `toml:",string"`
}

type Overflow struct {
	Uint8   uint8
	Float32 float32
}

type Datetime struct {
	T time.Time
}

type Types struct {
	Int    int
	Float  float64
	String string
}

type Omitempty struct {
	S string `toml:",omitempty"`
}

var nonempty = Omitempty{S: "nonempty"}

type testData struct {
	in  string
	ptr interface{}
	out interface{}
	err error
}

var s = string("string")

var unmarshalTests = []testData{
	{`embed0 = 3_456`, new(Embed1), Embed1{embed0{E0: 3456}}, nil},
	{`ea = """ea"""`, new(Embedb), Embedb{&Embeda{"ea"}}, nil},
	{"sTrInG = 'ip4'\n integer = 1234", new(Struct), Struct{STRING: "ip4", Integer: 1234}, nil},
	{"sTrInG = '''ip6'''\ninteger = 1234", new(Struct), Struct{STRING: "ip6", Integer: 1234}, nil},
	{`pointer = "string"`, new(Struct), Struct{Pointer: &s}, nil},
	{`"初次见面" = "你好，世界！"`, new(Unicode), Unicode{"你好，世界！"}, nil},
	{`"初次\u89c1\U00009762" = "你好，\u4e16\U0000754c！"`, new(Unicode), Unicode{"你好，世界！"}, nil},
	{`t = 2016-01-07T15:30:30Z`, new(Datetime), Datetime{time.Date(2016, 1, 7, 15, 30, 30, 0, time.UTC)}, nil},
	{`t = "2016-01-07T15:30:30Z"`, new(Datetime), Datetime{time.Date(2016, 1, 7, 15, 30, 30, 0, time.UTC)}, nil},
	{`Key = "ignored"`, new(Ignore), Ignore{}, nil},
	{`integer = "123456"`, new(String), String{123456}, nil},
	{``, &nonempty, Omitempty{}, nil},
	{
		in:  `uint8 = 257`,
		ptr: new(Overflow),
		err: &toml.UnmarshalOverflowError{"integer 257", reflect.TypeOf(uint8(0))},
	},
	{
		in:  `uint8 = -1`,
		ptr: new(Overflow),
		err: &toml.UnmarshalOverflowError{"integer -1", reflect.TypeOf(uint8(0))},
	},
	{
		in:  `float32 = 3.4e+49`,
		ptr: new(Overflow),
		err: &toml.UnmarshalOverflowError{"float 3.4e+49", reflect.TypeOf(float32(0))},
	},
	{
		in:  `int = "string"`,
		ptr: new(Types),
		err: &toml.UnmarshalTypeError{`string: "string"`, reflect.TypeOf(int(0))},
	},
	{
		in:  `string = 233`,
		ptr: new(Types),
		err: &toml.UnmarshalTypeError{`integer 233`, reflect.TypeOf(string(""))},
	},
	{
		in:  `int = 233.5`,
		ptr: new(Types),
		err: &toml.UnmarshalTypeError{`float 233.5`, reflect.TypeOf(int(0))},
	},
	{
		in: `
		[Nested]
		Ea = '''
Ea \
value'''
		`,
		ptr: new(Struct),
		out: Struct{Nested: Embeda{Ea: "Ea \\\nvalue"}},
	},
	{
		in: `Nested = { Ea = """E\
		a value""" }`,
		ptr: new(Struct),
		out: Struct{Nested: Embeda{Ea: "Ea value"}},
	},
	{
		in: `
		integers = [ 1, 2, 3, 4,]
		[[tables]]
		description = "I am a TOML table"
		[[tables]]
		name = "Another TOML table"
		`,
		ptr: new(interface{}),
		out: map[string]interface{}{
			"integers": []interface{}{int64(1), int64(2), int64(3), int64(4)},
			"tables": []interface{}{
				map[string]interface{}{"description": "I am a TOML table"},
				map[string]interface{}{"name": "Another TOML table"},
			},
		},
	},
	{
		in:  `[[table.array]]`,
		ptr: new(interface{}),
		out: map[string]interface{}{
			"table": map[string]interface{}{
				"array": []interface{}{
					map[string]interface{}{},
				},
			},
		},
	},
	{
		in: `
		points = [ { x = 1, y = 2, z = 3 },
		           { x = 7, y = 8, z = 9 },
			   { x = 2, y = 4, z = 8 } ]`,
		ptr: new(interface{}),
		out: map[string]interface{}{
			"points": []interface{}{
				map[string]interface{}{
					"x": int64(1), "y": int64(2), "z": int64(3),
				},
				map[string]interface{}{
					"x": int64(7), "y": int64(8), "z": int64(9),
				},
				map[string]interface{}{
					"x": int64(2), "y": int64(4), "z": int64(8),
				},
			},
		},
	},
	{
		in: `
		[[fruit]]
		  name = "apple"

		  [fruit.physical]
		    color = "red"
		    shape = "round"

		  [[fruit.variety]]
		    name = "red delicious"

		  [[fruit.variety]]
		    name = "granny smith"

		[[fruit]]
		  name = "banana"

		  [[fruit.variety]]
		    name = "plantain"
		`,
		ptr: new(interface{}),
		out: map[string]interface{}{
			"fruit": []interface{}{
				map[string]interface{}{
					"name": "apple",
					"physical": map[string]interface{}{
						"color": "red",
						"shape": "round",
					},
					"variety": []interface{}{
						map[string]interface{}{"name": "red delicious"},
						map[string]interface{}{"name": "granny smith"},
					},
				},
				map[string]interface{}{
					"name": "banana",
					"variety": []interface{}{
						map[string]interface{}{"name": "plantain"},
					},
				},
			},
		},
	},
	{
		in: `
		[[products]]
		name = "Hammer"
		sku = 738594937

		[[products]]

		[[products]]
		name = "Nail"
		sku = 284758393
		color = "gray"
		`,
		ptr: new(interface{}),
		out: map[string]interface{}{
			"products": []interface{}{
				map[string]interface{}{
					"name": "Hammer",
					"sku":  int64(738594937),
				},
				map[string]interface{}{},
				map[string]interface{}{
					"name":  "Nail",
					"sku":   int64(284758393),
					"color": "gray",
				},
			},
		},
	},
}

func TestUnmarshal(t *testing.T) {
	for i, test := range unmarshalTests {
		err := toml.Unmarshal([]byte(test.in), test.ptr)

		if test.err != nil {
			if !reflect.DeepEqual(test.err, err) {
				t.Errorf("#%d: error got %s\n, want %s", i, err, test.err)
			}
			continue
		}

		if err != nil {
			t.Errorf("#%d: got error: %s", i, err)
			continue
		}

		got := reflect.ValueOf(test.ptr).Elem().Interface()
		if !reflect.DeepEqual(got, test.out) {
			t.Errorf("#%d: got %+v\n, want %+v", i, got, test.out)
			continue
		}
	}
}
