package toml_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kezhuw/toml"
)

type EncodeIgnore struct {
	Export string
}

type EncodeNested struct {
	Nested string
}

type EncodeEmbed struct {
	Uint32 uint32
	*EncodeNested
	EncodeIgnore `toml:"-"`
}

type EncodeTable struct {
	S string `toml:"s"`
}

type EncodeStruct struct {
	Int        int
	Uint       uint `toml:"unsigned,string"`
	Float      float64
	String     string
	Zero       int    `toml:",omitempty"`
	Empty      string `toml:",omitempty"`
	Notempty   string `toml:",omitempty"`
	Date       time.Time
	Ignored    string `toml:"-"`
	unexported string
	*EncodeEmbed
	Strings []string
	Table   EncodeTable
	Tables  []*EncodeTable `toml:",omitempty"`
	Inlines []EncodeTable  `toml:",inline"`
}

type encodeData struct {
	in    interface{}
	out   interface{}
	err   error
	print bool
}

var marshalTests = []encodeData{
	{
		in:  EncodeStruct{},
		out: &EncodeStruct{},
	},
	{
		in: EncodeStruct{
			Strings: []string{"string a", "string b"},
			Table: EncodeTable{
				S: "string in table",
			},
			Inlines: []EncodeTable{
				{S: "inline table 1"},
				{S: "inline table 2"},
			},
			Tables: []*EncodeTable{
				&EncodeTable{S: "pointer type table 1"},
				&EncodeTable{S: "pointer type table 2"},
			},
		},
		out: new(EncodeStruct),
	},
	{
		in: EncodeStruct{
			Int:        -3242,
			Uint:       9999329,
			Float:      3.3e9,
			String:     "toml string",
			Notempty:   "not empty",
			Date:       time.Date(2016, 1, 7, 15, 30, 30, 0, time.UTC),
			Ignored:    "ignore",
			unexported: "unexported",
			EncodeEmbed: &EncodeEmbed{
				Uint32: 99342,
				EncodeIgnore: EncodeIgnore{
					Export: "export field",
				},
				EncodeNested: &EncodeNested{
					Nested: "nested field",
				},
			},
		},
		out: &EncodeStruct{
			Ignored:    "ignore",
			unexported: "unexported",
			EncodeEmbed: &EncodeEmbed{
				EncodeIgnore: EncodeIgnore{
					Export: "export field",
				},
			},
		},
	},
	{
		in: map[string]interface{}{
			"integers": []interface{}{int64(1), int64(2), int64(3), int64(4)},
			"tables": []interface{}{
				map[string]interface{}{"description": "I am a TOML table"},
				map[string]interface{}{"name": "Another TOML table"},
			},
		},
		out: &map[string]interface{}{},
	},
}

func TestMarshal(t *testing.T) {
	for i, test := range marshalTests {
		b, err := toml.Marshal(test.in)

		if test.print && err == nil {
			t.Errorf("\n# %d: marshaled TOML document:\n%s# EOF\n", i, string(b))
		}

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

		err = toml.Unmarshal(b, test.out)
		if err != nil {
			t.Errorf("#%d: unmarshal error: %s\ntext:\n%s\nfrom: %+v", i, err, string(b), test.in)
			continue
		}

		got := reflect.ValueOf(test.out).Elem().Interface()
		if !reflect.DeepEqual(test.in, got) {
			t.Errorf("#%d:\ngot  %+v,\nwant %+v\n,\ntext:\n%s", i, got, test.in, string(b))
			continue
		}
	}
}
