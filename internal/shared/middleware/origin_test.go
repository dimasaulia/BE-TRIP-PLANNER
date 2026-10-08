package middleware

import "testing"

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://app.example.com"}

	tests := map[string]bool{
		"":                             true, // not a browser, nothing to forge
		"http://localhost:3000":        true,
		"HTTP://LOCALHOST:3000":        true,
		"https://app.example.com/":     true,
		"https://evil.example":         false,
		"http://app.example.com":       false, // wrong scheme
		"https://app.example.com.evil": false, // prefix is not a match
		"null":                         false,
	}

	for origin, want := range tests {
		if got := OriginAllowed(allowed, origin); got != want {
			t.Errorf("OriginAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
}
