# svg-hush-go

A pure Go version of [Cloudflare's SVG-Hush](https://github.com/cloudflare/svg-hush),
adapted for sanitizing untrusted, standalone SVG documents on the backend.

This is an **unofficial, independently maintained Go adaptation**. It is not
affiliated with or endorsed by Cloudflare. Its allowlists, attribute
classifications, URL policy, and conservative CSS URL filtering are derived from
Cloudflare's Rust implementation. This package has **no third-party dependencies**
and requires no Rust, Node.js, cgo, subprocesses, or network access at runtime.

The implementation is based on upstream commit
[`fba34edcbb9c58d855628a67032b43aed5f2b5bd`](https://github.com/cloudflare/svg-hush/tree/fba34edcbb9c58d855628a67032b43aed5f2b5bd)
(upstream package version `0.9.7`). See [UPSTREAM.md](UPSTREAM.md) for provenance
and compatibility differences.

## Install

Requires **Go 1.24 or newer**.

```sh
go get github.com/hiteshshimpi-55/svg-hush-go
```

The repository must be pushed to GitHub before this command can resolve it.
For a private repository, configure Git authentication and `GOPRIVATE` as usual.
Once a reviewed version is tagged, pin that version in consuming applications.

## Use

The import path ends in `svg-hush-go`; the Go package name is `svghush`.

```go
package main

import (
    "fmt"
    "log"

    svghush "github.com/hiteshshimpi-55/svg-hush-go"
)

func main() {
    input := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(2)</script><rect width="10"/></svg>`)
    output, err := svghush.Sanitize(input)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(output))
}
```

Output:

```xml
<svg xmlns="http://www.w3.org/2000/svg"><rect width="10"></rect></svg>
```

`Sanitize` does not modify its input, and the returned bytes do not alias it.
Calls are independent and safe for concurrent use. Every error returns a `nil`
output: **never serve the original input as a fallback on sanitization failure**.

## Policy

- Require a single `<svg>` root in `http://www.w3.org/2000/svg`.
- Keep supported SVG elements and attributes; remove unknown elements with their
  entire subtrees. Keep shapes, text, gradients, filters, and supported animation.
- Remove scripts, event handlers, embedded HTML (`foreignObject`), foreign
  namespaces, comments, and processing instructions.
- Reject malformed XML, duplicate attributes, invalid namespace bindings,
  undeclared prefixes, document type declarations, and custom entities.
- Resolve namespace prefixes before filtering. Serialize retained SVG elements
  in a fixed SVG namespace and convert accepted `xlink:href` attributes to `href`.
- Remove links from `<a>`. Only resource-bearing elements such as `<image>`,
  `<use>`, and gradients may retain a filtered `href`.
- Prevent SMIL `attributeName` from targeting URLs, styles, event handlers, or
  unknown attributes. Supported animation of ordinary numeric/text attributes
  is retained; this package does not strip all animation.
- Preserve upstream `data-*` text attributes and normalized `aria-*` attributes.
  Applications must not reinterpret these as trusted HTML, URLs, or commands.
- Filter CSS `url(...)` references and `@import`, and discard escape-bearing
  chunks. Additionally, drop CSS values/stylesheets with comments, unclosed
  strings, or unsupported functions, including `image-set(...)`, `image(...)`,
  and `expression(...)`. Drop rewritten CSS if it is not stable under a second
  filtering pass. This is a deliberately restricted CSS policy, not a
  general-purpose CSS sanitizer.
- By default, drop all `data:` URLs. Optionally allow JPEG, PNG, and GIF data URLs,
  validating their encoding and canonicalizing them to base64. Their declared
  media type is **not** proof that their bytes form a valid, harmless image.

### URLs: same-origin paths or local fragments

The default follows SVG-Hush's approach of stripping the origin from supported
hierarchical URLs. It does **not** preserve an external origin:

| Input | Default output |
| --- | --- |
| `https://example.com/assets/logo.png` | `/assets/logo.png` |
| `//example.com/assets/logo.png` | `/assets/logo.png` |
| `assets/logo.png` | `assets/logo.png` |
| `#gradient` | `#gradient` |
| `javascript:alert(1)` | Removed |
| `data:image/svg+xml,...` | Removed, including when raster data is enabled |

Same-origin references can still cause requests to the serving host, whose
redirects and endpoint behavior are outside the sanitizer's control. For
self-contained previews, use **`LocalReferencesOnly: true`** to retain only
`#fragment` references:

```go
output, err := svghush.SanitizeWithOptions(input, svghush.Options{
    LocalReferencesOnly: true,
    MaxInputBytes:       2 << 20, // 2 MiB
    MaxOutputBytes:      4 << 20, // 4 MiB
    MaxDepth:            128,
})
if err != nil {
    // Reject the preview. Do not fall back to the unsanitized input.
    return err
}
```

`LocalReferencesOnly` takes precedence over `AllowDataImages`. Without that
restriction, opt in to the upstream standard-raster-image preset with:

```go
output, err := svghush.SanitizeWithOptions(input, svghush.Options{
    AllowDataImages: true,
})
```

### Processing limits

| Resource | Default |
| --- | --- |
| Input bytes | 5 MiB |
| Serialized output bytes | 10 MiB |
| Element nesting | 256 |
| Element count | 100,000 |
| Attributes per element, including namespace declarations | 200 |
| Namespace bindings in scope | 200 |
| XML name bytes, prefix plus local name | 1,000 |

`Options` can override input size, output size, and depth. Zero selects the
default; a negative limit is invalid. Fixed complexity limits also apply to
discarded subtrees. Input must be UTF-8, optionally with a UTF-8 BOM. Do not pass
compressed data or an entire archive as SVG input.

The API takes `[]byte`, so applications should limit uploads/download reads
**before allocating the input**, for example with `io.LimitReader(maxBytes+1)`.
The package performs no network or filesystem I/O and does not decode or render
images. XML limits are not rendering CPU, pixel-count, or animation limits.

Errors can be classified with `errors.Is`:

```go
switch {
case errors.Is(err, svghush.ErrInvalidSVG):
    // Unsupported or malformed document.
case errors.Is(err, svghush.ErrLimitExceeded):
    // Input, output, nesting, or fixed complexity limit.
case errors.Is(err, svghush.ErrInvalidOptions):
    // Invalid application configuration.
}
```

## Backend preview integration

Presigned uploads can remain direct to object storage. When a preview is
requested, authorize access, read bounded object bytes, call this package, and
return **those sanitized bytes**. Do not validate one object and then serve an
unprocessed presigned URL: it can expose different bytes after an overwrite.

Keep original downloads separate from previews. If the product requires
sanitized bytes in storage, add server-side upload processing; preview-time
sanitization alone does not change the original object.

Serve output as `Content-Type: image/svg+xml; charset=utf-8` with
`X-Content-Type-Options: nosniff`. For self-contained SVG previews, an example
resource-response policy is:

```http
Content-Security-Policy: sandbox; default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src 'none'; base-uri 'none'; form-action 'none'
```

Choose headers for the actual delivery context, and test the result in the
browsers you support. A server's response headers are not attached to a new
`Blob` URL created later by a frontend. Keep user content isolated, preferably
on a separate origin without application cookies. Do not inject the output into
an HTML page or transform it into a different markup context and assume that the
same security properties still apply.

## Development

### Repository layout

```text
svg-hush-go/
├── sanitize.go                 # Public API, options, defaults, and errors
├── doc.go                      # Public package documentation
├── example_test.go             # Runnable examples attached to the public API
├── internal/
│   └── sanitizer/
│       ├── sanitize.go         # XML document processing
│       ├── namespaces.go       # Namespace validation
│       ├── attributes.go       # SVG element and attribute allowlists
│       ├── css.go              # CSS filtering
│       └── urls.go             # URL and data-image filtering
├── tests/
│   ├── sanitize_test.go        # Black-box tests against the public import path
│   ├── fuzz_test.go            # Whole-document fuzzing
│   ├── browser_test.go         # Optional Chromium regression checks
│   └── testdata/               # Upstream fixtures and browser harness
└── .github/workflows/ci.yml
```

Import the module root; `internal/sanitizer` is private to this module.
Public behavior and browser tests live in `tests/`; examples remain at
the root so Go documentation associates them with the exported API.

### Checks

```sh
go test ./...
go test -race -coverpkg=./... -cover ./...
go vet ./...
go test ./tests -run '^$' -fuzz '^FuzzSanitize$' -fuzztime 30s
```

Lint with `golangci-lint v2.15.0` and the checked-in `.golangci.yml`:

```sh
golangci-lint run ./...
```

Tests cover public URL/CSS behavior and a semantic comparison with the
upstream whole-document fixture, with documented differences applied. Go's XML
serializer produces different formatting from Rust's serializer. These tests
are **not** a live differential run against the Rust binary. Fuzz targets check
idempotence and independently inspect retained XML for forbidden content.

### Optional browser regression checks

`tests/browser_test.go` is enabled only with `-tags browser`. Its test-only Node/
Playwright harness opens sanitized SVGs as same-origin blob documents in an
unsandboxed Chromium frame. A malicious positive control must execute first;
sanitized cases must retain a benign rectangle while executing no scripts and
making no resource requests under the fragment-only policy.

Install Node, then install Playwright outside the Go module:

```sh
npm install --prefix /tmp/svg-hush-browser-tools --no-save --no-package-lock playwright
/tmp/svg-hush-browser-tools/node_modules/.bin/playwright install chromium
SVG_HUSH_PLAYWRIGHT=/tmp/svg-hush-browser-tools/node_modules/playwright/index.mjs \
  go test -tags browser -run '^TestBrowserRegression$' -v ./tests
```

Set `SVG_HUSH_NODE` to an absolute Node executable if it is not on `PATH`, and
`SVG_HUSH_CHROMIUM` to use a particular installed Chromium/Chrome executable.
These tools are not used by the package at runtime or by ordinary Go tests.
The optional suite currently targets Chromium; cross-browser testing remains
necessary before claiming support for other rendering engines.

This implementation has not undergone an independent security audit. Passing
tests and fuzzing do not prove the absence of sanitizer bypasses. See
[SECURITY.md](SECURITY.md) for reporting and deployment guidance and
[CONTRIBUTING.md](CONTRIBUTING.md) for changes to the security policy.

## License and attribution

MIT. The adapted code, allowlists, tests, and fixtures retain Cloudflare's
copyright notice. New Go-specific work is also MIT-licensed. See [LICENSE](LICENSE)
and [UPSTREAM.md](UPSTREAM.md). Cloudflare's upstream manifest offers MIT or
Apache-2.0; this adaptation uses the MIT option.
