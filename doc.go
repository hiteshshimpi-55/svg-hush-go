// Package svghush sanitizes standalone SVG XML documents using an allowlist
// adapted from Cloudflare's svg-hush. It removes scripting and foreign content
// and rewrites resource URLs to same-origin paths. It does not fetch resources.
//
// Sanitize uses bounded defaults and rejects data URLs. SanitizeWithOptions can
// allow embedded raster images or restrict references to local fragments.
// Calls are independent and safe to execute concurrently. Returned bytes do not
// alias the input. Errors always return a nil output, never partial SVG.
//
// This is an unofficial Go adaptation, not a byte-for-byte compatible port or an
// independently audited security product. Serve the result as image/svg+xml
// with a restrictive Content-Security-Policy. See the README for policy details
// and intentional differences from upstream.
package svghush
