package toml_test

import (
	"fmt"
	"time"

	"github.com/kezhuw/toml"
)

func ExampleUnmarshal_integer() {
	data := []byte(`key = 12345`)
	var out struct{ Key int }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: 12345
}

func ExampleUnmarshal_float() {
	data := []byte(`key = 3.14`)
	var out struct{ Key float64 }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: 3.14
}

func ExampleUnmarshal_boolean() {
	data := []byte(`key = true`)
	var out struct{ Key bool }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: true
}

func ExampleUnmarshal_string() {
	data := []byte(`key = "value"`)
	var out struct{ Key string }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: value
}

func ExampleUnmarshal_datetimeNative() {
	data := []byte(`key = 2016-01-07T15:30:30.123456789Z`)
	var out struct{ Key time.Time }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key.Format(time.RFC3339Nano))
	// Output: 2016-01-07T15:30:30.123456789Z
}

func ExampleUnmarshal_datetimeTextUnmarshaler() {
	data := []byte(`key = "2016-01-07T15:30:30.123456789Z"`)
	var out struct{ Key time.Time }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key.Format(time.RFC3339Nano))
	// Output: 2016-01-07T15:30:30.123456789Z
}

func ExampleUnmarshal_array() {
	data := []byte(`key = [1, 2, 3,4]`)
	var out struct{ Key []int }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: [1 2 3 4]
}

func ExampleUnmarshal_table() {
	data := []byte(`[key]
	name = "name"
	value = "value"`)
	var out struct {
		Key struct {
			Name  string
			Value string
		}
	}

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Printf("key.name = %q\n", out.Key.Name)
	fmt.Printf("key.value = %q\n", out.Key.Value)
	// Output:
	// key.name = "name"
	// key.value = "value"
}

func ExampleUnmarshal_inlineTable() {
	data := []byte(`key = { name = "name", value = "value" }`)
	var out struct {
		Key struct {
			Name  string
			Value string
		}
	}

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Printf("key.name = %q\n", out.Key.Name)
	fmt.Printf("key.value = %q\n", out.Key.Value)
	// Output:
	// key.name = "name"
	// key.value = "value"
}

func ExampleUnmarshal_tableArray() {
	data := []byte(`
	[[array]]
	description = "Table In Array"
	`)
	var out struct {
		Array []struct{ Description string }
	}

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Array[0].Description)
	// Output: Table In Array
}

func ExampleUnmarshal_interface() {
	data := []byte(`key = [1, 2, 3, 4,]`)
	var out struct{ Key interface{} }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: [1 2 3 4]
}

func ExampleUnmarshal_tagName() {
	data := []byte(`KKKK = "value"`)
	var out struct {
		Key string `toml:"KKKK"`
	}

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: value
}

func ExampleUnmarshal_tagIgnore() {
	data := []byte(`key = "value"`)
	var out struct {
		Key string `toml:"-"`
	}

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output:
}

func ExampleUnmarshal_tagString() {
	data := []byte(`key = "12345"`)
	var out struct {
		Key int `toml:",string"`
	}

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output: 12345
}

func ExampleUnmarshal_tagOmitempty() {
	data := []byte(``)
	var out struct {
		Key string `toml:",omitempty"`
	}
	out.Key = "Not empty, for now."

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Key)
	// Output:
}
