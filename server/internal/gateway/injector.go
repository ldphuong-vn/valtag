package gateway

import (
	"net/http"
	"strings"
)

// MatchPath checks if a URL path matches a glob-style pattern.
// Supports: "/*" (match all), "/v1/*" (prefix), exact match.
func MatchPath(pattern, path string) bool {
	if pattern == "/*" || pattern == "*" {
		return true
	}
	// Trailing wildcard: "/v1/*" matches "/v1/anything/here"
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	// Exact match
	return pattern == path
}

// InjectCredential replaces the placeholder or sets the credential in the request
// based on the route's injection configuration.
func InjectCredential(req *http.Request, route *ProxyRoute, secretValue string) {
	value := formatValue(route.InjectionFormat, secretValue)

	switch route.InjectionType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+secretValue)
	case "header":
		req.Header.Set(route.InjectionKey, value)
	case "query":
		q := req.URL.Query()
		q.Set(route.InjectionKey, secretValue)
		req.URL.RawQuery = q.Encode()
	default:
		req.Header.Set(route.InjectionKey, value)
	}
}

// ScanForPlaceholder checks all request header values for a valtag_pk_ placeholder.
func ScanForPlaceholder(req *http.Request) string {
	for _, values := range req.Header {
		for _, v := range values {
			if pk := extractPlaceholder(v); pk != "" {
				return pk
			}
		}
	}
	return ""
}

// StripPlaceholder removes the placeholder value from request headers,
// replacing it with an empty string so it doesn't leak to the target API.
func StripPlaceholder(req *http.Request, placeholder string) {
	for key, values := range req.Header {
		for i, v := range values {
			if strings.Contains(v, placeholder) {
				req.Header[key][i] = strings.ReplaceAll(v, placeholder, "")
			}
		}
	}
}

// extractPlaceholder finds a valtag_pk_ token within a header value.
func extractPlaceholder(value string) string {
	const prefix = "valtag_pk_"
	idx := strings.Index(value, prefix)
	if idx < 0 {
		return ""
	}
	// Extract from prefix to next space or end
	rest := value[idx:]
	end := strings.IndexAny(rest, " \t,;\"'")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// formatValue applies the injection format template.
// {value} is replaced with the actual secret.
func formatValue(format, secret string) string {
	if format == "" || format == "{value}" {
		return secret
	}
	return strings.ReplaceAll(format, "{value}", secret)
}
