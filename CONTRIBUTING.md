# Contributing

Use Go 1.24 or newer. Keep the public API small, use descriptive Go names, run
`gofmt`, and document exported identifiers. The library must not perform network
or filesystem I/O, log user content, introduce mutable shared state, or return
partially sanitized bytes with an error.

Every parser or policy change needs a regression case covering its externally
observable behavior. For namespace, URL, CSS, or animation changes, include
adversarial inputs and explain why the change cannot introduce active content or
cross-origin resource loading. Preserve the upstream license and update
`UPSTREAM.md` when changing compatibility behavior. Keep the runtime dependency
set empty unless a reviewed dependency offers a demonstrated security benefit.

Before submitting:

```sh
gofmt -w .
go test -race -coverpkg=./... -cover ./...
go vet ./...
golangci-lint run ./...
go test ./tests -run '^$' -fuzz '^FuzzSanitize$' -fuzztime 30s
```

Use golangci-lint v2.15.0. Add minimized fuzzer failures to
`testdata/fuzz/` in the package containing that fuzz target so ordinary `go test` runs them as regression cases. Security
policy changes also need browser tests in the intended standalone SVG delivery
context; XML assertions alone do not establish browser safety. See the optional
`-tags browser` regression suite in the README.

See `SECURITY.md` for private vulnerability reporting.
