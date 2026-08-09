package fetch

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeSeroval(body []byte) (any, error) {
	s := string(body)
	sep := strings.Index(s, "],")
	if sep < 0 {
		return nil, fmt.Errorf("unexpected stream shape")
	}
	expr := stripChunkHeaders(s[sep+2:])
	expr = strings.TrimSpace(expr)
	expr = strings.TrimRight(expr, ")")
	expr = strings.TrimSpace(expr)
	if i := strings.Index(expr, ")($R["); i >= 0 {
		prefix := "($R=>$R[0]="
		if strings.HasPrefix(expr, prefix) {
			expr = expr[len(prefix):i]
		}
	}
	expr = strings.TrimSpace(expr)
	switch expr {
	case "void 0", "null", "":
		return nil, nil
	}
	if expr[0] == '"' {
		var s string
		if err := json.Unmarshal([]byte(expr), &s); err != nil {
			return nil, fmt.Errorf("invalid string value: %w", err)
		}
		return s, nil
	}
	if expr[0] != '{' && expr[0] != '[' {
		return nil, fmt.Errorf("unexpected value prefix %q", expr[:min(len(expr), 20)])
	}
	jsonStr, err := normalizeJS(expr)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return nil, fmt.Errorf("invalid object value: %w", err)
	}
	return v, nil
}

func stripChunkHeaders(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, ";0x")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		if len(s) < i+12 {
			return b.String()
		}
		s = s[i+12:]
	}
}

func normalizeJS(s string) (string, error) {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
			b.WriteByte(ch)
		case '!':
			if i+1 < len(s) && s[i+1] == '0' {
				b.WriteString("true")
				i++
			} else if i+1 < len(s) && s[i+1] == '1' {
				b.WriteString("false")
				i++
			} else {
				b.WriteByte(ch)
			}
		case '$':
			if rest := s[i:]; strings.HasPrefix(rest, "$R[") {
				j := strings.Index(rest, "]=")
				if j > 0 {
					i += j + 1
					continue
				}
			}
			b.WriteByte(ch)
		case ':', '{', ',', '[':
			if ch == ':' {
				if err := quoteKeyBefore(&b); err != nil {
					return "", err
				}
			}
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated string in stream")
	}
	return b.String(), nil
}

func quoteKeyBefore(b *strings.Builder) error {
	out := b.String()
	if out == "" {
		return nil
	}
	start := len(out) - 1
	for start >= 0 && isKeyChar(out[start]) {
		start--
	}
	if start == len(out)-1 {
		return nil
	}
	key := out[start+1:]
	rest := out[:start+1]
	b.Reset()
	b.WriteString(rest)
	b.WriteByte('"')
	b.WriteString(key)
	b.WriteByte('"')
	return nil
}

func isKeyChar(ch byte) bool {
	return ch == '_' || ch == '$' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}
