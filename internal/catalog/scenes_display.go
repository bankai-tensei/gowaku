package catalog

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// Display.

func sceneBasicText() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.SizedBox{Width: 340, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space6, Children: []core.Widget{
			basic.Text{Content: "Large title", Style: th.Text.LargeTitle},
			basic.Text{Content: "Title", Style: th.Text.Title},
			basic.Text{Content: "Body text breaks into lines by the rules of its style, and the line height stays on an even grid."},
			basic.Text{Content: "Caption in the secondary ink", Style: colored(th.Text.Caption, th.Color.Ink2)},
			// MaxLines cuts the text short with an ellipsis.
			basic.Text{Content: "A one-line preview of a long message that does not fit in the row", Style: th.Text.Body.WithMaxLines(1)},
		}}}
	})
	// catalog:end
}

func sceneBasicRichText() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		c := &th.Color
		return basic.SizedBox{Width: 320, Child: basic.RichText{Style: colored(th.Text.Body, c.Ink).WithMaxLines(2), Spans: []text.Span{
			{Text: "Rich text mixes "},
			{Text: "accented", Style: colored(th.Text.Body, c.AccentInk)},
			{Text: ", "},
			{Text: "bold", Style: colored(th.Text.Body, c.Ink).WithWeight(700)},
			{Text: ", "},
			{Text: "large", Style: text.Style{Size: 20, Color: c.Ink}},
			{Text: ", "},
			{Text: "quiet", Style: colored(th.Text.Caption, c.Ink2)},
			{Text: " and "},
			{Text: "highlighted", Background: c.Warning.WithAlpha(0.35)},
			{Text: " runs in one paragraph that wraps, and stops after two lines with an ellipsis."},
		}}}
	})
	// catalog:end
}

func sceneBasicIcon() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		// The stock icons of package icons. Build them once and keep them, as
		// in InitState: each call makes a new path.
		set := []*paint.Path{
			icons.Folder(), icons.File(), icons.Document(), icons.Image(), icons.Search(), icons.Plus(),
			icons.Minus(), icons.Close(), icons.Check(), icons.ChevronLeft(), icons.ChevronRight(), icons.ChevronUp(),
			icons.ChevronDown(), icons.Settings(), icons.Trash(), icons.Edit(), icons.Copy(), icons.Home(),
			icons.Star(), icons.Info(), icons.Warning(), icons.Error(), icons.Menu(), icons.More(),
		}
		stock := make([]core.Widget, len(set))
		for i, p := range set {
			stock[i] = basic.Icon{Path: p, Color: th.Color.Ink2}
		}
		return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space20, Children: []core.Widget{
			basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossCenter, Children: []core.Widget{
				basic.Icon{Path: set[0], Color: th.Color.AccentInk, Label: "Folder"},
				basic.Icon{Path: set[18], Size: 32, Color: th.Color.Danger},
				basic.Icon{Path: set[8], Size: 16, Color: th.Color.Success},
				basic.Icon{Path: circleIcon(), Color: th.Color.Ink2}, // a path of your own
			}},
			basic.SizedBox{Width: 400, Child: basic.Wrap{Spacing: theme.Space16, RunSpacing: theme.Space16, Children: stock}},
		}}
	})
	// catalog:end
}

func sceneBasicImage() core.Widget {
	picture := photo() // paint.NewImage of an image.Image, made once and kept
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
		basic.Image{Source: picture, Width: 160, Label: "Hills under a clear sky"},
		basic.Image{Source: picture, Width: 100, Height: 100, Fit: layout.FitCover, Radius: 50},
		basic.Image{Source: picture, Width: 48, Height: 48, Quality: paint.FilterNearest},
	}}
}

func sceneBasicSVG() core.Widget {
	cloud, err := paint.ParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
<path d="M7 18h10a4 4 0 0 0 .5-7.97A6 6 0 0 0 6.1 9.5 4.5 4.5 0 0 0 7 18z"/></svg>`))
	if err != nil {
		panic(err)
	}
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
			basic.SVG{Data: cloud, Label: "Cloud"},
			basic.SVG{Data: cloud, Size: 36, Color: th.Color.AccentInk},
			basic.SVG{Data: cloud, Size: 56, Color: th.Color.Ink2},
		}}
	})
}

func sceneBasicAnimatedImage() core.Widget {
	movie, err := paint.DecodeAnimation(bytes.NewReader(blinkGIF()))
	if err != nil {
		panic(err)
	}
	return basic.AnimatedImage{Source: movie, Width: 96, Height: 96, Radius: 12, Quality: paint.FilterNearest, Label: "Blinking light"}
}

// blinkGIF encodes a two frame GIF of 12 by 12 pixels.
func blinkGIF() []byte {
	pal := color.Palette{color.RGBA{0x2f, 0x5f, 0xd0, 255}, color.RGBA{0xff, 0xd6, 0x6b, 255}}
	frame := func(lit bool) *image.Paletted {
		m := image.NewPaletted(image.Rect(0, 0, 12, 12), pal)
		for y := 3; y < 9; y++ {
			for x := 3; x < 9; x++ {
				if lit {
					m.SetColorIndex(x, y, 1)
				}
			}
		}
		return m
	}
	var b bytes.Buffer
	g := &gif.GIF{Image: []*image.Paletted{frame(true), frame(false)}, Delay: []int{50, 50}}
	if err := gif.EncodeAll(&b, g); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func sceneControlBadge() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		inbox := basic.Icon{Path: squareIcon(), Color: th.Color.Ink2}
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space24, Cross: layout.CrossCenter, Children: []core.Widget{
			control.Badge{Count: 3, Child: inbox},
			control.Badge{Count: 120, Child: inbox},
			control.Badge{Dot: true, Child: inbox},
			control.Badge{Label: "New"},
			control.Badge{Count: 12, Quiet: true},
		}}
	})
	// catalog:end
}

func sceneControlCard() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossStart, Children: []core.Widget{
		basic.SizedBox{Width: 200, Child: control.Card{Label: "Summary", Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space6, Children: []core.Widget{
				basic.Text{Content: "Summary"},
				basic.Text{Content: "A plain card holds one piece of content."},
			},
		}}},
		basic.SizedBox{Width: 200, Child: control.Card{Label: "Open note", OnPress: func() {}, Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space6, Children: []core.Widget{
				basic.Text{Content: "Pressable"},
				basic.Text{Content: "With OnPress the whole card is a button."},
			},
		}}},
	}}
}

func sceneBasicDivider() core.Widget {
	return basic.SizedBox{Width: 300, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
		basic.Text{Content: "Above the line"},
		basic.Divider{},
		basic.Text{Content: "Indented line below"},
		basic.Divider{Indent: theme.Space16},
		basic.Row{Size: layout.MainMin, Spacing: theme.Space12, Children: []core.Widget{
			basic.Text{Content: "Left"},
			basic.SizedBox{Height: 20, Child: basic.Divider{Axis: layout.Vertical}},
			basic.Text{Content: "Right"},
		}},
	}}}
}

// Navigation.

func sceneControlTabs() core.Widget {
	return basic.SizedBox{Width: 340, Height: 150, Child: control.Tabs{
		Label: "Settings", Selected: 1, OnChange: func(int) {},
		Pages: []control.TabPage{
			{Label: "General"},
			{Label: "Appearance", Child: basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Text{
				Content: "The page of the chosen tab shows below the bar.",
			}}},
			{Label: "Advanced"},
		},
	}}
}

func sceneCollectionSidebar() core.Widget {
	open := func() {}
	return basic.SizedBox{Height: 340, Child: collection.Sidebar{
		Title:  "Library",
		Header: field.Search{Placeholder: "Search", OnChange: func(string) {}},
		Children: []core.Widget{
			collection.SidebarItem{Label: "Home", Icon: icons.Home(), OnPress: open},
			collection.SidebarItem{Label: "Shared", Icon: icons.Folder(), Detail: "With Aoi and Ren, edited today", OnPress: open},
			collection.SidebarItem{Label: "Documents", Icon: icons.Document(), Count: 3, Selected: true, OnPress: open},
			collection.SidebarItem{Label: "Starred", Icon: icons.Star(), Count: 128, OnPress: open},
			collection.SidebarItem{Label: "Pictures", Icon: icons.Image(), OnPress: open},
		},
		Footer: control.Button{Label: "New Folder", Icon: icons.Plus(), Style: control.ButtonQuiet, OnPress: open},
	}}
}

func sceneNavBreadcrumb() core.Widget {
	trail := []nav.BreadcrumbItem{
		{Label: "Home", Icon: icons.Folder()}, {Label: "Documents"}, {Label: "Projects"}, {Label: "gowaku"}, {Label: "widgets"},
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		basic.SizedBox{Width: 480, Child: nav.Breadcrumb{Items: trail, OnSelect: func(int) {}}},
		// Where the trail does not fit, the middle folds into a menu.
		basic.SizedBox{Width: 260, Child: nav.Breadcrumb{Items: trail, OnSelect: func(int) {}}},
	}}
}

func sceneNavSteps() core.Widget {
	labels := []string{"Account", "Shipping", "Payment", "Review"}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space20, Children: []core.Widget{
		nav.Steps{Labels: labels, Current: 2, OnStepPress: func(int) {}},
		// Every step done.
		nav.Steps{Labels: labels[:3], Current: 3},
	}}
}

// noteTitles are the notes of the Navigator scenes.
func noteTitles() []string { return []string{"Groceries", "Trip to Kyoto", "Reading list"} }

// noteList is the first page of the Navigator scenes: a row for each note,
// whose icon is a Hero, that opens the page of the note.
type noteList struct{ core.StatelessBase }

// Build implements core.StatelessWidget.
func (noteList) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	titles := noteTitles()
	rows := make([]core.Widget, 0, 1+len(titles))
	rows = append(rows, basic.Padding{Insets: geom.Insets{Left: theme.Space16, Top: theme.Space12, Bottom: theme.Space4}, Child: basic.Text{
		Content: "Notes", Style: th.Text.Title,
	}})
	for i, title := range titles {
		rows = append(rows, interact.Pressable{Label: title, OnPress: func() { nav.NavigatorOf(ctx).PushNamed("/notes/" + strconv.Itoa(i)) },
			Child: basic.Padding{Insets: geom.Symmetric(theme.Space16, theme.Space6), Child: basic.Row{
				Spacing: theme.Space12, Cross: layout.CrossCenter, Children: []core.Widget{
					nav.Hero{Tag: i, Child: basic.Container{Width: 28, Height: 28, Radius: 7, Color: th.Color.Accent}},
					basic.Text{Content: title},
				},
			}},
		})
	}
	return basic.Container{Color: th.Color.Card, Child: basic.Column{Cross: layout.CrossStretch, Children: rows}}
}

// notePage is the page of one note, with the way back and its icon large.
type notePage struct {
	core.StatelessBase
	index int
}

// Build implements core.StatelessWidget.
func (p notePage) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{Color: th.Color.Card, Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Padding{Insets: geom.All(theme.Space8), Child: basic.Row{Children: []core.Widget{nav.BackButton{Label: "Notes"}}}},
		basic.Padding{Insets: geom.Symmetric(theme.Space16, theme.Space4), Child: basic.Row{
			Spacing: theme.Space12, Cross: layout.CrossCenter, Children: []core.Widget{
				nav.Hero{Tag: p.index, Child: basic.Container{Width: 56, Height: 56, Radius: 14, Color: th.Color.Accent}},
				basic.Text{Content: noteTitles()[p.index], Style: th.Text.LargeTitle},
			},
		}},
		basic.Padding{Insets: geom.Symmetric(theme.Space16, theme.Space8), Child: basic.Text{
			Content: "Alt+Left or the back button returns to the list.", Style: colored(th.Text.Body, th.Color.Ink2),
		}},
	}}}
}

func sceneNavNavigator() core.Widget {
	return basic.SizedBox{Width: 360, Height: 190, Child: nav.Navigator{
		Home: noteList{}, // its rows call NavigatorOf(ctx).PushNamed("/notes/1")
		Routes: []nav.RoutePattern{{Pattern: "/notes/:id", Build: func(p nav.RouteParams) core.Widget {
			i, _ := p.Int("id")
			return notePage{index: i}
		}}},
	}}
}

func sceneNavBackButton() core.Widget {
	// catalog:begin
	// The top bar of a page that a Navigator pushed:
	return basic.Row{Children: []core.Widget{nav.BackButton{Label: "Notes"}}}
	// catalog:end
}

// Feedback and overlays.

func sceneControlProgressBar() core.Widget {
	return basic.SizedBox{Width: 260, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
		control.ProgressBar{Value: 0.62, Label: "Library sync"},
		control.ProgressBar{Value: 0.25, Label: "Upload"},
		control.ProgressBar{Indeterminate: true, Label: "Connecting"},
	}}}
}

func sceneControlProgressRing() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space24, Cross: layout.CrossCenter, Children: []core.Widget{
			control.ProgressRing{Value: 0.32, Label: "Album art"},
			control.ProgressRing{Value: 0.75, Size: 48, Color: th.Color.Success, Label: "Lyrics", Child: basic.Text{
				Content: "75", Style: colored(th.Text.Caption, th.Color.Ink2),
			}},
			control.ProgressRing{Indeterminate: true, Label: "Syncing"},
		}}
	})
	// catalog:end
}

func sceneOverlayTooltip() core.Widget {
	tool := func(label, tip string) core.Widget {
		return overlay.Tooltip{Message: tip, Child: control.Button{Label: label, Style: control.ButtonQuiet, OnPress: func() {}}}
	}
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space4, Children: []core.Widget{
		tool("Bold", "Bold (Ctrl+B)"),
		tool("Italic", "Italic (Ctrl+I)"),
		tool("Link", "Insert a link (Ctrl+K)"),
	}}
}

func sceneOverlayToast() core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		return control.Button{Label: "Move to trash", OnPress: func() {
			overlay.ShowToast(ctx, overlay.Toast{Message: "Moved 3 notes to the trash", Action: "Undo", OnAction: func() {}})
		}}
	}}
}

func sceneOverlayDialog() core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		return control.Button{Label: "Delete", Style: control.ButtonDanger, OnPress: func() {
			overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
				return overlay.Dialog{
					Title:   "Delete this note?",
					Message: "It cannot be undone.",
					Actions: []core.Widget{
						control.Button{Label: "Cancel", OnPress: dismiss},
						control.Button{Label: "Delete", Style: control.ButtonDanger, OnPress: dismiss},
					},
				}
			})
		}}
	}}
}

func sceneOverlaySheet() core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		return control.Button{Label: "Share", OnPress: func() {
			overlay.ShowSheet(ctx, overlay.Sheet{Label: "Share note"}, func(dismiss func()) core.Widget {
				return basic.Column{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
					basic.Text{Content: "Anyone with the link can read this note."},
					basic.Row{Main: layout.MainEnd, Spacing: theme.Space8, Children: []core.Widget{
						control.Button{Label: "Cancel", OnPress: dismiss},
						control.Button{Label: "Copy link", Style: control.ButtonPrimary, OnPress: dismiss},
					}},
				}}
			})
		}}
	}}
}
