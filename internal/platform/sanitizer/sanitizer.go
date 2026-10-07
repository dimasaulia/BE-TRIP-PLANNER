package sanitizer

import (
	"fmt"
	"strings"
)

var sensitiveKeys = map[string]struct{}{
	"authorization":  {},
	"password":       {},
	"token":          {},
	"access_token":   {},
	"refresh_token":  {},
	"secret":         {},
	"client_secret":  {},
	"api_key":        {},
	"otp":            {},
	"mfa_code":       {},
	"private_key":    {},
	"credential":     {},
	"credentials":    {},
	"cookie":         {},
	"set_cookie":     {},
	"salary":         {},
	"pin":            {},
	"passcode":       {},
	"security_code":  {},
	"recovery_code":  {},
	"recovery_codes": {},
}

func Attrs(attrs []any) []any {
	if len(attrs) == 0 {
		return attrs
	}

	sanitized := make([]any, len(attrs))
	copy(sanitized, attrs)

	for i := 0; i < len(sanitized)-1; i += 2 {
		key, ok := sanitized[i].(string)
		if !ok {
			continue
		}

		sanitized[i+1] = Value(key, sanitized[i+1])
	}

	return sanitized
}

func Value(key string, value any) any {
	normalized := normalizeKey(key)
	if _, exists := sensitiveKeys[normalized]; exists {
		return "[MASKED]"
	}

	switch typed := value.(type) {
	case map[string]any:
		return Map(typed)
	case map[string]string:
		result := make(map[string]string, len(typed))
		for k, v := range typed {
			if _, exists := sensitiveKeys[normalizeKey(k)]; exists {
				result[k] = "[MASKED]"
				continue
			}
			result[k] = v
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = Value("", item)
		}
		return result
	default:
		return value
	}
}

func Map(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = Value(key, value)
	}
	return result
}

func Error(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, ".", "_")
	return key
}
