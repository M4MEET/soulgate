package components

import (
	"regexp"
)

// urlPattern matches http and https URLs, stopping at whitespace or closing parenthesis.
var urlPattern = regexp.MustCompile(`https?://[^\s)]+`)

// Hyperlink wraps text in an OSC-8 terminal hyperlink escape sequence.
// Terminals that support OSC-8 (iTerm2, WezTerm, Windows Terminal, etc.)
// render the text as a clickable link. Unsupporting terminals show plain text.
//
// OSC-8 format: ESC]8;;URL ESC\TEXT ESC]8;; ESC\
func Hyperlink(url, text string) string {
	return "\033]8;;" + url + "\033\\" + text + "\033]8;;\033\\"
}

// AutolinkURLs detects URLs in text and wraps them in OSC-8 hyperlinks.
// Any sequence matching https?://[^\s)]+ is linked to itself.
func AutolinkURLs(text string) string {
	return urlPattern.ReplaceAllStringFunc(text, func(url string) string {
		return Hyperlink(url, url)
	})
}
