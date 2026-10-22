// Copyright 2022 Cloudflare, Inc.
// Copyright 2026 svg-hush-go contributors.
// SPDX-License-Identifier: MIT

package sanitizer

import (
	"encoding/base64"
	"mime"
	"net/url"
	"strings"
)

const relativePathPrefix = "/__relpath_prefix__/"

// filterURL retains only relative references. Hierarchical web URLs lose their
// authority, as in upstream. Go's net/url is not a WHATWG parser: normalize web
// backslashes and control characters first, and reject unsupported schemes.
func filterURL(value string, options Options) (string, bool) {
	value = strings.Trim(value, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f ")
	value = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(value)
	if options.LocalReferencesOnly {
		if !strings.HasPrefix(value, "#") {
			return "", false
		}
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		return filterDataURL(value, options)
	}
	value = strings.ReplaceAll(value, "\\", "/")
	reference, err := url.Parse(value)
	if err != nil || reference.Opaque != "" {
		return "", false
	}
	if reference.Scheme != "" {
		switch strings.ToLower(reference.Scheme) {
		case "http", "https", "ftp":
			if reference.Host == "" {
				return "", false
			}
		default:
			return "", false
		}
	}
	if reference.Host != "" && reference.Path == "" {
		reference.Path = "/"
	}
	base := &url.URL{Scheme: "https", Host: "127.0.0.1", Path: relativePathPrefix}
	resolved := base.ResolveReference(reference)
	path := strings.TrimPrefix(resolved.EscapedPath(), relativePathPrefix)
	if strings.HasPrefix(path, "//") {
		return "", false
	}
	path = strings.ReplaceAll(path, ":", "%3a")
	if resolved.RawQuery != "" || resolved.ForceQuery {
		// Go preserves raw query bytes. Quote characters are not allowed to
		// escape into a CSS URL, and spaces must be percent-encoded.
		query := strings.NewReplacer(" ", "%20", "\"", "%22", "<", "%3C", ">", "%3E").Replace(resolved.RawQuery)
		path += "?" + query
	}
	if path == "/" {
		return "", false
	}
	if resolved.Fragment != "" || strings.HasSuffix(value, "#") {
		path += "#" + resolved.EscapedFragment()
	}
	path = strings.NewReplacer("(", "%28", ")", "%29").Replace(path)
	return path, true
}

func filterDataURL(value string, options Options) (string, bool) {
	if !options.AllowDataImages || options.LocalReferencesOnly {
		return "", false
	}
	metadata, payload, found := strings.Cut(value[len("data:"):], ",")
	if !found {
		return "", false
	}
	isBase64 := strings.HasSuffix(strings.ToLower(metadata), ";base64")
	if isBase64 {
		metadata = metadata[:len(metadata)-len(";base64")]
	}
	mediaType, _, err := mime.ParseMediaType(metadata)
	if err != nil {
		return "", false
	}
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif":
	default:
		return "", false
	}
	payload, _, _ = strings.Cut(payload, "#")
	decodedPayload, err := url.PathUnescape(payload)
	if err != nil {
		return "", false
	}
	data := []byte(decodedPayload)
	if isBase64 {
		encoded := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "", "\f", "").Replace(decodedPayload)
		data, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			data, err = base64.RawStdEncoding.DecodeString(encoded)
		}
		if err != nil {
			return "", false
		}
	}
	// Canonical output avoids carrying encoded delimiters and MIME parameters
	// into a second parser. The media type itself is not image validation.
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), true
}
