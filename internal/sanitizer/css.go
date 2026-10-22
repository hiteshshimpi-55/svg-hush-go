// Copyright 2022 Cloudflare, Inc.
// Copyright 2026 svg-hush-go contributors.
// SPDX-License-Identifier: MIT

package sanitizer

import "strings"

// filterCSS ports upstream's conservative URL-function filtering, with an
// additional syntax gate. Browser features such as image-set("https://...")
// can load resources without url(), so unknown functions and CSS comments are
// rejected rather than silently passed through.
func filterCSS(value string, options Options) string {
	if !supportedCSSSyntax(value) {
		return ""
	}
	output := filterCSSChunks(value, options)
	// Removing an escape-bearing chunk can change quote or function boundaries.
	// Reject that transformation unless its output is accepted and stable under
	// the same policy. This is bounded to two passes, never recursive retries.
	if !supportedCSSSyntax(output) || filterCSSChunks(output, options) != output {
		return ""
	}
	return output
}

func filterCSSChunks(value string, options Options) string {
	var output strings.Builder
	insideURL := false
	for _, chunk := range splitInclusive(value, "(") {
		if insideURL {
			resource, rest, found := strings.Cut(chunk, ")")
			if !found {
				continue
			}
			chunk = rest
			resource = strings.TrimSpace(strings.Trim(strings.TrimSpace(resource), "\"'"))
			filtered, ok := filterURL(resource, options)
			if !ok || strings.ContainsAny(filtered, "()'\"\\ \t\r\n\f") {
				filtered = "#"
			}
			output.WriteString(filtered)
			output.WriteByte(')')
			insideURL = false
		}
		for _, part := range splitInclusive(chunk, ";{},") {
			if strings.Contains(part, "\\") || strings.Contains(strings.ToLower(part), "@import") {
				if last := part[len(part)-1]; last == '{' || last == '}' {
					output.WriteByte(last)
				}
				continue
			}
			output.WriteString(part)
			insideURL = strings.HasSuffix(strings.ToLower(part), "url(")
		}
	}
	if insideURL {
		output.WriteByte(')')
	}
	return strings.TrimSpace(output.String())
}

func splitInclusive(value, delimiters string) []string {
	var chunks []string
	start := 0
	for index := range value {
		if strings.IndexByte(delimiters, value[index]) >= 0 {
			chunks = append(chunks, value[start:index+1])
			start = index + 1
		}
	}
	if start < len(value) {
		chunks = append(chunks, value[start:])
	}
	return chunks
}

func supportedCSSSyntax(value string) bool {
	if strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return false
	}
	var quote byte
	for index := 0; index < len(value); index++ {
		character := value[index]
		if quote != 0 {
			if character == '\\' {
				index++
				continue
			}
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		if character != '(' {
			continue
		}
		start := index
		for start > 0 && isCSSIdentifier(value[start-1]) {
			start--
		}
		function := strings.ToLower(value[start:index])
		if function == "url" {
			// The upstream filter discards nested parentheses and sanitizes the
			// resource independently. Do not interpret its bytes as functions.
			if end := strings.IndexByte(value[index+1:], ')'); end >= 0 {
				index += end + 1
			} else {
				return true // The URL filter closes or removes incomplete input.
			}
			continue
		}
		switch function {
		case "", "rgb", "rgba", "hsl", "hsla", "hwb", "lab", "lch", "oklab", "oklch",
			"matrix", "matrix3d", "translate", "translatex", "translatey", "translatez", "translate3d",
			"scale", "scalex", "scaley", "scalez", "scale3d", "rotate", "rotatex", "rotatey", "rotatez", "rotate3d",
			"skew", "skewx", "skewy", "calc", "min", "max", "clamp", "local":
		default:
			return false
		}
	}
	return quote == 0
}

func isCSSIdentifier(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || character == '-' || character == '_'
}
