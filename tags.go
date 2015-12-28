package toml

import (
	"strings"
)

type tagOptions map[string]struct{}

func (o tagOptions) Has(opt string) bool {
	_, ok := o[opt]
	return ok
}

func parseTag(tag string) (string, tagOptions) {
	splits := strings.Split(tag, ",")
	if len(splits) == 1 {
		return splits[0], nil
	}
	options := make(map[string]struct{}, len(splits)-1)
	for i := 1; i < len(splits); i++ {
		options[splits[i]] = struct{}{}
	}
	return splits[0], options
}
