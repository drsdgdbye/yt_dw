package validator

import "testing"

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name  string
		rawURL string
		want  bool
	}{
		{"valid youtube", "https://youtube.com/watch?v=abc123", true},
		{"valid subdomain", "https://sub.domain.com", true},
		{"valid ipv4", "https://127.0.0.1", true},
		{"valid localhost", "https://localhost", true},
		{"valid with query", "https://example.com/path?q=1#frag", true},
		{"valid trailing slash", "https://example.com/", true},
		{"invalid http scheme", "http://youtube.com/watch?v=abc123", false},
		{"invalid ftp scheme", "ftp://youtube.com", false},
		{"empty string", "", false},
		{"only spaces", "  ", false},
		{"no host", "https://", false},
		{"no dot in host", "https://myserver", false},
		{"no dot no scheme", "myserver", false},
		{"relative path", "/path/to/video", false},
		{"ipv6", "https://[::1]", true},
		{"russian domain", "https://домен.рф", true},
		{"with auth", "https://user:pass@example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidURL(tt.rawURL); got != tt.want {
				t.Errorf("IsValidURL(%q) = %v, want %v", tt.rawURL, got, tt.want)
			}
		})
	}
}
