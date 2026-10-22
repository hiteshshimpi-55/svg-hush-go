# Security

This is a new, unofficial Go adaptation of Cloudflare SVG-Hush. It has not
undergone an independent security audit. No supported-version or response-time
guarantee is currently made; use a reviewed, pinned revision and keep the Go
toolchain updated.

## Reporting

Use [GitHub private vulnerability reporting](https://github.com/hiteshshimpi-55/svg-hush-go/security/advisories/new)
if the repository owner has enabled it. Otherwise contact the maintainer through
the contact information on their GitHub profile. Do not post exploit details or
private uploaded files in a public issue while a bypass is unresolved. For a
problem shared with the upstream Rust project, coordinate disclosure with its
maintainers too.

Include the package revision, Go version, minimal input, options, sanitized
output, browser/version, delivery headers, and the observable security effect.
Avoid credentials, private files, or production URLs with access tokens.

## Scope and deployment

The package accepts bounded, standalone UTF-8 SVG XML. It never fetches resources
or evaluates scripts. It removes unsupported content and limits XML complexity;
it is not an antivirus, raster-image validator, rendering sandbox, or protection
for arbitrary frontend HTML sinks. SVG filters/animation can still be expensive
to render. Retained data attributes must remain untrusted to consuming code.

Prefer `LocalReferencesOnly` for self-contained previews. Always serve the
successful output, never the original on failure. Isolate user content, configure
appropriate response headers, and keep original-file downloads separate.

Changing parsing or rendering contexts can invalidate sanitizer assumptions.
Do not insert standalone SVG output into an HTML DOM or transform it with
another library without separately reviewing that sink. Server response headers
do not automatically transfer to client-created blob URLs.
