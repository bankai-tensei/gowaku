package paint

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

const (
	xlinkNamespace = "http://www.w3.org/1999/xlink"

	// svgMaxDepth bounds how deep elements nest. Deeper elements are left out
	// and counted, so that hostile input cannot exhaust the stack of the
	// recursive walks over the tree.
	svgMaxDepth = 256
)

// svgNode is an element of the document, or a piece of character data with an
// empty name. ParseSVG reads the whole tree first, because references (use,
// gradients, clip paths) can point forward, and a style element applies to
// every element wherever it stands.
type svgNode struct {
	name string
	// foreign marks an element of another vocabulary, which draws nothing.
	foreign  bool
	attrs    []svgAttr
	children []*svgNode
	parent   *svgNode
	// text is the character data of a node without a name.
	text string
	// decls are the properties that apply to the element, in increasing
	// priority: presentation attributes, rules of style elements by
	// specificity and order, then the style attribute. See svgcss.go.
	decls []svgDecl
}

type svgAttr struct {
	name, value string
}

// svgDecl is one property set on an element.
type svgDecl struct {
	name, value string
}

// attr returns the attribute name of n, or "" if it is absent.
func (n *svgNode) attr(name string) string {
	for i := range n.attrs {
		if n.attrs[i].name == name {
			return n.attrs[i].value
		}
	}
	return ""
}

// has reports whether n has the attribute name.
func (n *svgNode) has(name string) bool {
	for i := range n.attrs {
		if n.attrs[i].name == name {
			return true
		}
	}
	return false
}

// href returns the reference of n: href, or else xlink:href.
func (n *svgNode) href() string {
	if v := n.attr("href"); v != "" {
		return v
	}
	return n.attr("xlink:href")
}

// prop returns the value that the cascade gives the property name on n, and
// whether it is set at all.
func (n *svgNode) prop(name string) (string, bool) {
	v, ok := "", false
	for i := range n.decls {
		if n.decls[i].name == name {
			v, ok = n.decls[i].value, true
		}
	}
	return strings.TrimSpace(v), ok
}

// textContent returns the character data of n and its descendants.
func (n *svgNode) textContent() string {
	var b strings.Builder
	var walk func(*svgNode)
	walk = func(m *svgNode) {
		if m.name == "" {
			b.WriteString(m.text)
		}
		for _, c := range m.children {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// svgTree is the parsed document.
type svgTree struct {
	root *svgNode
	ids  map[string]*svgNode
}

// readSVGTree reads data into a tree. Elements nested deeper than svgMaxDepth
// are dropped and counted in skipped.
func readSVGTree(data []byte) (tree svgTree, skipped int, err error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	tree.ids = make(map[string]*svgNode)
	var cur *svgNode
	depth, drop := 0, 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return tree, skipped, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if drop > 0 {
				drop++
				continue
			}
			if tree.root == nil {
				if t.Name.Local != "svg" || (t.Name.Space != "" && t.Name.Space != svgNamespace) {
					return tree, skipped, errors.New("the root is not an svg element")
				}
			} else if cur == nil {
				// A second root cannot happen in well-formed XML.
				return tree, skipped, errors.New("more than one root element")
			}
			if depth == svgMaxDepth {
				skipped++
				drop = 1
				continue
			}
			n := &svgNode{name: t.Name.Local, parent: cur}
			n.foreign = t.Name.Space != "" && t.Name.Space != svgNamespace
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "":
					n.attrs = append(n.attrs, svgAttr{a.Name.Local, a.Value})
				case a.Name.Local == "href" && (a.Name.Space == xlinkNamespace || a.Name.Space == "xlink"):
					n.attrs = append(n.attrs, svgAttr{"xlink:href", a.Value})
				}
			}
			if cur == nil {
				tree.root = n
			} else {
				cur.children = append(cur.children, n)
			}
			if id := n.attr("id"); id != "" && !n.foreign {
				// The first element with an id wins, as in browsers.
				if _, ok := tree.ids[id]; !ok {
					tree.ids[id] = n
				}
			}
			cur = n
			depth++
		case xml.EndElement:
			if drop > 0 {
				drop--
				continue
			}
			if cur != nil {
				cur = cur.parent
				depth--
			}
		case xml.CharData:
			if cur != nil && drop == 0 && len(t) > 0 {
				cur.children = append(cur.children, &svgNode{parent: cur, text: string(t)})
			}
		}
	}
	if tree.root == nil {
		return tree, skipped, errors.New("no svg element")
	}
	return tree, skipped, nil
}

// lookup returns the element that a reference such as "#id" or "url(#id)"
// names, or nil.
func (t *svgTree) lookup(ref string) *svgNode {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "url(") {
		end := strings.IndexByte(ref, ')')
		if end < 0 {
			return nil
		}
		ref = strings.Trim(strings.TrimSpace(ref[4:end]), `"'`)
	}
	id, ok := strings.CutPrefix(ref, "#")
	if !ok {
		// References to other documents are not followed.
		return nil
	}
	return t.ids[id]
}
