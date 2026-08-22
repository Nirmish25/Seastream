package bencode

import (
	"fmt"
	"strconv"
)

func Decode(data []byte) (any, error) {
	val, consumed, err := decode(data, 0)
	if err != nil {
		return nil, err
	}
	if consumed != len(data) {
		return nil, fmt.Errorf("trailing bytes after position %d", consumed)
	}
	return val, nil
}

func decode(data []byte, pos int) (any, int, error) {
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unexpected end of data at position %d", pos)
	}

	switch {
	case data[pos] == 'i':
		return decodeInt(data, pos)
	case data[pos] == 'l':
		return decodeList(data, pos)
	case data[pos] == 'd':
		return decodeDict(data, pos)
	case data[pos] >= '0' && data[pos] <= '9':
		return decodeString(data, pos)
	default:
		return nil, pos, fmt.Errorf("unknown type marker %q at position %d", data[pos], pos)
	}
}

func decodeInt(data []byte, pos int) (int64, int, error) {
	pos++
	end := pos
	for end < len(data) && data[end] != 'e' {
		end++
	}
	if end >= len(data) {
		return 0, pos, fmt.Errorf("unterminated integer starting at position %d", pos-1)
	}
	n, err := strconv.ParseInt(string(data[pos:end]), 10, 64)
	if err != nil {
		return 0, pos, fmt.Errorf("invalid integer %q: %w", data[pos:end], err)
	}
	return n, end + 1, nil
}

func decodeString(data []byte, pos int) (string, int, error) {
	colon := pos
	for colon < len(data) && data[colon] != ':' {
		colon++
	}
	if colon >= len(data) {
		return "", pos, fmt.Errorf("missing ':' in string starting at position %d", pos)
	}
	length, err := strconv.Atoi(string(data[pos:colon]))
	if err != nil {
		return "", pos, fmt.Errorf("invalid string length %q: %w", data[pos:colon], err)
	}
	if length < 0 {
		return "", pos, fmt.Errorf("negative string length %d", length)
	}
	start := colon + 1
	end := start + length
	if end > len(data) {
		return "", pos, fmt.Errorf("string length %d overruns data (only %d bytes left)", length, len(data)-start)
	}
	return string(data[start:end]), end, nil
}

func decodeList(data []byte, pos int) ([]any, int, error) {
	pos++
	var list []any
	for pos < len(data) && data[pos] != 'e' {
		val, next, err := decode(data, pos)
		if err != nil {
			return nil, pos, err
		}
		list = append(list, val)
		pos = next
	}
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unterminated list")
	}
	return list, pos + 1, nil
}

func ValueRange(data []byte, key string) (start, end int, err error) {
	if len(data) == 0 || data[0] != 'd' {
		return 0, 0, fmt.Errorf("data is not a bencoded dict")
	}
	pos := 1
	for pos < len(data) && data[pos] != 'e' {
		k, next, err := decodeString(data, pos)
		if err != nil {
			return 0, 0, fmt.Errorf("reading dict key: %w", err)
		}
		pos = next
		valStart := pos
		_, next, err = decode(data, pos)
		if err != nil {
			return 0, 0, fmt.Errorf("reading value for key %q: %w", k, err)
		}
		if k == key {
			return valStart, next, nil
		}
		pos = next
	}
	return 0, 0, fmt.Errorf("key %q not found", key)
}

func decodeDict(data []byte, pos int) (map[string]any, int, error) {
	pos++
	dict := make(map[string]any)
	for pos < len(data) && data[pos] != 'e' {
		key, next, err := decodeString(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict key at position %d: %w", pos, err)
		}
		pos = next
		val, next, err := decode(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict value for key %q: %w", key, err)
		}
		dict[key] = val
		pos = next
	}
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unterminated dict")
	}
	return dict, pos + 1, nil
}
