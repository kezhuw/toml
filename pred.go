package toml

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isHex(r rune) bool {
	switch {
	default:
		return false
	case 'A' <= r && r <= 'Z':
	case 'a' <= r && r <= 'z':
	case '0' <= r && r <= '9':
	}
	return true
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isBareKeyChar(r rune) bool {
	switch {
	default:
		return false
	case 'A' <= r && r <= 'Z':
	case 'a' <= r && r <= 'z':
	case '0' <= r && r <= '9':
	case r == '-' || r == '_':
	}
	return true
}
