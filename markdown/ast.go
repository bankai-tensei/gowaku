package markdown

// blockKind says what a block of the document is.
type blockKind uint8

// The kinds of block.
const (
	blockDocument blockKind = iota
	blockParagraph
	blockHeading
	blockCode
	blockQuote
	blockList
	blockItem
	blockRule
	blockTable
	// blockHTML is an HTML block while the blocks are parsed; buildHTML
	// replaces it with the blocks it stands for.
	blockHTML
	// blockDetails is a <details> element: summary is its title, and its
	// children show while it is open.
	blockDetails
)

// align is the alignment of a column of a table.
type align uint8

// The alignments of a column.
const (
	alignNone align = iota
	alignLeft
	alignCenter
	alignRight
)

// block is a node of the block tree. Which fields are used depends on kind.
type block struct {
	kind     blockKind
	parent   *block
	children []*block
	open     bool

	// startLine and endLine are the lines of the source, from 1, that the block
	// covers. They decide whether a list is loose.
	startLine, endLine int

	// content collects the raw text of a paragraph, a heading or a code block
	// while the blocks are parsed.
	content []byte
	// inlines is the parsed content of a paragraph or a heading.
	inlines []inline

	// level is the level of a heading, from 1 to 6, and slug its anchor
	// (heading.go).
	level int
	slug  string
	// textAlign is the align attribute of the HTML a paragraph or a heading
	// came from or sits in.
	textAlign align

	// htmlType is the type of an HTML block, from 1 to 7, by the start
	// conditions of CommonMark. html says that the block was made from
	// HTML, with its inlines already parsed.
	htmlType int
	html     bool
	// summary is the title of a details block, and startsOpen whether it
	// has the open attribute.
	summary    []inline
	startsOpen bool

	// The fields of a code block. info is the info string of a fenced block,
	// whose first word is the language.
	fenced      bool
	fenceChar   byte
	fenceLen    int
	fenceOffset int
	info        string
	// closed says that a fenced block met its closing fence.
	closed bool

	// The fields of a list and of its items.
	ordered      bool
	bulletChar   byte
	delimiter    byte
	start        int
	tight        bool
	markerOffset int
	padding      int
	task         bool
	checked      bool

	// The fields of a table. rows[0] is the header row; every row has as many
	// cells as aligns.
	aligns  []align
	rawRows [][]string
	rows    [][][]inline
	// cells are the cells of a table written in HTML, each a run of
	// blocks, in place of rows; header says that its first row is <th>.
	cells  [][][]*block
	header bool
}

// inlineKind says what a piece of inline content is.
type inlineKind uint8

// The kinds of inline content.
const (
	inlineText inlineKind = iota
	inlineCode
	inlineEmph
	inlineStrong
	inlineStrike
	inlineLink
	inlineImage
	inlineSoftBreak
	inlineHardBreak
	// inlineKbd is a key, as <kbd>; its text is the literal.
	inlineKbd
	inlineSub
	inlineSup
	// inlineTag is a tag of raw HTML while it waits to be paired with its
	// closing tag (pairTags). None is left after parsing.
	inlineTag
)

// inline is a node of the inline tree of a paragraph, a heading or a cell.
type inline struct {
	kind inlineKind
	// text is the literal of text and of code.
	text string
	// dest and title belong to links and images. An autolink has its text as
	// its only child.
	dest, title string
	children    []inline
	// tag is the tag of an inlineTag.
	tag *htmlTag
	// width and height are the size an <img> asks for, in pixels, or zero;
	// widthPercent says that width is a percentage of the width there is.
	width, height float32
	widthPercent  bool
	// sources are the <source> elements of the <picture> around an image.
	sources []pictureSource
}

// colorScheme is a value of prefers-color-scheme.
type colorScheme uint8

// The color schemes. schemeAny is any of them.
const (
	schemeAny colorScheme = iota
	schemeLight
	schemeDark
)

// pictureSource is a <source> of a <picture>: src replaces the image while
// the theme is scheme.
type pictureSource struct {
	scheme colorScheme
	src    string
}

// plainText appends the text of ins, as a screen reader would read it, to dst.
func plainText(dst []byte, ins []inline) []byte {
	for i := range ins {
		in := &ins[i]
		switch in.kind {
		case inlineText, inlineCode, inlineKbd:
			dst = append(dst, in.text...)
		case inlineSoftBreak:
			dst = append(dst, ' ')
		case inlineHardBreak:
			dst = append(dst, '\n')
		default:
			dst = plainText(dst, in.children)
		}
	}
	return dst
}
