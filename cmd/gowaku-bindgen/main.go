// Command gowaku-bindgen writes Go bindings of Wayland protocols for gowaku,
// without cgo.
//
// It reads protocol XML files and writes one Go file with, for each interface:
// its wl_interface table, a proxy type with a method per request, a listener
// interface with a method per event, a no-op implementation of it to embed, and
// the constants of its enums. Requests go through wl_proxy_marshal_array_flags
// and events come back through the one dispatcher of platform/wayland, so the
// bindings take no purego callback and no unsafe of their own.
//
// Usage:
//
//	gowaku-bindgen [flags] -out file_linux.go protocol.xml...
//
// The flags:
//
//	-out file      the Go file to write
//	-pkg name      its package; by default the name of its directory
//	-runtime path  the package the output calls, by default
//	               github.com/bankai-tensei/gowaku/platform/wayland; empty when
//	               the output belongs to that package itself
//	-table name    the array of wl_interface tables, by default interfaces
//	-tags expr     the build constraint of the output, by default linux
//	-skip a,b      interfaces to leave out
//
// With the default runtime, a proxy is created with Platform.Bind on a global of
// Platform.Globals, requests take Platform.Conn, and listenXxx(conn, proxy,
// listener) sends the events of a proxy to a listener, which is called inside
// Platform.WaitEvent on the UI goroutine. Every interface that a new_id of the
// input names must be in the input too. Interfaces of other protocols, such as
// wl_surface, become wayland.Proxy; the wl_surface of a window is
// wayland.Proxy(h.Surface) for its platform.WaylandHandle h.
//
// See cmd/gowaku-bindgen/README.md for an example, and
// docs/ADR/0021-wayland-platform.md and docs/ADR/0077-wayland-bindgen.md for
// the design.
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// DefaultRuntime is the package that generated bindings call by default.
const DefaultRuntime = "github.com/bankai-tensei/gowaku/platform/wayland"

func main() {
	out := flag.String("out", "", "Go file to write")
	pkg := flag.String("pkg", "", "package of the output; by default the name of its directory")
	runtime := flag.String("runtime", DefaultRuntime, "package the output calls; empty inside that package")
	skip := flag.String("skip", "", "comma-separated interfaces to leave out")
	table := flag.String("table", "interfaces", "name of the array of wl_interface tables")
	tags := flag.String("tags", "linux", "build constraint of the output")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "usage: gowaku-bindgen [flags] -out file.go protocol.xml...") //nolint:errcheck // nothing is left to do if the usage cannot be written
		flag.PrintDefaults()
	}
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*out, Options{Package: *pkg, Table: *table, Tags: *tags, Runtime: *runtime}, flag.Args(), *skip); err != nil {
		fmt.Fprintln(os.Stderr, "gowaku-bindgen:", err)
		os.Exit(1)
	}
}

func run(out string, opts Options, names []string, skip string) error {
	if opts.Package == "" {
		abs, err := filepath.Abs(out)
		if err != nil {
			return err
		}
		opts.Package = filepath.Base(filepath.Dir(abs))
		if !token.IsIdentifier(opts.Package) {
			return fmt.Errorf("directory %q is not a package name: use -pkg", opts.Package)
		}
	}
	protocols := make([]*Protocol, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		p, err := Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		protocols = append(protocols, p)
	}
	src, err := Generate(opts, protocols, strings.Split(skip, ","))
	if err != nil {
		return err
	}
	return os.WriteFile(out, src, 0o644)
}

// Protocol is one XML file.
type Protocol struct {
	Name       string      `xml:"name,attr"`
	Interfaces []Interface `xml:"interface"`
}

// Interface is a Wayland interface.
type Interface struct {
	Name        string      `xml:"name,attr"`
	Version     int         `xml:"version,attr"`
	Description Description `xml:"description"`
	Requests    []Message   `xml:"request"`
	Events      []Message   `xml:"event"`
	Enums       []Enum      `xml:"enum"`
}

// Message is a request or an event.
type Message struct {
	Name        string      `xml:"name,attr"`
	Type        string      `xml:"type,attr"`
	Since       int         `xml:"since,attr"`
	Description Description `xml:"description"`
	Args        []Arg       `xml:"arg"`
}

// Arg is an argument of a message.
type Arg struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	Interface string `xml:"interface,attr"`
	AllowNull bool   `xml:"allow-null,attr"`
	Enum      string `xml:"enum,attr"`
}

// Enum is a set of named values.
type Enum struct {
	Name     string  `xml:"name,attr"`
	Bitfield bool    `xml:"bitfield,attr"`
	Entries  []Entry `xml:"entry"`
}

// Entry is a value of an enum.
type Entry struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Description holds the summary of an element.
type Description struct {
	Summary string `xml:"summary,attr"`
}

// Parse reads a protocol file.
func Parse(data []byte) (*Protocol, error) {
	var p Protocol
	if err := xml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("not a protocol")
	}
	return &p, nil
}

// Signature returns the signature string of libwayland for m: the version it
// arrived in, then a letter per argument, with ? before those that can be null.
// A new_id without an interface stands for the three arguments of
// wl_registry.bind: interface name, version and id.
func Signature(m *Message) string {
	var b strings.Builder
	if m.Since > 1 {
		fmt.Fprint(&b, m.Since)
	}
	for _, a := range m.Args {
		if a.AllowNull {
			b.WriteByte('?')
		}
		switch a.Type {
		case "int":
			b.WriteByte('i')
		case "uint":
			b.WriteByte('u')
		case "fixed":
			b.WriteByte('f')
		case "string":
			b.WriteByte('s')
		case "object":
			b.WriteByte('o')
		case "new_id":
			if a.Interface == "" {
				b.WriteString("su")
			}
			b.WriteByte('n')
		case "array":
			b.WriteByte('a')
		case "fd":
			b.WriteByte('h')
		}
	}
	return b.String()
}

// types returns the interface of each argument of the signature of m, "" where
// there is none.
func types(m *Message) []string {
	var t []string
	for _, a := range m.Args {
		if a.Type == "new_id" && a.Interface == "" {
			t = append(t, "", "")
		}
		t = append(t, a.Interface)
	}
	return t
}

// goName turns a snake_case name into camelCase, with an upper first letter if
// exported.
func goName(s string, upper bool) string {
	var b strings.Builder
	for i, part := range strings.Split(s, "_") {
		if part == "" {
			continue
		}
		if i == 0 && !upper {
			b.WriteString(part)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	name := b.String()
	if name != "" && name[0] >= '0' && name[0] <= '9' {
		name = "N" + name
	}
	return name
}

// keywords are the names of arguments that Go reserves.
var keywords = map[string]bool{"interface": true, "type": true, "func": true, "map": true, "range": true, "default": true}

func argName(s string) string {
	n := goName(s, false)
	if keywords[n] {
		return n + "_"
	}
	return n
}

// runtimeAlias is the name under which the output imports the runtime.
const runtimeAlias = "wl"

type generator struct {
	b     bytes.Buffer
	known map[string]bool
	index map[string]int
	// ext says that the output calls the exported runtime of another package
	// instead of the unexported one of its own.
	ext bool
}

func (g *generator) p(line string, args ...any) {
	fmt.Fprintf(&g.b, line, args...)
	g.b.WriteByte('\n')
}

// Options are what the output looks like.
type Options struct {
	// Package is the package of the output, Table the name of its array of
	// wl_interface tables, and Tags its build constraint.
	Package, Table, Tags string
	// Runtime is the import path of the package that the output calls, empty
	// when the output belongs to it.
	Runtime string
}

// Generate writes the bindings of protocols.
func Generate(opts Options, protocols []*Protocol, skip []string) ([]byte, error) {
	g := &generator{known: map[string]bool{}, index: map[string]int{}, ext: opts.Runtime != ""}
	var all []*Interface
	for _, p := range protocols {
		for i := range p.Interfaces {
			in := &p.Interfaces[i]
			if slices.Contains(skip, in.Name) {
				continue
			}
			g.index[in.Name] = len(all)
			g.known[in.Name] = true
			all = append(all, in)
		}
	}
	if err := g.check(all); err != nil {
		return nil, err
	}
	g.p("// Code generated by gowaku-bindgen; DO NOT EDIT.")
	g.p("")
	g.p("//go:build %s", opts.Tags)
	g.p("")
	g.p("package %s", opts.Package)
	g.p("")
	if g.ext {
		g.p("import %s %q", runtimeAlias, opts.Runtime)
		g.p("")
	}
	g.p("// %s holds the wl_interface of every interface below, in this order.", opts.Table)
	g.p("var %s [%d]%s", opts.Table, len(all), g.ifaceType())
	g.p("")
	for _, in := range all {
		g.p("// %sInterface is the wl_interface of %s.", goName(in.Name, false), in.Name)
		g.p("var %sInterface = &%s[%d]", goName(in.Name, false), opts.Table, g.index[in.Name])
	}
	g.p("")
	g.p("func init() {")
	for _, in := range all {
		g.table(in)
	}
	g.p("}")
	for _, in := range all {
		g.iface(in)
	}
	return format.Source(g.b.Bytes())
}

// check rejects a new_id of an interface that is not generated: libwayland
// needs its table to make the proxy.
func (g *generator) check(all []*Interface) error {
	for _, in := range all {
		for _, ms := range [][]Message{in.Requests, in.Events} {
			for _, m := range ms {
				for _, a := range m.Args {
					if a.Type == "new_id" && a.Interface != "" && !g.known[a.Interface] {
						return fmt.Errorf("%s.%s makes a %s, which is not generated: add the protocol that defines it", in.Name, m.Name, a.Interface)
					}
				}
			}
		}
	}
	return nil
}

// The names of the runtime in the output.

func (g *generator) proxyType() string {
	if g.ext {
		return runtimeAlias + ".Proxy"
	}
	return "proxy"
}

func (g *generator) ifaceType() string {
	if g.ext {
		return runtimeAlias + ".Interface"
	}
	return "wlInterface"
}

func (g *generator) connParam() string {
	if g.ext {
		return "c *" + runtimeAlias + ".Conn"
	}
	return "c *conn"
}

// table fills the wl_interface of in.
func (g *generator) table(in *Interface) {
	tableOf := func(ms []Message) string {
		if len(ms) == 0 {
			return "nil"
		}
		var b strings.Builder
		if g.ext {
			fmt.Fprintf(&b, "[]%s.Message{\n", runtimeAlias)
		} else {
			fmt.Fprintf(&b, "&[]wlMessage{\n")
		}
		for i := range ms {
			m := &ms[i]
			t := types(m)
			var parts []string
			for _, name := range t {
				if g.known[name] {
					parts = append(parts, goName(name, false)+"Interface")
				} else {
					// Not generated, or no interface: libwayland needs the
					// interface only to create the proxy of a new_id.
					parts = append(parts, "nil")
				}
			}
			if g.ext {
				typ := ""
				if len(parts) > 0 {
					typ = fmt.Sprintf(", Types: []*%s.Interface{%s}", runtimeAlias, strings.Join(parts, ", "))
				}
				fmt.Fprintf(&b, "{Name: %q, Signature: %q%s},\n", m.Name, Signature(m), typ)
				continue
			}
			typ := "nil"
			if len(parts) > 0 {
				typ = fmt.Sprintf("&[]*wlInterface{%s}[0]", strings.Join(parts, ", "))
			}
			fmt.Fprintf(&b, "{cstr(%q), cstr(%q), %s},\n", m.Name+"\x00", Signature(m)+"\x00", typ)
		}
		if g.ext {
			fmt.Fprintf(&b, "}")
		} else {
			fmt.Fprintf(&b, "}[0]")
		}
		return b.String()
	}
	if g.ext {
		g.p("%sInterface.Set(%q, %d, %s, %s)", goName(in.Name, false), in.Name, in.Version,
			tableOf(in.Requests), tableOf(in.Events))
		return
	}
	g.p("%sInterface.set(%q, %d, %d, %s, %d, %s)", goName(in.Name, false), in.Name+"\x00", in.Version,
		len(in.Requests), tableOf(in.Requests), len(in.Events), tableOf(in.Events))
}

// iface writes the proxy, listener and constants of in.
func (g *generator) iface(in *Interface) {
	typ := goName(in.Name, false)
	g.p("")
	summary := in.Description.Summary
	if summary == "" {
		summary = in.Name
	}
	g.p("// %s is a proxy of %s: %s.", typ, in.Name, oneLine(summary))
	g.p("type %s %s", typ, g.proxyType())
	g.p("")
	g.p("// %sVersion is the newest version of %s that the bindings know.", typ, in.Name)
	g.p("const %sVersion = %d", typ, in.Version)
	for i := range in.Requests {
		g.request(in, i)
	}
	g.listener(in)
	for _, e := range in.Enums {
		g.enum(in, &e)
	}
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// goType returns the Go type of an argument.
func (g *generator) goType(a *Arg) string {
	switch a.Type {
	case "int", "fd":
		return "int32"
	case "uint":
		return "uint32"
	case "fixed":
		return "float64"
	case "string":
		return "string"
	case "array":
		return "[]byte"
	case "object", "new_id":
		if a.Interface != "" && g.known[a.Interface] {
			return goName(a.Interface, false)
		}
		return g.proxyType()
	}
	panic("gowaku-bindgen: unknown type " + a.Type)
}

// setArg writes the statement that stores the argument a, whose Go value is v,
// as argument n of the request.
func (g *generator) setArg(n int, a *Arg, v string) {
	if g.ext {
		switch a.Type {
		case "int", "fd":
			g.p("c.SetInt(%d, %s)", n, v)
		case "uint":
			g.p("c.SetUint(%d, %s)", n, v)
		case "fixed":
			g.p("c.SetFixed(%d, %s)", n, v)
		case "string":
			g.p("c.SetString(%d, %s)", n, v)
		case "array":
			g.p("c.SetArray(%d, %s)", n, v)
		case "object":
			if g.goType(a) != g.proxyType() {
				v = runtimeAlias + ".Proxy(" + v + ")"
			}
			g.p("c.SetObject(%d, %s)", n, v)
		case "new_id":
			g.p("c.SetNewID(%d)", n)
		}
		return
	}
	switch a.Type {
	case "int", "fd":
		g.p("c.args[%d] = uint64(uint32(%s))", n, v)
	case "uint":
		g.p("c.args[%d] = uint64(%s)", n, v)
	case "fixed":
		g.p("c.args[%d] = uint64(uint32(toFixed(%s)))", n, v)
	case "string":
		g.p("c.args[%d] = c.str(%d, %s)", n, n, v)
	case "array":
		g.p("c.args[%d] = c.array(%d, %s)", n, n, v)
	case "object":
		g.p("c.args[%d] = uint64(%s)", n, v)
	case "new_id":
		g.p("c.args[%d] = 0", n)
	}
}

func (g *generator) request(in *Interface, i int) {
	m := &in.Requests[i]
	typ := goName(in.Name, false)
	var params []string
	var newID *Arg
	for j := range m.Args {
		a := &m.Args[j]
		if a.Type == "new_id" {
			newID = a
			if a.Interface == "" {
				params = append(params, "iface *"+g.ifaceType(), "version uint32")
			}
			continue
		}
		params = append(params, argName(a.Name)+" "+g.goType(a))
	}
	self := "proxy(o)"
	if g.ext {
		self = runtimeAlias + ".Proxy(o)"
	}
	result := ""
	ret := ""
	ifacePtr, version := "nil", "0"
	if newID != nil {
		if newID.Interface == "" {
			result, ifacePtr, version = g.proxyType(), "iface", "version"
			ret = "return "
		} else {
			result = g.goType(newID)
			ifacePtr = goName(newID.Interface, false) + "Interface"
			if g.ext {
				version = "c.Version(" + self + ")"
			} else {
				version = "c.version(proxy(o))"
			}
			ret = "return " + result + "("
		}
	}
	summary := m.Description.Summary
	if summary == "" {
		summary = m.Name
	}
	name := goName(m.Name, false)
	if m.Since > 1 {
		g.p("")
		g.p("// %s%sSince is the version of %s that added %s.", typ, goName(m.Name, true), in.Name, m.Name)
		g.p("const %s%sSince = %d", typ, goName(m.Name, true), m.Since)
	}
	g.p("")
	g.p("// %s sends %s.%s: %s.", name, in.Name, m.Name, oneLine(summary))
	g.p("func (o %s) %s(%s) %s {", typ, name, strings.Join(append([]string{g.connParam()}, params...), ", "), result)
	n := 0
	for j := range m.Args {
		a := &m.Args[j]
		if a.Type == "new_id" && a.Interface == "" {
			if g.ext {
				g.p("c.SetInterface(%d, iface)", n)
				n++
				g.p("c.SetUint(%d, version)", n)
			} else {
				g.p("c.args[%d] = iface.nameArg()", n)
				n++
				g.p("c.args[%d] = uint64(version)", n)
			}
			n++
		}
		g.setArg(n, a, argName(a.Name))
		n++
	}
	flags := "0"
	if m.Type == "destructor" {
		flags = "marshalDestroy"
		if g.ext {
			flags = runtimeAlias + ".MarshalDestroy"
		}
	}
	call := fmt.Sprintf("c.marshal(proxy(o), %d, %s, %s, %s)", i, ifacePtr, version, flags)
	if g.ext {
		call = fmt.Sprintf("c.Marshal(%s, %d, %s, %s, %s)", self, i, ifacePtr, version, flags)
	}
	switch {
	case newID == nil:
		g.p("%s", call)
	case newID.Interface == "":
		g.p("%s%s", ret, call)
	default:
		g.p("%s%s)", ret, call)
	}
	g.p("}")
}

func (g *generator) listener(in *Interface) {
	typ := goName(in.Name, false)
	if len(in.Events) == 0 {
		return
	}
	for i := range in.Events {
		m := &in.Events[i]
		if m.Since <= 1 {
			continue
		}
		if slices.ContainsFunc(in.Requests, func(r Message) bool { return r.Name == m.Name }) {
			panic("gowaku-bindgen: request and event share the name " + in.Name + "." + m.Name)
		}
		g.p("")
		g.p("// %s%sSince is the version of %s that added the event %s.", typ, goName(m.Name, true), in.Name, m.Name)
		g.p("const %s%sSince = %d", typ, goName(m.Name, true), m.Since)
	}
	g.p("")
	g.p("// %sListener receives the events of %s. Embed %sNop to take only some.", typ, in.Name, typ)
	g.p("type %sListener interface {", typ)
	for i := range in.Events {
		m := &in.Events[i]
		summary := m.Description.Summary
		if summary == "" {
			summary = m.Name
		}
		g.p("// %s%s is %s.%s: %s.", typ, goName(m.Name, true), in.Name, m.Name, oneLine(summary))
		g.p("%s%s(%s)", typ, goName(m.Name, true), g.eventParams(m))
	}
	g.p("}")
	g.p("")
	g.p("// %sNop ignores every event of %s.", typ, in.Name)
	g.p("type %sNop struct{}", typ)
	for i := range in.Events {
		m := &in.Events[i]
		g.p("")
		g.p("// %s%s implements %sListener.", typ, goName(m.Name, true), typ)
		g.p("func (%sNop) %s%s(%s) {}", typ, typ, goName(m.Name, true), g.eventParams(m))
	}
	g.p("")
	g.p("// listen%s sends the events of o to l.", goName(in.Name, true))
	if g.ext {
		g.p("func listen%s(c *%s.Conn, o %s, l %sListener) {", goName(in.Name, true), runtimeAlias, typ, typ)
		g.p("%s.Listen(c, %s.Proxy(o), l, dispatch%s)", runtimeAlias, runtimeAlias, goName(in.Name, true))
		g.p("}")
		g.p("")
		g.p("func dispatch%s(l %sListener, opcode uint32, a *%s.Args) {", goName(in.Name, true), typ, runtimeAlias)
	} else {
		g.p("func (c *conn) listen%s(o %s, l %sListener) {", goName(in.Name, true), typ, typ)
		g.p("c.listen(proxy(o), l, dispatch%s)", goName(in.Name, true))
		g.p("}")
		g.p("")
		g.p("func dispatch%s(listener any, opcode uint32, a *wlArgs) {", goName(in.Name, true))
		g.p("l := listener.(%sListener)", typ)
	}
	g.p("switch opcode {")
	for i := range in.Events {
		m := &in.Events[i]
		var args []string
		for j := range m.Args {
			a := &m.Args[j]
			args = append(args, g.eventArg(j, a))
		}
		g.p("case %d:", i)
		g.p("l.%s%s(%s)", typ, goName(m.Name, true), strings.Join(args, ", "))
	}
	g.p("}")
	g.p("}")
}

// eventArg returns the expression that reads argument j of an event.
func (g *generator) eventArg(j int, a *Arg) string {
	var get string
	switch a.Type {
	case "int", "fd":
		get = "int"
	case "uint":
		get = "uint"
	case "fixed":
		get = "fixed"
	case "string":
		get = "string"
	case "array":
		get = "array"
	case "object", "new_id":
		get = "object"
	}
	if g.ext {
		get = strings.ToUpper(get[:1]) + get[1:]
	}
	v := fmt.Sprintf("a.%s(%d)", get, j)
	if a.Type == "object" || a.Type == "new_id" {
		t := g.goType(a)
		if t == g.proxyType() && g.ext {
			return v
		}
		return t + "(" + v + ")"
	}
	return v
}

func (g *generator) eventParams(m *Message) string {
	params := make([]string, 0, len(m.Args))
	for j := range m.Args {
		a := &m.Args[j]
		params = append(params, argName(a.Name)+" "+g.goType(a))
	}
	return strings.Join(params, ", ")
}

func (g *generator) enum(in *Interface, e *Enum) {
	typ := goName(in.Name, false)
	g.p("")
	g.p("// The values of %s.%s.", in.Name, e.Name)
	g.p("const (")
	for _, entry := range e.Entries {
		g.p("%s%s%s = %s", typ, goName(e.Name, true), goName(entry.Name, true), entry.Value)
	}
	g.p(")")
}
