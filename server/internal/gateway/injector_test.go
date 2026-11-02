package gateway

import (
	"net/http"
	"testing"
)

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/*", "/anything", true},
		{"/*", "/", true},
		{"/*", "/a/b/c", true},
		{"*", "/foo", true},
		{"/v1/*", "/v1/chat/completions", true},
		{"/v1/*", "/v1", true},
		{"/v1/*", "/v1/", true},
		{"/v1/*", "/v2/models", false},
		{"/v1/chat/*", "/v1/chat/completions", true},
		{"/v1/chat/*", "/v1/models", false},
		{"/exact", "/exact", true},
		{"/exact", "/exact/more", false},
		{"/exact", "/other", false},
	}
	for _, tt := range tests {
		got := MatchPath(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("MatchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestInjectCredential_Bearer(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://api.openai.com/v1/models", nil)
	route := &ProxyRoute{
		InjectionType:   "bearer",
		InjectionKey:    "Authorization",
		InjectionFormat: "Bearer {value}",
	}
	InjectCredential(req, route, "sk-real-key-123")

	got := req.Header.Get("Authorization")
	want := "Bearer sk-real-key-123"
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestInjectCredential_Header(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://api.example.com/data", nil)
	route := &ProxyRoute{
		InjectionType:   "header",
		InjectionKey:    "X-Api-Key",
		InjectionFormat: "{value}",
	}
	InjectCredential(req, route, "my-secret-key")

	got := req.Header.Get("X-Api-Key")
	if got != "my-secret-key" {
		t.Errorf("X-Api-Key = %q, want %q", got, "my-secret-key")
	}
}

func TestInjectCredential_Query(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://api.example.com/data?foo=bar", nil)
	route := &ProxyRoute{
		InjectionType:   "query",
		InjectionKey:    "api_key",
		InjectionFormat: "{value}",
	}
	InjectCredential(req, route, "qk-12345")

	got := req.URL.Query().Get("api_key")
	if got != "qk-12345" {
		t.Errorf("api_key query = %q, want %q", got, "qk-12345")
	}
	// Existing params preserved
	if req.URL.Query().Get("foo") != "bar" {
		t.Error("existing query param 'foo' was lost")
	}
}

func TestScanForPlaceholder(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer valtag_pk_abc123def456")

	got := ScanForPlaceholder(req)
	if got != "valtag_pk_abc123def456" {
		t.Errorf("ScanForPlaceholder = %q, want %q", got, "valtag_pk_abc123def456")
	}
}

func TestScanForPlaceholder_None(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer sk-real-key")

	got := ScanForPlaceholder(req)
	if got != "" {
		t.Errorf("ScanForPlaceholder = %q, want empty", got)
	}
}

func TestStripPlaceholder(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer valtag_pk_test123")

	StripPlaceholder(req, "valtag_pk_test123")

	got := req.Header.Get("Authorization")
	if got != "Bearer " {
		t.Errorf("After strip, Authorization = %q, want %q", got, "Bearer ")
	}
}

func TestExtractPlaceholder_InMiddleOfValue(t *testing.T) {
	got := extractPlaceholder("Bearer valtag_pk_xyz789 extra")
	if got != "valtag_pk_xyz789" {
		t.Errorf("extractPlaceholder = %q, want %q", got, "valtag_pk_xyz789")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		format string
		secret string
		want   string
	}{
		{"{value}", "key123", "key123"},
		{"", "key123", "key123"},
		{"Bearer {value}", "key123", "Bearer key123"},
		{"token={value}&extra=1", "abc", "token=abc&extra=1"},
	}
	for _, tt := range tests {
		got := formatValue(tt.format, tt.secret)
		if got != tt.want {
			t.Errorf("formatValue(%q, %q) = %q, want %q", tt.format, tt.secret, got, tt.want)
		}
	}
}
