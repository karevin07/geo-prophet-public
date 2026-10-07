// Package twtext normalizes Traditional Chinese text from Taiwanese government open data.
package twtext

import "strings"

// Normalize converts full-width ASCII to half-width, unifies 臺 to 台 and trims spaces.
func Normalize(value string) string {
	builder := strings.Builder{}
	builder.Grow(len(value))
	for _, character := range value {
		switch {
		case character == '\u3000':
			builder.WriteRune(' ')
		case character >= '！' && character <= '～':
			builder.WriteRune(character - 0xFEE0)
		case character == '臺':
			builder.WriteRune('台')
		default:
			builder.WriteRune(character)
		}
	}
	return strings.TrimSpace(builder.String())
}
