// Copyright 2022 Cloudflare, Inc.
// Copyright 2026 svg-hush-go contributors.
// SPDX-License-Identifier: MIT

package svghush

import "github.com/hiteshshimpi-55/svg-hush-go/internal/sanitizer"

const (
	// DefaultMaxInputBytes is the maximum input size when no override is supplied.
	DefaultMaxInputBytes = sanitizer.DefaultMaxInputBytes
	// DefaultMaxOutputBytes limits XML escaping and serialization expansion.
	DefaultMaxOutputBytes = sanitizer.DefaultMaxOutputBytes
	// DefaultMaxDepth limits nesting, including elements that will be removed.
	DefaultMaxDepth = sanitizer.DefaultMaxDepth
)

var (
	// ErrInvalidSVG identifies malformed XML or an unacceptable document root.
	ErrInvalidSVG = sanitizer.ErrInvalidSVG
	// ErrLimitExceeded identifies input, output, or XML complexity limit failures.
	ErrLimitExceeded = sanitizer.ErrLimitExceeded
	// ErrInvalidOptions identifies a negative processing limit.
	ErrInvalidOptions = sanitizer.ErrInvalidOptions
)

// Options controls resource limits and URL policy. Zero limits select the
// documented defaults; negative limits are invalid. Options never enable
// arbitrary tags, event handlers, scripting, or foreign namespaces.
type Options struct {
	// MaxInputBytes limits the original UTF-8 input, including a possible BOM.
	MaxInputBytes int
	// MaxOutputBytes limits the serialized XML, including escaping expansion.
	MaxOutputBytes int
	// MaxDepth limits the nesting of both retained and discarded elements.
	MaxDepth int

	// AllowDataImages allows well-formed JPEG, PNG, and GIF data URLs by their
	// declared media type. It does not decode or verify the image itself.
	AllowDataImages bool
	// LocalReferencesOnly drops every resource URL except a #fragment. It also
	// disables data URLs, even when AllowDataImages is true.
	LocalReferencesOnly bool
}

// Sanitize removes unsupported SVG features using bounded default options.
// The input must contain exactly one svg root in the SVG namespace, encoded as
// UTF-8 XML. DOCTYPE declarations and custom entities are rejected. Output is
// deterministic UTF-8 XML. On failure, the returned byte slice is always nil.
func Sanitize(input []byte) ([]byte, error) {
	return sanitizer.Sanitize(input)
}

// SanitizeWithOptions sanitizes input using the given limits and URL policy.
// Limits apply to removed content too. The function consumes the whole document
// before returning output, so malformed trailing input cannot be accepted.
func SanitizeWithOptions(input []byte, options Options) ([]byte, error) {
	return sanitizer.SanitizeWithOptions(input, sanitizer.Options(options))
}
