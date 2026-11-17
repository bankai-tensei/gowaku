package main

import "html/template"

// words are the fixed words of the site, in each language. Values may hold
// HTML; they are written by hand here and never from a document.
var words = map[string]map[string]string{
	"en": {
		"lang.name":               "English",
		"lang.other":              "日本語",
		"nav.docs":                "Docs",
		"nav.catalog":             "Widgets",
		"nav.github":              "GitHub",
		"nav.guides":              "Guides",
		"nav.reference":           "Reference",
		"nav.project":             "Project",
		"short.docs":              "Overview",
		"short.docs/architecture": "Architecture",
		"short.docs/design":       "Design",
		"short.docs/benchmarks":   "Benchmarks",
		"short.docs/roadmap":      "Roadmap",
		"short.docs/contributing": "Contributing",
		"short.docs/changelog":    "Changelog",
		"short.docs/security":     "Security",
		"short.docs/third-party":  "Third-party notices",
		"nav.overview":            "Overview",
		"nav.theme":               "Switch between light and dark",
		"nav.menu":                "Menu",
		"toc":                     "On this page",
		"prev":                    "Previous",
		"next":                    "Next",
		"edit":                    "Edit this page on GitHub",
		"copy":                    "Copy",
		"copied":                  "Copied",
		"footer.tagline":          "A GPU-native desktop UI library for Go.",
		"footer.license":          "MIT License",
		"footer.built":            "This site is built from the Markdown of the repository.",
		"site.title":              "gowaku — beautiful, fast desktop apps in pure Go",
		"site.desc":               "gowaku is a GPU-native desktop UI library for Go. No cgo, no C compiler, 0% CPU when idle, and Japanese text done properly.",
		"notfound.title":          "This page does not exist.",
		"notfound.body":           "It may have moved when the docs were rearranged.",
		"notfound.home":           "Go to the home page",
		"catalog.title":           "Widget catalog",
		"catalog.lede":            "Every public widget of gowaku, with a picture and the code that drew it.",
		"catalog.search":          "Search widgets",
		"catalog.all":             "All",
		"catalog.none":            "No widget matches.",
		"catalog.count":           "widgets",
		"catalog.note":            "Pictures are drawn on the CPU by gpu/soft with the Go fonts, so the text in them is English. Lower-case helpers in the examples, such as themed and tile, are in internal/catalog/helpers.go.",
		"catalog.package":         "Package",
		"catalog.also":            "Also",
		"catalog.code":            "Code",
		"catalog.nopic":           "It draws nothing of its own, so there is no picture.",

		"hero.title":   "Beautiful, fast desktop apps in pure Go.",
		"hero.lede":    "gowaku draws your whole interface on the GPU, builds into one binary without cgo, and sets Japanese text the way it deserves.",
		"hero.start":   "Get started",
		"hero.catalog": "Browse the widgets",
		"hero.caption": "For example, you can build a screen like this. Every pixel, from the text to the colors of the diff, is drawn by the library.",
		"stat.start":   "to the first frame",
		"stat.idle":    "CPU when nothing moves",
		"stat.anim":    "the CPU of Gio for 100 animations",
		"stat.widgets": "widgets, from buttons to a dock layout",
		"stat.c":       "lines of C to compile",
		"f1.kicker":    "It's just Go",
		"f1.title":     "One <code>go build</code>.<br>Every platform.",
		"f1.body":      "No cgo, no C compiler, no SDK to install. gowaku calls Direct3D 11, Vulkan and Metal straight from Go, so you get a single binary, and the Linux and macOS ones from the same Windows laptop.",
		"f2.kicker":    "Fast, and quiet",
		"f2.title":     "Nothing moves,<br>nothing runs.",
		"f2.body":      "A declarative, Flutter-like widget tree. Only what changed is rebuilt, the drawing is batched for the GPU, and an idle window draws no frames at all.",
		"f2.big":       "Size does not slow it down: lists, tables and the editor lay out only what is in view, and a 10 GB log shows its first lines in 80 ms, with 84 MB of memory.",
		"f2.bench":     "CPU of one core, 100 animations at once",
		"f2.more":      "See all the benchmarks",
		"f3.kicker":    "Japanese, done properly",
		"f3.title":     "Vertical text, ruby,<br>and an IME that feels native.",
		"f3.body":      "Line-break rules and punctuation spacing after the W3C's JLReq. Inline IME composition with the active clause highlighted, on Windows, macOS and Wayland. Color emoji, Arabic and Hebrew too. What most toolkits treat as an edge case, gowaku was built around.",
		"f3.more":      "Read about text and the IME",
		"f4.kicker":    "Good-looking by default",
		"f4.title":     "Finished before<br>you start styling.",
		"f4.body":      "A quiet desktop design of its own. Light and dark follow the OS, and the whole palette comes from one accent color. Blur, shadows, gradients, custom shaders and SVG are there when you want more.",
		"f4.compare":   "Move between the light and the dark theme",
		"f4.hint":      "Drag across the picture: the same window in the light and the dark theme.",
		"f5.kicker":    "Built for real applications",
		"f5.title":     "Over a hundred widgets,<br>ready for real work.",
		"f5.body":      "Tables, trees, a code editor, a Markdown view, a dock layout, a command palette. Multiple windows, menus, title bars, tray icons, drag and drop, and screen readers on all three systems.",
		"f5.more":      "Open the catalog",
		"f6.kicker":    "Tested without a window",
		"f6.title":     "Your UI, in <code>go test</code>.",
		"f6.body":      "gowakutest runs the tree on the CPU with a fake clock. Find a button by its label, tap it, type, advance time and take a screenshot. It also flags overflow, unlabeled controls and low-contrast text.",
		"code.title":   "Reads like the UI it builds.",
		"code.body":    "Widgets are plain structs that work at their zero value. No builders, no reflection, no string IDs: the compiler checks the whole interface.",
		"cta.title":    "Make something quiet and fast.",
		"cta.body":     "Open source under the MIT License.",
		"bench.gowaku": "gowaku",
		"bench.gio":    "Gio",
		"bench.cogent": "Cogent Core",
		"platform.win": "Windows · Direct3D 11",
		"platform.lin": "Linux · Vulkan on Wayland or X11",
		"platform.mac": "macOS · Metal on Apple Silicon",
	},
	"ja": {
		"lang.name":               "日本語",
		"lang.other":              "English",
		"nav.docs":                "ドキュメント",
		"nav.catalog":             "ウィジェット",
		"nav.github":              "GitHub",
		"nav.guides":              "ガイド",
		"nav.reference":           "リファレンス",
		"nav.project":             "プロジェクト",
		"short.docs":              "目次",
		"short.docs/architecture": "アーキテクチャ",
		"short.docs/design":       "デザイン",
		"short.docs/benchmarks":   "ベンチマーク",
		"short.docs/roadmap":      "ロードマップ",
		"short.docs/contributing": "開発に参加する",
		"short.docs/changelog":    "変更履歴（英語）",
		"short.docs/security":     "セキュリティ（英語）",
		"short.docs/third-party":  "サードパーティのライセンス（英語）",
		"nav.overview":            "目次",
		"nav.theme":               "ライトとダークを切り替える",
		"nav.menu":                "メニュー",
		"toc":                     "このページの内容",
		"prev":                    "前へ",
		"next":                    "次へ",
		"edit":                    "GitHub でこのページを編集する",
		"copy":                    "コピー",
		"copied":                  "コピーしました",
		"footer.tagline":          "Go で書かれた、GPU ネイティブなデスクトップ UI ライブラリ。",
		"footer.license":          "MIT ライセンス",
		"footer.built":            "このサイトは、リポジトリの Markdown から自動で生成しています。",
		"site.title":              "gowaku — Pure Go で作る、美しく速いデスクトップアプリ",
		"site.desc":               "gowaku は Go で書かれた GPU ネイティブなデスクトップ UI ライブラリです。cgo も C コンパイラも不要で、アイドル時の CPU 使用率は 0%。日本語の組版と入力にも本格的に対応しています。",
		"notfound.title":          "ページが見つかりません。",
		"notfound.body":           "ドキュメントの整理で移動した可能性があります。",
		"notfound.home":           "トップページへ",
		"catalog.title":           "ウィジェットカタログ",
		"catalog.lede":            "gowaku の公開ウィジェットを、スクリーンショットとそれを描画したコードとともにすべて紹介します。",
		"catalog.search":          "ウィジェットを検索",
		"catalog.all":             "すべて",
		"catalog.none":            "該当するウィジェットはありません。",
		"catalog.count":           "項目",
		"catalog.note":            "画像は gpu/soft（CPU レンダラー）が Go フォントで描画しているため、画像内の文字は英語です。コード例に出てくる小文字の関数（themed、tile など）は、internal/catalog/helpers.go にある補助関数です。",
		"catalog.package":         "パッケージ",
		"catalog.also":            "関連",
		"catalog.code":            "コード",
		"catalog.nopic":           "自身では何も描画しないため、画像はありません。",

		"hero.title":   "Pure Go で作る、<wbr>美しく速いデスクトップアプリ。",
		"hero.lede":    "gowaku は画面のすべてを GPU で描画し、cgo なしで単一のバイナリにビルドできます。日本語の組版と入力にも、妥協なく対応しています。",
		"hero.start":   "はじめる",
		"hero.catalog": "ウィジェットを見る",
		"hero.caption": "たとえば、こんな画面を作れます。文字も差分の色も、1 ピクセル残らずライブラリが描いています。",
		"stat.start":   "最初のフレームまで",
		"stat.idle":    "アイドル時の CPU 使用率",
		"stat.anim":    "100 個の同時アニメーションでの CPU 使用率（Gio 比）",
		"stat.widgets": "ボタンからドックレイアウトまで、ウィジェットの数",
		"stat.c":       "ビルドに必要な C のコード（行）",
		"f1.kicker":    "純粋な Go",
		"f1.title":     "<code>go build</code> ひとつで、<br>すべての OS へ。",
		"f1.body":      "cgo も C コンパイラも、追加の SDK も必要ありません。Direct3D 11・Vulkan・Metal を Go から直接呼び出すので、成果物はバイナリ 1 つだけ。Windows のノート PC 1 台から、Linux 版も macOS 版もビルドできます。",
		"f2.kicker":    "速く、静かに",
		"f2.title":     "動きがなければ、<br>何も実行しない。",
		"f2.body":      "UI は Flutter のように宣言的に書けます。gowaku は変更のあった部分だけを再構築し、描画をまとめて GPU に送ります。動きのないウィンドウは、フレームを 1 枚も描画しません。",
		"f2.big":       "データが大きくても遅くなりません。リスト、表、エディタは表示中の部分だけをレイアウトし、10 GB のログでも最初の行が 80 ms で表示され、メモリは 84 MB です。",
		"f2.bench":     "100 個の同時アニメーションでの CPU 使用率（1 コア = 100%）",
		"f2.more":      "ベンチマークの詳細を見る",
		"f3.kicker":    "日本語に本気",
		"f3.title":     "縦書きもルビも、<br>自然な日本語入力も。",
		"f3.body":      "W3C の JLReq（日本語組版処理の要件）に沿って、禁則処理や約物の詰めを行います。IME は変換中の文字列をその場に表示し、注目文節も強調します（Windows・macOS・Wayland）。カラー絵文字やアラビア語、ヘブライ語にも対応。多くのツールキットが後回しにしてきた部分を、gowaku は設計の中心に置いています。",
		"f3.more":      "文字と IME のガイドへ",
		"f4.kicker":    "そのままで美しく",
		"f4.title":     "手を加える前から、<br>整った見た目に。",
		"f4.body":      "デスクトップのための、静かで控えめな独自のデザインです。ライトとダークは OS の設定に従い、配色はアクセントカラー 1 色からすべて生成します。ぼかしや影、グラデーション、独自のシェーダー、SVG も必要に応じて使えます。",
		"f4.compare":   "ライトとダークを見比べる",
		"f4.hint":      "画像の上をドラッグすると、同じウィンドウをライトとダークで見比べられます。",
		"f5.kicker":    "実用のアプリのために",
		"f5.title":     "実務に耐える、<br>100 以上のウィジェット。",
		"f5.body":      "表、ツリー、コードエディター、Markdown ビューアー、ドックレイアウト、コマンドパレットまでそろっています。複数ウィンドウ、メニュー、タイトルバー、システムトレイ、ドラッグ & ドロップに加え、3 つの OS のスクリーンリーダーにも対応しています。",
		"f5.more":      "カタログを見る",
		"f6.kicker":    "ウィンドウなしでテスト",
		"f6.title":     "UI のテストも <code>go test</code> で。",
		"f6.body":      "gowakutest は、ウィジェットツリーを CPU と仮想の時計で動かします。ラベルでボタンを探して押す、文字を入力する、時間を進める、スクリーンショットを撮る、といった操作をテストから行えます。はみ出しやラベルのない操作部品、コントラストの低い文字も検出します。",
		"code.title":   "書いたコードが、<br>そのまま UI の形になる。",
		"code.body":    "Widget はゼロ値のまま使える、ただの構造体です。ビルダーも reflect も文字列の ID も使わないので、UI 全体をコンパイラが検査できます。",
		"cta.title":    "静かで速いアプリを、Go で。",
		"cta.body":     "MIT ライセンスで公開しています。",
		"bench.gowaku": "gowaku",
		"bench.gio":    "Gio",
		"bench.cogent": "Cogent Core",
		"platform.win": "Windows · Direct3D 11",
		"platform.lin": "Linux · Vulkan（Wayland / X11）",
		"platform.mac": "macOS · Metal（Apple Silicon）",
	},
}

func word(lang, key string) string {
	if w, ok := words[lang][key]; ok {
		return w
	}
	if w, ok := words["en"][key]; ok {
		return w
	}
	panic("website: no word " + key)
}

// homeView is the data of the landing page.
type homeView struct {
	Code    template.HTML
	Build   template.HTML
	Test    template.HTML
	Mosaic  []catEntry
	Widgets int
}

const homeCounter = `func (s *counterState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: fmt.Sprintf("count: %d", s.n), Style: th.Text.LargeTitle},
		basic.AnimatedContainer{ // change a value, and it glides there
			Duration: 300 * time.Millisecond,
			Curve:    motion.EaseOutCubic,
			Color:    th.Color.AccentTint.WithAlpha(float32(s.n%10) / 10),
			Padding:  geom.All(16),
			Child: control.Button{
				Label:   "Add one",
				Style:   control.ButtonPrimary,
				OnPress: func() { s.SetState(func() { s.n++ }) },
			},
		},
	)}
}`

const homeBuild = `go build ./cmd/app                          # Windows
GOOS=linux  CGO_ENABLED=0 go build ./cmd/app  # Linux, from the same machine
GOOS=darwin GOARCH=arm64  go build ./cmd/app  # macOS`

const homeTest = `func TestAddOne(t *testing.T) {
	tt := gowakutest.New(t, counter{}, gowakutest.Options{})
	tt.Tap("Add one")
	if _, ok := tt.Find("count: 1"); !ok {
		t.Error("the count did not change")
	}
	for _, w := range tt.Warnings() {
		t.Error(w) // overflow, missing labels, low contrast
	}
}`

func (b *builder) home(lang string) error {
	cats, err := b.catalogModel(lang)
	if err != nil {
		return err
	}
	v := homeView{Code: template.HTML(highlight(homeCounter, "go")), Build: template.HTML(highlight(homeBuild, "bash")), Test: template.HTML(highlight(homeTest, "go"))}
	want := map[string]bool{"control.Button": true, "field.Calendar": true, "collection.Table": true, "field.ColorPicker": true,
		"nav.Breadcrumb": true, "control.SegmentedControl": true, "collection.Tree": true, "overlay.Toast": true,
		"control.Slider": true, "field.TokenField": true, "control.Switch": true, "control.ProgressRing": true}
	for _, c := range cats {
		for _, e := range c.Entries {
			v.Widgets++
			if want[e.Name] && e.Light != "" {
				v.Mosaic = append(v.Mosaic, e)
			}
		}
	}
	p := &page{
		Lang: lang, Title: word(lang, "site.title"), Description: word(lang, "site.desc"), Section: "home",
		URL: b.route("", lang), Alt: b.route("", other(lang)), Data: v,
	}
	return b.render("home", b.outPath(p.URL), p)
}
