package toml

import (
	"fmt"
	"strconv"
)

func normalizeKey(key string) string {
	for _, r := range key {
		if !isBareKeyChar(r) {
			return strconv.Quote(key)
		}
	}
	return key
}

func combineKeyPath(path, key string) string {
	key = normalizeKey(key)
	if path == "" {
		return key
	}
	return path + "." + key
}

func combineIndexPath(path string, i int) string {
	return fmt.Sprintf("%s[%d]", path, i)
}
