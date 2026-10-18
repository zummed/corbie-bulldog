package office

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// document reads a file as YAML, and gives the node it holds: nil for a
// file that holds nothing.
func document(contents []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(contents, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	return doc.Content[0], nil
}

// pairs gives the keys and values of a mapping in the order written.
func pairs(node *yaml.Node) func(yield func(*yaml.Node, *yaml.Node) bool) {
	return func(yield func(*yaml.Node, *yaml.Node) bool) {
		if node == nil || node.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if !yield(node.Content[i], node.Content[i+1]) {
				return
			}
		}
	}
}

// twice gives each key written again in a mapping that holds it already,
// anywhere in a node, at its second writing. A mapping read with one of
// its keys written twice would go by the first and never say so.
func twice(node *yaml.Node) []*yaml.Node {
	if node == nil || node.Kind == yaml.AliasNode {
		return nil
	}
	var again []*yaml.Node
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for k := range pairs(node) {
			if seen[k.Value] {
				again = append(again, k)
			}
			seen[k.Value] = true
		}
	}
	for _, c := range node.Content {
		again = append(again, twice(c)...)
	}
	return again
}

// child gives the value of a mapping under a key, or nil.
func child(node *yaml.Node, key string) *yaml.Node {
	for k, v := range pairs(node) {
		if k.Value == key {
			return v
		}
	}
	return nil
}

// text gives a scalar's text, and whether it is text: a scalar that is no
// number, true or false, or null.
func text(node *yaml.Node) (string, bool) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", false
	}
	return node.Value, true
}

// words gives a list of texts.
func words(node *yaml.Node) ([]string, bool) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, false
	}
	var out []string
	for _, item := range node.Content {
		s, ok := text(item)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// describe says what a node is, for a problem to say what was found.
func describe(node *yaml.Node) string {
	switch {
	case node == nil:
		return "nothing"
	case node.Kind == yaml.MappingNode:
		return "a mapping"
	case node.Kind == yaml.SequenceNode:
		return "a list"
	case node.Tag == "!!null":
		return "nothing"
	case node.Tag == "!!str":
		return fmt.Sprintf("the text %q", node.Value)
	}
	return node.Value
}

// inRow writes a mapping in a row, as a team file writes a model:
// { name: sonnet, service: claude-code }.
func inRow(node *yaml.Node) string {
	row := *node
	row.Style, row.HeadComment, row.LineComment, row.FootComment = yaml.FlowStyle, "", "", ""
	var content []*yaml.Node
	for _, c := range row.Content {
		c := *c
		c.HeadComment, c.LineComment, c.FootComment = "", "", ""
		content = append(content, &c)
	}
	row.Content = content
	out, err := yaml.Marshal(&row)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && s != "{}" {
		s = "{ " + strings.TrimSpace(s[1:len(s)-1]) + " }"
	}
	return s
}

// firstKey gives the key the first line of a file's YAML writes, read from
// its text, for a file that cannot be read as YAML.
func firstKey(src []byte) string {
	for line := range strings.Lines(string(src)) {
		line = strings.TrimSpace(line)
		if line == "" || line == "---" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, _ := strings.Cut(line, ":")
		return key
	}
	return ""
}
