package toml

import (
	"fmt"
)

// ParseError describes errors raised in parsing phase.
type ParseError struct {
	Line int // 1-based
	Pos  int // 0-based, relative to beginning of input
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("toml: line %d, pos %d: %s", e.Line, e.Pos, e.Err.Error())
}
