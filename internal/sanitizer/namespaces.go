package sanitizer

import (
	"encoding/xml"
	"fmt"
	"strings"
)

func checkName(name xml.Name) error {
	if len(name.Space)+len(name.Local) > maxNameBytes {
		return fmt.Errorf("%w: xml name", ErrLimitExceeded)
	}
	if name.Local == "" || strings.Contains(name.Space, ":") || strings.Contains(name.Local, ":") {
		return fmt.Errorf("%w: invalid qualified name", ErrInvalidSVG)
	}
	return nil
}

func isNamespaceDeclaration(name xml.Name) bool {
	return name.Space == "xmlns" || (name.Space == "" && name.Local == "xmlns")
}

func resolveNamespaces(element xml.StartElement, parent map[string]string) (map[string]string, error) {
	namespaces := make(map[string]string, len(parent)+len(element.Attr))
	for prefix, namespace := range parent {
		namespaces[prefix] = namespace
	}
	seen := make(map[xml.Name]struct{}, len(element.Attr))
	for _, attribute := range element.Attr {
		if err := checkName(attribute.Name); err != nil {
			return nil, err
		}
		if _, exists := seen[attribute.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate attribute", ErrInvalidSVG)
		}
		seen[attribute.Name] = struct{}{}
		if !isNamespaceDeclaration(attribute.Name) {
			continue
		}
		prefix := attribute.Name.Local
		if attribute.Name.Space == "" {
			prefix = ""
		}
		namespace := attribute.Value
		if prefix == "xmlns" || namespace == xmlnsNamespace ||
			(prefix == "xml" && namespace != xmlNamespace) ||
			(prefix != "xml" && namespace == xmlNamespace) || (prefix != "" && namespace == "") {
			return nil, fmt.Errorf("%w: invalid namespace declaration", ErrInvalidSVG)
		}
		namespaces[prefix] = namespace
	}
	if len(namespaces) > maxAttributes {
		return nil, fmt.Errorf("%w: too many in-scope namespaces", ErrLimitExceeded)
	}
	return namespaces, nil
}

func resolveName(name xml.Name, namespaces map[string]string, isElement bool) (string, error) {
	if name.Space == "" && !isElement {
		return "", nil
	}
	namespace, exists := namespaces[name.Space]
	if name.Space != "" && !exists {
		return "", fmt.Errorf("%w: undeclared namespace prefix", ErrInvalidSVG)
	}
	return namespace, nil
}

func validateAttributes(attributes []xml.Attr, namespaces map[string]string) error {
	seen := make(map[xml.Name]struct{}, len(attributes))
	for _, attribute := range attributes {
		if isNamespaceDeclaration(attribute.Name) {
			continue
		}
		namespace, err := resolveName(attribute.Name, namespaces, false)
		if err != nil {
			return err
		}
		expanded := xml.Name{Space: namespace, Local: attribute.Name.Local}
		if _, exists := seen[expanded]; exists {
			return fmt.Errorf("%w: duplicate expanded attribute name", ErrInvalidSVG)
		}
		seen[expanded] = struct{}{}
	}
	return nil
}
