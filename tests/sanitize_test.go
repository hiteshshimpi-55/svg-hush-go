// Copyright 2022 Cloudflare, Inc.
// Copyright 2026 svg-hush-go contributors.
// SPDX-License-Identifier: MIT

package tests

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	svghush "github.com/hiteshshimpi-55/svg-hush-go"
)

const (
	svgNamespace   = "http://www.w3.org/2000/svg"
	attributeLimit = 200
	nameByteLimit  = 1000
	elementLimit   = 100000
)

func TestSanitize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "shapes", body: `<rect width="10" height="20" fill="red"/>`, want: `<rect fill="red" height="20" width="10"/>`},
		{name: "unicode text", body: `<text title="你好">Hello 🌍 &amp; friends</text>`, want: `<text title="你好">Hello 🌍 &amp; friends</text>`},
		{name: "scripts", body: `<script>alert(1)</script><rect onload="alert(1)" onclick="x"/>`, want: `<rect/>`},
		{name: "foreign content subtree", body: `<foreignObject><svg><text>hidden</text></svg></foreignObject><text>visible</text>`, want: `<text>visible</text>`},
		{name: "unknown subtree", body: `<unknown><circle/></unknown><path/>`, want: `<path/>`},
		{name: "wrong namespace", body: `<g xmlns="https://example.invalid"><rect/></g><circle/>`, want: `<circle/>`},
		{name: "namespace aliases", body: `<s:rect xmlns:s="http://www.w3.org/2000/svg" width="2"/>`, want: `<rect width="2"/>`},
		{name: "namespace injection", body: `<s:svg xmlns:s="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg&quot; onload=&quot;alert(1)"><rect/></s:svg>`, want: `<svg/>`},
		{name: "foreign event attribute", body: `<rect xmlns:x="http://www.w3.org/1999/xhtml" x:onload="x" x:fill="red"/>`, want: `<rect/>`},
		{name: "xlink becomes href", body: `<use xmlns:x="http://www.w3.org/1999/xlink" x:href="#icon"/>`, want: `<use href="#icon"/>`},
		{name: "href collision", body: `<use xmlns:x="http://www.w3.org/1999/xlink" x:href="#first" href="#second"/>`, want: `<use href="#first"/>`},
		{name: "foreign href cannot shadow", body: `<use xmlns:x="urn:untrusted" x:href="#wrong" href="#right"/>`, want: `<use href="#right"/>`},
		{name: "anchor links removed", body: `<a href="#icon">link</a><rect href="/image"/>`, want: `<a>link</a><rect/>`},
		{name: "remote image authority removed", body: `<image href="https://evil.invalid/picture.png"/>`, want: `<image href="/picture.png"/>`},
		{name: "script scheme", body: `<image href="java&#x9;script:alert(1)"/>`, want: `<image/>`},
		{name: "data rejected by default", body: `<image href="data:image/png;base64,aGk="/>`, want: `<image/>`},
		{name: "safe animation target", body: `<animate attributeName="opacity" values="0;1"/>`, want: `<animate attributeName="opacity" values="0;1"/>`},
		{name: "unsafe animation targets", body: `<set attributeName="href" to="javascript:alert(1)"/><set attributeName="onload" to="x"/>`, want: `<set to=""/><set to="x"/>`},
		{name: "aria and data", body: `<g data-label="你好" aria-label="héllo" unknown="no"/>`, want: `<g data-label="你好" aria-label="hllo"/>`},
		{name: "URL functions", body: `<rect fill="url(https://evil.invalid/p.svg#g)"/>`, want: `<rect fill="url(/p.svg#g)"/>`},
		{name: "inline CSS", body: `<g style="fill:red; background:url(//evil.invalid);"/>`, want: `<g style="fill:red; background:url(#);"/>`},
		{name: "CSS text comments and cdata", body: `<style>.a {fill:u<!--comment-->r<![CDATA[l]]>(https://evil.invalid/p.svg)}</style>`, want: `<style>.a {fill:url(/p.svg)}</style>`},
		{name: "CSS child elements removed", body: `<style>x {fill:u<foreignObject>hidden</foreignObject>rl(#x)}</style>`, want: `<style>x {fill:url(#x)}</style>`},
		{name: "CSS image set denied", body: `<g style="background:image-set('https://evil.invalid/p.png' 1x)"/>`, want: `<g style=""/>`},
		{name: "CSS comments denied", body: `<style>@im/**/port 'https://evil.invalid/a.css';</style>`, want: `<style/>`},
		{name: "XML instructions removed", body: `<?xml-stylesheet href="https://evil.invalid/x.css"?><rect/><!--comment-->`, want: `<rect/>`},
		{name: "text escaped", body: `<text><![CDATA[<script>"'&]]></text>`, want: `<text>&lt;script&gt;"'&amp;</text>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := `<svg xmlns="` + svgNamespace + `">` + test.body + `</svg>`
			output, err := svghush.Sanitize([]byte(input))
			if err != nil {
				t.Fatalf("svghush.Sanitize(%q) error = %v, want nil", input, err)
			}
			want := `<svg xmlns="` + svgNamespace + `">` + test.want + `</svg>`
			if gotTokens, wantTokens := xmlTokens(t, output), xmlTokens(t, []byte(want)); !reflect.DeepEqual(gotTokens, wantTokens) {
				t.Errorf("svghush.Sanitize(%q) = %s, want equivalent XML %s", input, output, want)
			}
			assertSafeOutput(t, output)
		})
	}
}

func TestSanitizeRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()
	tests := []string{
		``, `hello`, `<svg/>`, `<svg xmlns="wrong"/>`, `<rect xmlns="http://www.w3.org/2000/svg"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg">`,
		`<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg"/>trailing`,
		`<svg xmlns="http://www.w3.org/2000/svg"/><`,
		`<!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<!DOCTYPE svg [<!ENTITY x "hello">]><svg xmlns="http://www.w3.org/2000/svg">&x;</svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><script><bad></script></bad></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" x="1" x="2"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="urn:x" xmlns:y="urn:x" x:a="1" y:a="2"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="urn:x" xmlns:x="urn:y"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><unknown:rect/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" unknown:attr="x"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xml="wrong"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="http://www.w3.org/XML/1998/namespace"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xmlns="urn:x"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:x=""/>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><a:g xmlns:a="http://www.w3.org/2000/svg" xmlns:b="http://www.w3.org/2000/svg"></b:g></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" x:a:b="bad"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg">&unknown;</svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"/><?xml version="1.0"?>`,
		`<?xml version="1.0"?><?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<!--comment--><?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<?xml unexpected="yes"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<?XML version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<?xml version="1.0" encoding="iso-8859-1"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`</svg>`,
		"<svg xmlns=\"http://www.w3.org/2000/svg\">\x00</svg>",
		"<svg xmlns=\"http://www.w3.org/2000/svg\">\xff</svg>",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			output, err := svghush.Sanitize([]byte(input))
			if !errors.Is(err, svghush.ErrInvalidSVG) || output != nil {
				t.Errorf("svghush.Sanitize(%q) = (%q, %v), want (nil, svghush.ErrInvalidSVG)", input, output, err)
			}
		})
	}
}

func TestSanitizeLimits(t *testing.T) {
	t.Parallel()
	minimal := `<svg xmlns="http://www.w3.org/2000/svg"/>`
	tests := []struct {
		name    string
		input   string
		options svghush.Options
	}{
		{name: "input", input: minimal, options: svghush.Options{MaxInputBytes: len(minimal) - 1}},
		{name: "output", input: minimal, options: svghush.Options{MaxOutputBytes: 10}},
		{name: "escaped output", input: `<svg xmlns="http://www.w3.org/2000/svg"><text>` + strings.Repeat("&amp;", 100) + `</text></svg>`, options: svghush.Options{MaxOutputBytes: 100}},
		{name: "depth", input: `<svg xmlns="http://www.w3.org/2000/svg"><g><g/></g></svg>`, options: svghush.Options{MaxDepth: 2}},
		{name: "removed depth", input: `<svg xmlns="http://www.w3.org/2000/svg"><script><g/></script></svg>`, options: svghush.Options{MaxDepth: 2}},
		{name: "long name", input: `<svg xmlns="http://www.w3.org/2000/svg"><` + strings.Repeat("a", nameByteLimit+1) + `/></svg>`},
		{name: "many elements", input: `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(`<g/>`, elementLimit) + `</svg>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			output, err := svghush.SanitizeWithOptions([]byte(test.input), test.options)
			if !errors.Is(err, svghush.ErrLimitExceeded) || output != nil {
				t.Errorf("svghush.SanitizeWithOptions(%s, %+v) = (%q, %v), want (nil, svghush.ErrLimitExceeded)", test.name, test.options, output, err)
			}
		})
	}
	t.Run("attribute count", func(t *testing.T) {
		t.Parallel()
		var input strings.Builder
		input.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"`)
		for index := range attributeLimit {
			input.WriteString(` a` + strconv.Itoa(index) + `="x"`)
		}
		input.WriteString(`/>`)
		output, err := svghush.Sanitize([]byte(input.String()))
		if !errors.Is(err, svghush.ErrLimitExceeded) || output != nil {
			t.Errorf("svghush.Sanitize(too many attributes) = (%q, %v), want (nil, svghush.ErrLimitExceeded)", output, err)
		}
	})
	t.Run("namespace scope count", func(t *testing.T) {
		t.Parallel()
		var input strings.Builder
		input.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"`)
		for index := range attributeLimit - 2 {
			input.WriteString(` xmlns:p` + strconv.Itoa(index) + `="urn:example"`)
		}
		input.WriteString(`><g xmlns:extra="urn:extra"/></svg>`)
		output, err := svghush.Sanitize([]byte(input.String()))
		if !errors.Is(err, svghush.ErrLimitExceeded) || output != nil {
			t.Errorf("svghush.Sanitize(too many in-scope namespaces) = (%q, %v), want (nil, svghush.ErrLimitExceeded)", output, err)
		}
	})
	for _, options := range []svghush.Options{{MaxDepth: -1}, {MaxInputBytes: -1}, {MaxOutputBytes: -1}} {
		output, err := svghush.SanitizeWithOptions([]byte(minimal), options)
		if !errors.Is(err, svghush.ErrInvalidOptions) || output != nil {
			t.Errorf("svghush.SanitizeWithOptions(%q, %+v) = (%q, %v), want (nil, svghush.ErrInvalidOptions)", minimal, options, output, err)
		}
	}
	if _, err := svghush.SanitizeWithOptions([]byte(minimal), svghush.Options{MaxInputBytes: len(minimal), MaxDepth: 1}); err != nil {
		t.Errorf("svghush.SanitizeWithOptions(input at exact limit) error = %v, want nil", err)
	}
}

func TestSanitizeUTF8Document(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{"", `<?xml version="1.0"?>`, `<?xml version='1.0' encoding='utf-8' standalone='yes'?>`} {
		for _, bom := range []string{"", "\xef\xbb\xbf"} {
			input := bom + declaration + `<svg xmlns="http://www.w3.org/2000/svg"><text>你好 🌍</text></svg>`
			output, err := svghush.Sanitize([]byte(input))
			if err != nil || !bytes.Contains(output, []byte("你好 🌍")) {
				t.Errorf("svghush.Sanitize(%q) = (%q, %v), want preserved UTF-8 text", input, output, err)
			}
		}
	}
}

func TestSanitizeOwnsOutput(t *testing.T) {
	t.Parallel()
	input := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	original := bytes.Clone(input)
	output, err := svghush.Sanitize(input)
	if err != nil {
		t.Fatal(err)
	}
	output[0] = '!'
	if !bytes.Equal(input, original) {
		t.Errorf("svghush.Sanitize input after output mutation = %q, want %q", input, original)
	}
}

func TestUpstreamFixture(t *testing.T) {
	t.Parallel()
	input, err := os.ReadFile("testdata/upstream/input.svg")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/upstream/expected.svg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svghush.Sanitize(input); !errors.Is(err, svghush.ErrInvalidSVG) {
		t.Fatalf("svghush.Sanitize(upstream fixture with DOCTYPE) error = %v, want svghush.ErrInvalidSVG", err)
	}
	// The port deliberately rejects DOCTYPE instead of relying on a DTD parser.
	start := bytes.Index(input, []byte("<!DOCTYPE"))
	end := bytes.IndexByte(input[start:], '>') + start + 1
	input = append(bytes.Clone(input[:start]), input[end:]...)
	output, err := svghush.SanitizeWithOptions(input, svghush.Options{AllowDataImages: true})
	if err != nil {
		t.Fatalf("svghush.SanitizeWithOptions(upstream fixture) error = %v, want nil", err)
	}
	// Raster data URLs are canonicalized to base64 in this port.
	expected = bytes.ReplaceAll(expected, []byte("data:image/gif,GIF89a"), []byte("data:image/gif;base64,R0lGODlh"))
	// The syntax gate drops these two malformed, quoted/backslash CSS values
	// instead of retaining upstream's partially rewritten CSS tail.
	expected = bytes.ReplaceAll(expected, []byte(`fill="url()#p7&apos;)"`), []byte(`fill=""`))
	expected = bytes.ReplaceAll(expected, []byte(`fill="&apos;//backslash.invalid/url.svg()#p8&apos;)"`), []byte(`fill=""`))
	if gotTokens, wantTokens := xmlTokens(t, output), xmlTokens(t, expected); !reflect.DeepEqual(gotTokens, wantTokens) {
		t.Errorf("svghush.SanitizeWithOptions(upstream fixture) =\n%s\nwant XML equivalent to testdata/upstream/expected.svg (documented differences applied)", output)
	}
}

func xmlTokens(t testing.TB, document []byte) []xml.Token {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(document))
	var tokens []xml.Token
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return tokens
		}
		if err != nil {
			t.Fatalf("decode XML %q: %v", document, err)
		}
		switch token := xml.CopyToken(token).(type) {
		case xml.ProcInst, xml.Comment:
			continue
		case xml.CharData:
			trimmed := bytes.TrimSpace(token)
			if len(trimmed) != 0 {
				tokens = append(tokens, xml.CharData(trimmed))
			}
		case xml.StartElement:
			sort.Slice(token.Attr, func(i, j int) bool { return token.Attr[i].Name.Local < token.Attr[j].Name.Local })
			tokens = append(tokens, token)
		default:
			tokens = append(tokens, token)
		}
	}
}

func assertSafeOutput(t testing.TB, output []byte) {
	t.Helper()
	for _, token := range xmlTokens(t, output) {
		element, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if element.Name.Space != "http://www.w3.org/2000/svg" {
			t.Fatalf("svghush.Sanitize output element namespace = %q, want SVG namespace", element.Name.Space)
		}
		switch strings.ToLower(element.Name.Local) {
		case "script", "foreignobject", "iframe", "object", "embed":
			t.Fatalf("svghush.Sanitize output contains forbidden element %q", element.Name.Local)
		}
		for _, attribute := range element.Attr {
			if strings.HasPrefix(strings.ToLower(attribute.Name.Local), "on") {
				t.Fatalf("svghush.Sanitize output contains event handler %q", attribute.Name.Local)
			}
		}
	}
}
