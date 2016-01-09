# TOML parser for Go

Compatible with [TOML][] version [v0.4.0](https://github.com/toml-lang/toml/blob/master/versions/en/toml-v0.4.0.md).

[![GoDoc](https://godoc.org/github.com/kezhuw/toml?status.svg)](http://godoc.org/github.com/kezhuw/toml)
[![Build Status](https://travis-ci.org/kezhuw/toml.svg?branch=master)](https://travis-ci.org/kezhuw/toml)

Run `go get github.com/kezhuw/toml` to install.

## Examples
Snippets copied from Go test files.

```go
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
```

```go
package toml_test

import (
	"fmt"
	"time"

	"github.com/kezhuw/toml"
)

type duration time.Duration

func (d *duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = duration(v)
	return nil
}

func ExampleUnmarshal_textUnmarshaler() {
	data := []byte(`timeout = "300ms"`)
	var out struct{ Timeout duration }

	err := toml.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(time.Duration(out.Timeout))
	// Output: 300ms
}
```

## Links
Other TOML libraries written in Go.

- https://github.com/BurntSushi/toml     TOML parser and encoder for Go with reflection
- https://github.com/pelletier/go-toml   Go library for the TOML language
- https://github.com/naoina/toml         TOML parser and encoder library for Golang

## License
Released under The MIT License (MIT). See [LICENSE](LICENSE) for the full license text.

## Contribution
Fire issue or pull request if you have any questions.

## TODO
- Encoder

[TOML]: https://github.com/toml-lang/toml
