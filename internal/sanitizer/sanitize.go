// Copyright 2022 Cloudflare, Inc.
// Copyright 2026 svg-hush-go contributors.
// SPDX-License-Identifier: MIT

package sanitizer

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	svgNamespace   = "http://www.w3.org/2000/svg"
	xlinkNamespace = "http://www.w3.org/1999/xlink"
	xmlNamespace   = "http://www.w3.org/XML/1998/namespace"
	xmlnsNamespace = "http://www.w3.org/2000/xmlns/"
	maxAttributes  = 200
	maxNameBytes   = 1000
	maxElements    = 100000

	// DefaultMaxInputBytes is the maximum input size when no override is supplied.
	DefaultMaxInputBytes = 5 << 20
	// DefaultMaxOutputBytes limits XML escaping and serialization expansion.
	DefaultMaxOutputBytes = 10 << 20
	// DefaultMaxDepth limits nesting, including elements that will be removed.
	DefaultMaxDepth = 256
)

var (
	// ErrInvalidSVG identifies malformed XML or an unacceptable document root.
	ErrInvalidSVG = errors.New("svghush: invalid svg")
	// ErrLimitExceeded identifies input, output, or XML complexity limit failures.
	ErrLimitExceeded = errors.New("svghush: limit exceeded")
	// ErrInvalidOptions identifies a negative processing limit.
	ErrInvalidOptions = errors.New("svghush: invalid options")

	xmlDeclarationPattern = regexp.MustCompile(`^version[ \t\r\n]*=[ \t\r\n]*(?:"1\.0"|'1\.0')(?:[ \t\r\n]+encoding[ \t\r\n]*=[ \t\r\n]*(?:"(?i:utf-8)"|'(?i:utf-8)'))?(?:[ \t\r\n]+standalone[ \t\r\n]*=[ \t\r\n]*(?:"(?:yes|no)"|'(?:yes|no)'))?[ \t\r\n]*$`)
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

type elementFrame struct {
	name       xml.Name
	namespaces map[string]string
	outputName string
	isSkipped  bool
	isStyle    bool
}

type documentFilter struct {
	options      Options
	frames       []elementFrame
	encoder      *xml.Encoder
	styleText    strings.Builder
	elementCount int
	rootSeen     bool
	rootClosed   bool
	tokenCount   int
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

// Sanitize removes unsupported SVG features using bounded default options.
// The input must contain exactly one svg root in the SVG namespace, encoded as
// UTF-8 XML. DOCTYPE declarations and custom entities are rejected. Output is
// deterministic UTF-8 XML. On failure, the returned byte slice is always nil.
func Sanitize(input []byte) ([]byte, error) {
	return SanitizeWithOptions(input, Options{})
}

// SanitizeWithOptions sanitizes input using the given limits and URL policy.
// Limits apply to removed content too. The function consumes the whole document
// before returning output, so malformed trailing input cannot be accepted.
func SanitizeWithOptions(input []byte, options Options) ([]byte, error) {
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	if len(input) > options.MaxInputBytes {
		return nil, fmt.Errorf("%w: input exceeds %d bytes", ErrLimitExceeded, options.MaxInputBytes)
	}
	input = bytes.TrimPrefix(input, []byte{0xef, 0xbb, 0xbf})
	output := &limitedBuffer{limit: options.MaxOutputBytes}
	filter := documentFilter{options: options, encoder: xml.NewEncoder(output)}
	decoder := xml.NewDecoder(bytes.NewReader(input))
	for {
		token, readErr := decoder.RawToken()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidSVG, readErr)
		}
		if err := filter.process(token); err != nil {
			return nil, err
		}
	}
	if !filter.rootClosed || len(filter.frames) != 0 {
		return nil, fmt.Errorf("%w: missing or incomplete svg root", ErrInvalidSVG)
	}
	if err := filter.encoder.Flush(); err != nil {
		return nil, fmt.Errorf("svghush: encode output: %w", err)
	}
	return output.Bytes(), nil
}

func normalizeOptions(options Options) (Options, error) {
	if options.MaxInputBytes < 0 || options.MaxOutputBytes < 0 || options.MaxDepth < 0 {
		return Options{}, ErrInvalidOptions
	}
	if options.MaxInputBytes == 0 {
		options.MaxInputBytes = DefaultMaxInputBytes
	}
	if options.MaxOutputBytes == 0 {
		options.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = DefaultMaxDepth
	}
	return options, nil
}

func (f *documentFilter) process(token xml.Token) error {
	f.tokenCount++
	switch token := token.(type) {
	case xml.StartElement:
		return f.startElement(token)
	case xml.EndElement:
		return f.endElement(token)
	case xml.CharData:
		if len(f.frames) == 0 {
			if strings.Trim(string(token), " \t\r\n") != "" {
				return fmt.Errorf("%w: text outside svg root", ErrInvalidSVG)
			}
			return nil
		}
		frame := f.frames[len(f.frames)-1]
		if frame.isSkipped {
			return nil
		}
		if frame.isStyle {
			f.styleText.Write(token)
			return nil
		}
		return f.encoder.EncodeToken(token)
	case xml.Directive:
		return fmt.Errorf("%w: document type declarations are not supported", ErrInvalidSVG)
	case xml.ProcInst:
		if strings.EqualFold(token.Target, "xml") &&
			(token.Target != "xml" || f.tokenCount != 1 || !xmlDeclarationPattern.Match(token.Inst)) {
			return fmt.Errorf("%w: invalid xml declaration", ErrInvalidSVG)
		}
		// Stylesheet and other processing instructions never reach the output.
		return nil
	case xml.Comment:
		return nil
	default:
		return fmt.Errorf("%w: unsupported xml token", ErrInvalidSVG)
	}
}

func (f *documentFilter) startElement(element xml.StartElement) error {
	f.elementCount++
	if len(f.frames) >= f.options.MaxDepth || f.elementCount > maxElements || len(element.Attr) > maxAttributes {
		return fmt.Errorf("%w: xml complexity", ErrLimitExceeded)
	}
	if err := checkName(element.Name); err != nil {
		return err
	}
	parentNamespaces := map[string]string{"xml": xmlNamespace}
	var parent elementFrame
	if len(f.frames) > 0 {
		parent = f.frames[len(f.frames)-1]
		parentNamespaces = parent.namespaces
	}
	namespaces, err := resolveNamespaces(element, parentNamespaces)
	if err != nil {
		return err
	}
	namespace, err := resolveName(element.Name, namespaces, true)
	if err != nil {
		return err
	}
	if err := validateAttributes(element.Attr, namespaces); err != nil {
		return err
	}
	if len(f.frames) == 0 {
		if f.rootSeen || element.Name.Local != "svg" || namespace != svgNamespace {
			return fmt.Errorf("%w: expected one svg root in the svg namespace", ErrInvalidSVG)
		}
		f.rootSeen = true
	}
	_, isAllowed := allowedElements[element.Name.Local]
	frame := elementFrame{
		name: element.Name, namespaces: namespaces, outputName: element.Name.Local,
		isSkipped: parent.isSkipped || parent.isStyle || namespace != svgNamespace || (!isAllowed && element.Name.Local != "style"),
		isStyle:   element.Name.Local == "style",
	}
	f.frames = append(f.frames, frame)
	if frame.isSkipped {
		return nil
	}
	attributes := f.filterAttributes(element, namespaces)
	if len(f.frames) == 1 {
		// Never copy namespace URIs from the input into the output. All kept
		// names have been resolved and are serialized in this fixed namespace.
		attributes = append([]xml.Attr{{Name: xml.Name{Local: "xmlns"}, Value: svgNamespace}}, attributes...)
	}
	if frame.isStyle {
		f.styleText.Reset()
	}
	return f.encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: frame.outputName}, Attr: attributes})
}

func (f *documentFilter) endElement(element xml.EndElement) error {
	if len(f.frames) == 0 {
		return fmt.Errorf("%w: unexpected closing element", ErrInvalidSVG)
	}
	frame := f.frames[len(f.frames)-1]
	if frame.name != element.Name {
		return fmt.Errorf("%w: mismatched closing element", ErrInvalidSVG)
	}
	f.frames = f.frames[:len(f.frames)-1]
	if len(f.frames) == 0 {
		f.rootClosed = true
	}
	if frame.isSkipped {
		return nil
	}
	if frame.isStyle {
		filtered := filterCSS(normalizeASCII(f.styleText.String()), f.options)
		if err := f.encoder.EncodeToken(xml.CharData(filtered)); err != nil {
			return err
		}
		f.styleText.Reset()
	}
	return f.encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: frame.outputName}})
}

func (f *documentFilter) filterAttributes(element xml.StartElement, namespaces map[string]string) []xml.Attr {
	attributes := make([]xml.Attr, 0, len(element.Attr))
	seenHref := false
	for _, attribute := range element.Attr {
		if isNamespaceDeclaration(attribute.Name) {
			continue
		}
		namespace, _ := resolveName(attribute.Name, namespaces, false) // Validated before filtering.
		if namespace != "" && (namespace != xlinkNamespace || attribute.Name.Local != "href") {
			continue
		}
		name := attribute.Name.Local
		if name == "href" {
			if seenHref || !mayUseHref(element.Name.Local) {
				continue
			}
			seenHref = true
		}
		kind, isAllowed := attributeTypes[name]
		if !isAllowed {
			switch {
			case strings.HasPrefix(name, "data-"):
				kind = attributeText
			case strings.HasPrefix(name, "aria-"):
				kind = attributeASCII
			default:
				continue
			}
		}
		value := attribute.Value
		if kind != attributeText {
			value = normalizeASCII(value)
		}
		switch kind {
		case attributeName:
			targetKind, exists := attributeTypes[strings.TrimSpace(attribute.Value)]
			if !exists || !isPlainAttribute(targetKind) {
				continue
			}
		case attributeURL:
			filtered, ok := filterURL(value, f.options)
			if !ok {
				continue
			}
			value = filtered
		case attributeURLFunction, attributeStyle:
			value = filterCSS(value, f.options)
		}
		attributes = append(attributes, xml.Attr{Name: xml.Name{Local: name}, Value: value})
	}
	sort.Slice(attributes, func(i, j int) bool { return attributes[i].Name.Local < attributes[j].Name.Local })
	return attributes
}

func isPlainAttribute(kind attributeType) bool {
	return kind == attributeASCII || kind == attributeKeyword || kind == attributeNumber || kind == attributeText
}

func mayUseHref(element string) bool {
	switch element {
	case "radialGradient", "linearGradient", "image", "use", "pattern", "feImage":
		return true
	default:
		return false
	}
}

func normalizeASCII(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimSpace(strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) {
			return ' '
		}
		if character > 0x7f || character == 0 {
			return -1
		}
		return character
	}, value))
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, fmt.Errorf("%w: output exceeds %d bytes", ErrLimitExceeded, b.limit)
	}
	return b.Buffer.Write(data)
}
