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
