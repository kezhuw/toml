package types

import (
	"time"
)

type Value interface {
	Type() string
	TOMLValue()
}

type Environment interface {
	Value
	TomlEnvironment()
}

type Array struct {
	Closed bool
	Elems  []Value
}

type Table struct {
	Implicit bool
	Elems    map[string]Value
}

type String string

type Integer int64

type Float float64

type Boolean bool

type Datetime time.Time

func (t *Table) Type() string   { return "table" }
func (a *Array) Type() string   { return "array" }
func (s String) Type() string   { return "string" }
func (i Integer) Type() string  { return "integer" }
func (f Float) Type() string    { return "float" }
func (b Boolean) Type() string  { return "boolean" }
func (d Datetime) Type() string { return "datetime" }

func (a *Array) TOMLValue()   {}
func (t *Table) TOMLValue()   {}
func (s String) TOMLValue()   {}
func (i Integer) TOMLValue()  {}
func (f Float) TOMLValue()    {}
func (b Boolean) TOMLValue()  {}
func (d Datetime) TOMLValue() {}

func (a *Array) TomlEnvironment() {}
func (t *Table) TomlEnvironment() {}
