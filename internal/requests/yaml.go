package requests

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A mapping is a YAML mapping written as a block, its keys in the order
// given.
type mapping []entry

// An inline mapping is a YAML mapping written on one line.
type inline []entry

type entry struct {
	key   string
	value any
}

// A list is a YAML sequence written on one line.
type list []any

func (m mapping) MarshalYAML() (any, error) { return node(m, 0) }

func (m inline) MarshalYAML() (any, error) { return node(m, yaml.FlowStyle) }

func (l list) MarshalYAML() (any, error) {
	n := &yaml.Node{}
	if err := n.Encode([]any(l)); err != nil {
		return nil, err
	}
	n.Style = yaml.FlowStyle
	return n, nil
}

// node writes a mapping's entries in order. A text that holds an
// expression, or that has to be quoted, is written in double quotes, as a
// person writes one.
func node(entries []entry, style yaml.Style) (*yaml.Node, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Style: style}
	for _, e := range entries {
		var key, value yaml.Node
		key.SetString(e.key)
		if err := value.Encode(e.value); err != nil {
			return nil, err
		}
		if value.Kind == yaml.ScalarNode && value.Tag == "!!str" &&
			(value.Style == yaml.SingleQuotedStyle || strings.Contains(value.Value, "${") || style == yaml.FlowStyle && !word.MatchString(value.Value)) {
			value.Style = yaml.DoubleQuotedStyle
		}
		n.Content = append(n.Content, &key, &value)
	}
	return n, nil
}

// word is a text that needs no quotes on one line.
var word = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// encode writes a definition as YAML, indented by two spaces.
func encode(definition mapping) ([]byte, error) {
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(definition); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// literal writes a text as a JavaScript string, in single quotes, for an
// expression.
func literal(text string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range text {
		switch {
		case r == '\\' || r == '\'':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x2028 || r == 0x2029:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// literals writes texts as a JavaScript list.
func literals(texts []string) string {
	written := make([]string, len(texts))
	for i, text := range texts {
		written[i] = literal(text)
	}
	return "[" + strings.Join(written, ", ") + "]"
}
