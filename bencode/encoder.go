package bencode

import (
	"bytes"
	"fmt"
	"sort"
)

func Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encode(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case int:
		fmt.Fprintf(buf, "i%de", val)
	case int64:
		fmt.Fprintf(buf, "i%de", val)
	case string:
		fmt.Fprintf(buf, "%d:", len(val))
		buf.WriteString(val)
	case []byte:
		fmt.Fprintf(buf, "%d:", len(val))
		buf.Write(val)
	case []any:
		buf.WriteByte('l')
		for _, item := range val {
			if err := encode(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	case map[string]any:
		buf.WriteByte('d')
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(buf, "%d:%s", len(k), k)
			if err := encode(buf, val[k]); err != nil {
				return fmt.Errorf("encoding value for key %q: %w", k, err)
			}
		}
		buf.WriteByte('e')
	default:
		return fmt.Errorf("unsupported type %T — use int64, string, []byte, []any, or map[string]any", v)
	}
	return nil
}
