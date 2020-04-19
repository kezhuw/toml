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

type Integer16 uint16

type Struct struct {
	Integer16
	STRING  string `toml:"sTrInG"`
	Pointer *string
	Nested  Embeda
}

type Unicode struct {
	Welcome string `toml:"初次见面"`
}

type Ignore struct {
	Key string `toml:"-"`
}

type IgnoreEmbed struct {
	embed0 `toml:"-"`
	Embeda
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
