# Upstream provenance

- Project: [Cloudflare SVG-Hush](https://github.com/cloudflare/svg-hush).
- Source commit: `fba34edcbb9c58d855628a67032b43aed5f2b5bd`.
- Cargo package version at that commit: `0.9.7`.
- License choice for this adaptation: MIT, retaining the upstream license text
  and `Copyright 2022 Cloudflare, Inc.` in [LICENSE](LICENSE).
- This is an unofficial adaptation, with no Cloudflare endorsement implied.

## Source mapping

| Upstream | Go adaptation |
| --- | --- |
| `src/lib.rs`: allowed elements, filtering, attribute classification | `internal/sanitizer/attributes.go`, `internal/sanitizer/sanitize.go`, `internal/sanitizer/namespaces.go` |
| `src/attrs.rs`: 255 attribute classifications | `internal/sanitizer/attributes.go` |
| `src/lib.rs`: URL and CSS filtering | `internal/sanitizer/urls.go`, `internal/sanitizer/css.go` |
| `src/data_url_filter.rs`: standard raster-image preset | `internal/sanitizer/urls.go`, `Options.AllowDataImages` |
| Embedded unit tests and `tests/tests.rs` | Go public API tests under `tests/` |
| `tests/test.xml`, `tests/filtered.xml` | Unmodified files in `tests/testdata/upstream/` |

## Intentional differences

1. This package is a native Go library using `encoding/xml` and `net/url`, not a
   binding to Rust. It has no external dependencies and does not include the
   upstream command-line program or arbitrary trusted data-URL callback API.
2. Exactly one SVG-namespaced `svg` root is required. XML is consumed through EOF
   and no partial output is returned. Prefix scopes, duplicate raw/expanded
   attributes, and matching qualified closing names are validated explicitly.
3. Only UTF-8 input is supported, optionally with a UTF-8 BOM. UTF-16, Latin-1,
   HTTP encoding overrides, custom entities, and all DOCTYPE declarations are
   rejected. The upstream fixture's DOCTYPE is removed in the compatibility test;
   the original fixture is also tested to ensure that this port rejects it.
4. Output uses Go's XML escaping and expanded closing tags, omits comments and
   processing instructions, and does not include an XML declaration. Namespace
   declarations are reconstructed from the fixed SVG namespace, never copied
   from attacker-supplied namespace URIs.
5. Input/output/nesting and fixed complexity limits are enforced. In-scope
   namespace bindings are bounded to avoid unbounded scope-map growth.
6. Web backslashes and ASCII URL controls are normalized before `net/url` parsing.
   HTTP, HTTPS, FTP, and relative references are supported. Other schemes are
   dropped rather than emulating all WHATWG/Rust URL behavior. Parentheses in
   emitted URL paths are percent-encoded. This is not complete WHATWG URL parity.
7. CSS comments, unclosed strings, unstable rewrites, and unsupported functions cause the entire
   value or stylesheet to be removed. A narrow function set covers color,
   transform, arithmetic, local-font, and filtered `url()` syntax. This also drops
   two malformed quoted/backslash values retained as CSS tails in the upstream
   expected fixture. Rewritten CSS must remain valid under the syntax gate and
   unchanged by a second filtering pass; otherwise it is dropped. Leading and
   trailing ASCII whitespace is canonicalized. No general CSS or inline-HTML
   safety guarantee is made.
8. Raster data URLs are parsed, their payload encodings validated, and their
   contents re-encoded as canonical base64. Nested SVG and non-JPEG/PNG/GIF MIME
   types remain forbidden. Media type validation is not image validation.
9. `LocalReferencesOnly` is an additional restrictive mode that drops all
   non-fragment references, including data URLs.

## Updating the port

Review upstream changes, especially security fixes and XML/URL dependency
changes. Compare the 82 element entries and 255 attribute classifications with
the recorded upstream revision, then update the implementation and tests as
needed. Retain the original fixture bytes and apply explicit compatibility
adjustments in tests rather than silently replacing expected behavior.

Record the new source commit and intentional differences here. Run unit tests,
race checks, lint, fuzzing, and browser regression tests before a security-policy
change is released. The Rust binary has not been executed as an oracle by this
repository's normal Go tests.
