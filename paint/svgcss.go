package paint

import (
	"slices"
	"strings"
)

// The style elements of a document are read as a small subset of CSS: rules
// whose selectors are lists of type, class, id and universal selectors,
// compounds of them (rect.a#b), and the descendant and child combinators.
// A selector with anything else, such as an attribute selector, a
// pseudo-class or a sibling combinator, matches nothing, and at-rules such as
// @media and @font-face are ignored with their blocks. Declarations marked
// !important beat the style attribute. See docs/ADR/0112-svg-structure-paint-clip.md.

// svgCompound is a compound selector: every part must match.
type svgCompound struct {
	typ     string // "" or "*" matches any element
	id      string
	classes []string
}

// svgSelector is a complex selector. parts run from the leftmost compound to
// the subject; child[i] says that parts[i] must be the parent of parts[i+1],
// not just an ancestor.
type svgSelector struct {
	parts       []svgCompound
	child       []bool
	specificity int
}

type svgRule struct {
	selectors []svgSelector
	decls     []svgDecl
	important []bool
}

// parseStyleSheet reads the rules of a style element.
func parseStyleSheet(css string) []svgRule {
	css = stripComments(css)
	var rules []svgRule
	for {
		css = strings.TrimSpace(css)
		if css == "" {
			return rules
		}
		if css[0] == '@' {
			css = skipAtRule(css)
			continue
		}
		open := strings.IndexByte(css, '{')
		if open < 0 {
			return rules
		}
		end := matchingBrace(css, open)
		prelude, body := css[:open], css[open+1:end]
		if end < len(css) {
			css = css[end+1:]
		} else {
			css = ""
		}
		var r svgRule
		for sel := range strings.SplitSeq(prelude, ",") {
			if s, ok := parseSelector(sel); ok {
				r.selectors = append(r.selectors, s)
			}
		}
		if len(r.selectors) == 0 {
			continue
		}
		for decl := range strings.SplitSeq(body, ";") {
			name, value, ok := strings.Cut(decl, ":")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			important := false
			if v, ok := strings.CutSuffix(value, "!important"); ok {
				value, important = strings.TrimSpace(v), true
			}
			r.decls = append(r.decls, svgDecl{strings.ToLower(strings.TrimSpace(name)), value})
			r.important = append(r.important, important)
		}
		rules = append(rules, r)
	}
}

func stripComments(css string) string {
	var b strings.Builder
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			b.WriteString(css)
			return b.String()
		}
		b.WriteString(css[:i])
		j := strings.Index(css[i+2:], "*/")
		if j < 0 {
			return b.String()
		}
		css = css[i+2+j+2:]
	}
}

// matchingBrace returns the index of the brace that closes the one at open, or
// len(css) if there is none.
func matchingBrace(css string, open int) int {
	depth := 0
	for i := open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(css)
}

// skipAtRule returns what follows the at-rule at the start of css: up to its
// semicolon, or past its block.
func skipAtRule(css string) string {
	semi := strings.IndexByte(css, ';')
	open := strings.IndexByte(css, '{')
	if open < 0 || (semi >= 0 && semi < open) {
		if semi < 0 {
			return ""
		}
		return css[semi+1:]
	}
	end := matchingBrace(css, open)
	if end >= len(css) {
		return ""
	}
	return css[end+1:]
}

// parseSelector reads one complex selector, and reports false for one outside
// the subset.
func parseSelector(s string) (svgSelector, bool) {
	var sel svgSelector
	s = strings.TrimSpace(s)
	if s == "" {
		return sel, false
	}
	pendingChild := false
	for s != "" {
		if s[0] == '>' {
			if len(sel.parts) == 0 || pendingChild {
				return sel, false
			}
			pendingChild = true
			s = strings.TrimSpace(s[1:])
			continue
		}
		end := 0
		for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '\n' && s[end] != '\r' && s[end] != '>' {
			end++
		}
		c, ok := parseCompound(s[:end])
		if !ok {
			return sel, false
		}
		if len(sel.parts) > 0 {
			sel.child = append(sel.child, pendingChild)
		}
		pendingChild = false
		sel.parts = append(sel.parts, c)
		if c.id != "" {
			sel.specificity += 1 << 16
		}
		sel.specificity += len(c.classes) << 8
		if c.typ != "" && c.typ != "*" {
			sel.specificity++
		}
		s = strings.TrimSpace(s[end:])
	}
	if pendingChild {
		return sel, false
	}
	return sel, true
}

func parseCompound(s string) (svgCompound, bool) {
	var c svgCompound
	if s == "" {
		return c, false
	}
	i := 0
	name := func() string {
		start := i
		for i < len(s) && (isNameByte(s[i]) || s[i] >= 0x80) {
			i++
		}
		return s[start:i]
	}
	if s[0] == '*' {
		c.typ = "*"
		i = 1
	} else if isNameByte(s[0]) {
		c.typ = name()
	}
	for i < len(s) {
		switch s[i] {
		case '.':
			i++
			n := name()
			if n == "" {
				return c, false
			}
			c.classes = append(c.classes, n)
		case '#':
			i++
			n := name()
			if n == "" || c.id != "" && c.id != n {
				return c, false
			}
			c.id = n
		default:
			// Attribute selectors, pseudo-classes and sibling combinators.
			return c, false
		}
	}
	return c, true
}

func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '_'
}

func (c *svgCompound) matches(n *svgNode) bool {
	if n.name == "" || n.foreign {
		return false
	}
	if c.typ != "" && c.typ != "*" && c.typ != n.name {
		return false
	}
	if c.id != "" && n.attr("id") != c.id {
		return false
	}
	if len(c.classes) > 0 {
		classes := strings.Fields(n.attr("class"))
		for _, want := range c.classes {
			if !slices.Contains(classes, want) {
				return false
			}
		}
	}
	return true
}

// matches reports whether the selector matches n, by walking up from the
// subject.
func (s *svgSelector) matches(n *svgNode) bool {
	budget := svgMatchBudget
	return s.matchFrom(len(s.parts)-1, n, &budget)
}

// svgMatchBudget bounds the backtracking of one match, which a hostile chain
// of descendant combinators over deep nesting could make exponential. A match
// that runs out fails.
const svgMatchBudget = 4096

func (s *svgSelector) matchFrom(i int, n *svgNode, budget *int) bool {
	*budget--
	if *budget < 0 || !s.parts[i].matches(n) {
		return false
	}
	if i == 0 {
		return true
	}
	if s.child[i-1] {
		return n.parent != nil && s.matchFrom(i-1, n.parent, budget)
	}
	for a := n.parent; a != nil; a = a.parent {
		if s.matchFrom(i-1, a, budget) {
			return true
		}
	}
	return false
}

// svgMatch is a declaration that a rule gives an element, with what orders it.
type svgMatch struct {
	decl        svgDecl
	important   bool
	specificity int
	order       int
}

// cascade fills in the decls of every element of the tree.
func cascade(root *svgNode, rules []svgRule) {
	var matches []svgMatch
	var walk func(n *svgNode)
	walk = func(n *svgNode) {
		if n.name == "" || n.foreign {
			return
		}
		n.decls = n.decls[:0]
		for _, a := range n.attrs {
			switch a.name {
			case "style", "class", "id", "href", "xlink:href":
				continue
			}
			n.decls = append(n.decls, svgDecl(a))
		}
		matches = matches[:0]
		order := 0
		for ri := range rules {
			r := &rules[ri]
			best := -1
			for si := range r.selectors {
				if sp := r.selectors[si].specificity; sp > best && r.selectors[si].matches(n) {
					best = sp
				}
			}
			if best >= 0 {
				for di, d := range r.decls {
					matches = append(matches, svgMatch{d, r.important[di], best, order})
					order++
				}
			} else {
				order += len(r.decls)
			}
		}
		slices.SortStableFunc(matches, func(a, b svgMatch) int {
			if a.specificity != b.specificity {
				return a.specificity - b.specificity
			}
			return a.order - b.order
		})
		for _, m := range matches {
			if !m.important {
				n.decls = append(n.decls, m.decl)
			}
		}
		forEachDeclaration(n.attr("style"), func(name, value string) {
			n.decls = append(n.decls, svgDecl{name, value})
		})
		for _, m := range matches {
			if m.important {
				n.decls = append(n.decls, m.decl)
			}
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)
}

// svgMaxRules bounds the rules read from style elements: every element is
// matched against every rule.
const svgMaxRules = 2048

// styleSheets gathers the rules of every style element of the tree, in
// document order. Only type text/css, or none, is read.
func styleSheets(root *svgNode) []svgRule {
	var rules []svgRule
	var walk func(n *svgNode)
	walk = func(n *svgNode) {
		if n.foreign {
			return
		}
		if n.name == "style" {
			if t := strings.TrimSpace(n.attr("type")); t == "" || t == "text/css" {
				rules = append(rules, parseStyleSheet(n.textContent())...)
			}
			if len(rules) > svgMaxRules {
				rules = rules[:svgMaxRules]
			}
			return
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)
	return rules
}
