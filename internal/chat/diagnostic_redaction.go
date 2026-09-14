package chat

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	privateMaterialPattern  = regexp.MustCompile(`(?is)-----BEGIN [^-]*(?:PRIVATE KEY|SECRET)[^-]*-----.*?-----END [^-]*(?:PRIVATE KEY|SECRET)[^-]*-----`)
	bearerCredentialPattern = regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*bearer\s+[^\s,;]+`)
	credentialPattern       = regexp.MustCompile(`(?i)\b(authorization|token|secret|password|private[_ -]?key)\s*[:=]\s*[^\s,;]+`)
	windowsPathPattern      = regexp.MustCompile(`(?i)\b[A-Z]:\\[^\s,;]+`)
	unixPathPattern         = regexp.MustCompile(`(?:^|[\s(])/(?:[^\s,;:)]+/?)+`)
)

// redactDiagnosticText is the only representation of arbitrary failures that
// may be emitted to product logs or diagnostics. Stable error codes and object
// identifiers are logged separately by callers.
func redactDiagnosticText(value string) string {
	value = privateMaterialPattern.ReplaceAllString(value, "<redacted-private-material>")
	value = bearerCredentialPattern.ReplaceAllString(value, "authorization=<redacted>")
	value = credentialPattern.ReplaceAllStringFunc(value, func(match string) string {
		if index := strings.IndexAny(match, ":="); index >= 0 {
			return strings.TrimSpace(match[:index]) + "=<redacted>"
		}
		return "<redacted-credential>"
	})
	value = windowsPathPattern.ReplaceAllString(value, "<path>")
	value = unixPathPattern.ReplaceAllStringFunc(value, func(match string) string {
		prefix := ""
		if len(match) > 0 && (match[0] == ' ' || match[0] == '(' || match[0] == '\t' || match[0] == '\n') {
			prefix = match[:1]
		}
		return prefix + "<path>"
	})
	return strings.TrimSpace(value)
}

func redactDiagnosticError(err error) string {
	if err == nil {
		return ""
	}
	code, _ := transferErrorInfo(err)
	message := redactDiagnosticText(err.Error())
	if code == "" || message == string(code) {
		return message
	}
	return fmt.Sprintf("%s: %s", code, message)
}
