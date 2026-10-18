package changes

import (
	"bytes"
	"reflect"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// A Setting is one field of a file set to a value: the keys to it, from
// the top of the file, and the value, as data or as a YAML node.
type Setting struct {
	Path  string   `json:"path"`
	Field []string `json:"field"`
	Value any      `json:"value"`
}

// ErrMarks says a file marks where a document begins or ends, and so is
// not rewritten in part.
var ErrMarks error = Refusal("it marks where a document begins or ends, with --- or ..., so it is not changed in part: change it whole")

// setField gives a YAML file's text with one field set, changing only the
// lines that change: its comments, blank lines, how far it is set in, its
// line endings, how it ends and how each line is set out are as they
// were, and a line written into a mapping set out between braces is spaced
// as the file spaces them. The rewrite is used only when it reads as the
// same value as the file written plainly; otherwise the plain text is
// given, and kept says the layout was not kept.
func setField(src string, field []string, value any) (text string, kept bool, err error) {
	if len(field) == 0 {
		return "", false, refuse("no field is named to set")
	}
	for line := range strings.Lines(src) {
		if mark := strings.TrimRight(line, "\r\n"); mark == "---" || mark == "..." || strings.HasPrefix(mark, "--- ") {
			return "", false, ErrMarks
		}
	}
	crlf := strings.Contains(src, "\r\n")
	lf := strings.ReplaceAll(src, "\r\n", "\n")
	ended := func(text string) string {
		if crlf {
			return strings.ReplaceAll(text, "\n", "\r\n")
		}
		return text
	}
	v, err := valueNode(value)
	if err != nil {
		return "", false, err
	}
	unit := indentUnit(lf)
	// The file written plainly, with the field set.
	doc, root, err := parsed(lf)
	if err != nil {
		return "", false, err
	}
	original := copied(root)
	if err := setNode(root, field, v); err != nil {
		return "", false, err
	}
	var plainBuf bytes.Buffer
	enc := yaml.NewEncoder(&plainBuf)
	enc.SetIndent(unit)
	if err := enc.Encode(doc); err != nil {
		return "", false, err
	}
	plain := plainBuf.String()
	if !strings.HasSuffix(lf, "\n") && lf != "" {
		plain = strings.TrimRight(plain, "\n")
	}
	// The file with only the lines that change changed, from the file as
	// it was read before the field was set in it.
	if out, ok := splice(lf, original, field, v, unit); ok && sameValue(out, plain) {
		return ended(out), true, nil
	}
	return ended(plain), false, nil
}

// parsed reads a file's text as one YAML document whose top is a mapping,
// an empty file being an empty one.
func parsed(text string) (*yaml.Node, *yaml.Node, error) {
	doc := &yaml.Node{}
	if err := yaml.Unmarshal([]byte(text), doc); err != nil {
		return nil, nil, refuse("it is not YAML that can be read: %v", err)
	}
	if len(doc.Content) == 0 {
		doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, refuse("it is not a mapping of keys, so no field of it can be set")
	}
	return doc, root, nil
}

// copied gives a node and everything in it, copied.
func copied(n *yaml.Node) *yaml.Node {
	c := *n
	c.Content = nil
	for _, inner := range n.Content {
		c.Content = append(c.Content, copied(inner))
	}
	return &c
}

// valueNode gives a value as a YAML node.
func valueNode(value any) (*yaml.Node, error) {
	if n, ok := value.(*yaml.Node); ok {
		if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
			return n.Content[0], nil
		}
		return n, nil
	}
	n := &yaml.Node{}
	if err := n.Encode(value); err != nil {
		return nil, refuse("the value cannot be written as YAML: %v", err)
	}
	// A collection given as data is written in a row, on the line of its
	// key.
	return flowed(n), nil
}

// setNode sets a field in a mapping, making the mappings on the way to it.
func setNode(m *yaml.Node, field []string, v *yaml.Node) error {
	for i, key := range field {
		last := i == len(field)-1
		var found *yaml.Node
		for j := 0; j+1 < len(m.Content); j += 2 {
			if m.Content[j].Value == key {
				found = m.Content[j+1]
				if last {
					m.Content[j+1] = v
				}
				break
			}
		}
		switch {
		case last && found != nil:
			return nil
		case last:
			m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
			return nil
		case found == nil || found.Kind != yaml.MappingNode:
			next := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if found == nil {
				m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, next)
			} else {
				*found = *next
			}
			m = next
		default:
			m = found
		}
	}
	return nil
}

// sameValue says whether two texts read as the same value.
func sameValue(a, b string) bool {
	var va, vb any
	if yaml.Unmarshal([]byte(a), &va) != nil || yaml.Unmarshal([]byte(b), &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}

// indentUnit gives how far a file sets in a line inside another: the least
// any line of it is set in, two where none is.
func indentUnit(text string) int {
	unit := 0
	for line := range strings.Lines(text) {
		in := len(line) - len(strings.TrimLeft(line, " "))
		rest := strings.TrimSpace(line)
		if in > 0 && rest != "" && !strings.HasPrefix(rest, "#") && (unit == 0 || in < unit) {
			unit = in
		}
	}
	if unit == 0 {
		return 2
	}
	return unit
}

// A text is a file's lines, with where each begins.
type text struct {
	src    string
	starts []int
}

func newText(src string) *text {
	t := &text{src: src, starts: []int{0}}
	for i, c := range src {
		if c == '\n' {
			t.starts = append(t.starts, i+1)
		}
	}
	return t
}

// at gives the offset of a line and column, as the YAML reader counts
// them from 1, the column in characters.
func (t *text) at(line, column int) int {
	if line < 1 || line > len(t.starts) {
		return -1
	}
	off := t.starts[line-1]
	for c := 1; c < column && off < len(t.src) && t.src[off] != '\n'; c++ {
		_, size := utf8.DecodeRuneInString(t.src[off:])
		off += size
	}
	return off
}

// lineEnd gives the offset just past a line's line ending, or the end.
func (t *text) lineEnd(line int) int {
	if line < len(t.starts) {
		return t.starts[line]
	}
	return len(t.src)
}

// indentOf gives how far a line is set in.
func (t *text) indentOf(line int) int {
	s := t.src[t.starts[line-1]:t.lineEnd(line)]
	return len(s) - len(strings.TrimLeft(s, " "))
}

// end gives the offset just past a node as it is written, or -1 where it
// cannot be told. A block collection ends with its last entry.
func (t *text) end(n *yaml.Node, keyIndent int) int {
	start := t.at(n.Line, n.Column)
	if start < 0 {
		return -1
	}
	switch {
	case n.Kind == yaml.ScalarNode && (n.Style == yaml.LiteralStyle || n.Style == yaml.FoldedStyle):
		last := n.Line
		for l := n.Line + 1; l <= len(t.starts); l++ {
			s := strings.TrimSpace(t.src[t.starts[l-1]:t.lineEnd(l)])
			if s == "" {
				continue
			}
			if t.indentOf(l) <= keyIndent {
				break
			}
			last = l
		}
		e := t.lineEnd(last)
		if e > 0 && t.src[e-1] == '\n' {
			e--
		}
		return e
	case n.Kind == yaml.ScalarNode:
		return start + tokenLength(t.src[start:], n)
	case n.Style&yaml.FlowStyle != 0:
		return matching(t.src, start)
	case len(n.Content) == 0:
		return -1
	}
	return t.end(n.Content[len(n.Content)-1], n.Column-1)
}

// tokenLength gives how long a scalar is as written at the start of a
// text, or nothing when it cannot be told.
func tokenLength(rest string, n *yaml.Node) int {
	switch n.Style {
	case 0, yaml.TaggedStyle:
		if strings.HasPrefix(rest, n.Value) {
			return len(n.Value)
		}
	case yaml.DoubleQuotedStyle:
		for i := 1; i < len(rest) && rest[i] != '\n'; i++ {
			switch rest[i] {
			case '\\':
				i++
			case '"':
				return i + 1
			}
		}
	case yaml.SingleQuotedStyle:
		for i := 1; i < len(rest) && rest[i] != '\n'; i++ {
			if rest[i] == '\'' {
				if i+1 < len(rest) && rest[i+1] == '\'' {
					i++
					continue
				}
				return i + 1
			}
		}
	}
	return 0
}

// matching gives the offset just past the bracket that closes the one at
// start, or -1.
func matching(src string, start int) int {
	depth := 0
	for i := start; i < len(src); i++ {
		switch c := src[i]; c {
		case '"', '\'':
			for i++; i < len(src) && src[i] != c; i++ {
				if c == '"' && src[i] == '\\' {
					i++
				}
			}
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// splice sets a field in a file's text by changing only what changes: the
// value in its place, or a new line or entry where the mapping holding it
// ends. It says whether it could.
func splice(src string, root *yaml.Node, field []string, v *yaml.Node, unit int) (string, bool) {
	t := newText(src)
	spaced := strings.Contains(src, "{ ")
	m := root
	for i, key := range field {
		k, val := entry(m, key)
		switch {
		case k == nil:
			return t.insert(m, key, nest(field[i+1:], v), unit, spaced)
		case i == len(field)-1 || val.Kind != yaml.MappingNode:
			return t.replace(m, k, val, nest(field[i+1:], v), unit, spaced)
		}
		m = val
	}
	return "", false
}

// entry gives a key of a mapping, and its value.
func entry(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

// nest gives a value under keys, each a mapping set out on lines of its
// own.
func nest(keys []string, v *yaml.Node) *yaml.Node {
	if len(keys) == 0 {
		return v
	}
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: keys[0]}, nest(keys[1:], v)}}
}

// isBlock says whether a value is written on lines of its own.
func isBlock(n *yaml.Node) bool {
	switch {
	case n.Kind == yaml.ScalarNode:
		return n.Style == yaml.LiteralStyle || n.Style == yaml.FoldedStyle
	case n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode:
		return n.Style&yaml.FlowStyle == 0 && len(n.Content) > 0
	}
	return false
}

// replace puts a new value in place of a key's value.
func (t *text) replace(m, k, val, v *yaml.Node, unit int, spaced bool) (string, bool) {
	keyIndent := k.Column - 1
	end := t.end(val, keyIndent)
	if end < 0 {
		return "", false
	}
	start := t.at(val.Line, val.Column)
	flow := m.Style&yaml.FlowStyle != 0
	if isBlock(val) && !flow {
		// From just past the key's colon.
		ks := t.at(k.Line, k.Column)
		if ks < 0 {
			return "", false
		}
		n := tokenLength(t.src[ks:], k)
		if n == 0 {
			return "", false
		}
		colon := strings.IndexByte(t.src[ks+n:], ':')
		if colon < 0 {
			return "", false
		}
		start = ks + n + colon + 1
		if isBlock(v) {
			b, ok := block(v, keyIndent+unit, unit, spaced)
			return t.src[:start] + "\n" + b + t.src[end:], ok
		}
		in, ok := inline(v, spaced)
		return t.src[:start] + " " + in + t.src[end:], ok
	}
	if start < 0 || start > end {
		return "", false
	}
	in, ok := inline(v, spaced)
	return t.src[:start] + in + t.src[end:], ok
}

// insert writes a new key and its value where a mapping ends.
func (t *text) insert(m *yaml.Node, key string, v *yaml.Node, unit int, spaced bool) (string, bool) {
	k, ok := inline(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, false)
	if !ok {
		return "", false
	}
	if m.Style&yaml.FlowStyle != 0 {
		open := t.at(m.Line, m.Column)
		end := matching(t.src, open)
		if open < 0 || end < 0 {
			return "", false
		}
		in, ok := inline(v, spaced)
		pair := k + ": " + in
		closing := end - 1
		inner := t.src[open+1 : closing]
		if strings.TrimSpace(inner) == "" {
			if spaced {
				pair = " " + pair + " "
			}
			return t.src[:open+1] + pair + t.src[closing:], ok
		}
		last := open + 1 + len(strings.TrimRight(inner, " \t\n"))
		return t.src[:last] + ", " + pair + t.src[last:], ok
	}
	indent := 0
	at := len(t.src)
	if len(m.Content) > 0 {
		indent = m.Content[0].Column - 1
		end := t.end(m.Content[len(m.Content)-1], indent)
		if end < 0 {
			return "", false
		}
		at = len(t.src)
		if nl := strings.IndexByte(t.src[end:], '\n'); nl >= 0 {
			at = end + nl + 1
		}
	}
	line := strings.Repeat(" ", indent) + k + ":"
	if isBlock(v) {
		b, ok := block(v, indent+unit, unit, spaced)
		if !ok {
			return "", false
		}
		line += "\n" + b
	} else {
		in, ok := inline(v, spaced)
		if !ok {
			return "", false
		}
		line += " " + in
	}
	before := t.src[:at]
	if before != "" && !strings.HasSuffix(before, "\n") {
		before += "\n"
	}
	after := t.src[at:]
	if after != "" || strings.HasSuffix(t.src, "\n") || t.src == "" {
		line += "\n"
	}
	return before + line + after, true
}

// inline writes a value on one line: a collection in a row, between
// braces or brackets, spaced inside as the file spaces them.
func inline(v *yaml.Node, spaced bool) (string, bool) {
	row := flowed(v)
	out, err := yaml.Marshal(row)
	if err != nil {
		return "", false
	}
	s := strings.TrimSuffix(string(out), "\n")
	if strings.Contains(s, "\n") {
		return "", false
	}
	if spaced {
		s = spaceBraces(s)
	}
	return s, true
}

// flowed gives a copy of a value with every collection in it written in a
// row.
func flowed(v *yaml.Node) *yaml.Node {
	c := *v
	if c.Kind == yaml.MappingNode || c.Kind == yaml.SequenceNode {
		c.Style |= yaml.FlowStyle
	}
	c.HeadComment, c.LineComment, c.FootComment = "", "", ""
	c.Content = nil
	for _, inner := range v.Content {
		c.Content = append(c.Content, flowed(inner))
	}
	return &c
}

// block writes a value on lines of its own, each set in by indent, with
// what is in a row in it spaced as the file spaces its rows.
func block(v *yaml.Node, indent, unit int, spaced bool) (string, bool) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(unit)
	if err := enc.Encode(v); err != nil {
		return "", false
	}
	var lines []string
	for line := range strings.Lines(strings.TrimSuffix(b.String(), "\n")) {
		line = strings.TrimSuffix(line, "\n")
		if spaced {
			line = spaceBraces(line)
		}
		if line != "" {
			line = strings.Repeat(" ", indent) + line
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), true
}

// spaceBraces gives a line with a space inside each brace that opens or
// closes a mapping written in a row. The braces of an expression, ${…},
// and what is quoted are left as they are.
func spaceBraces(line string) string {
	var out strings.Builder
	var open []bool // of each brace open: whether it is a mapping's
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for ; j < len(line) && line[j] != c; j++ {
				if c == '"' && line[j] == '\\' {
					j++
				}
			}
			end := min(j+1, len(line))
			out.WriteString(line[i:end])
			i = end - 1
		case c == '{':
			mapping := i == 0 || line[i-1] != '$'
			open = append(open, mapping)
			out.WriteByte(c)
			if mapping && i+1 < len(line) && line[i+1] != ' ' && line[i+1] != '}' {
				out.WriteByte(' ')
			}
		case c == '}' && len(open) > 0:
			if open[len(open)-1] && line[i-1] != ' ' && line[i-1] != '{' {
				out.WriteByte(' ')
			}
			open = open[:len(open)-1]
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}
