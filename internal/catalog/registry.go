package catalog

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// Entries returns every entry of the catalog, in the order of the pages.
func Entries() []Entry {
	primary, secondary := input.ButtonPrimary, input.ButtonSecondary
	return []Entry{
		// Buttons and actions.
		{
			Name: "control.Button", Also: []string{"control.ButtonStyle", "control.ButtonSize"}, Category: "actions", Scene: sceneControlButton, Act: tabs(1),
			JA: "押すと `OnPress` を実行するボタン。`Style` で通常・主要・控えめ・破壊的の 4 種類の見た目を、`Size` で高さを選ぶ。`OnPress` を指定しないと無効になる。",
			EN: "Runs `OnPress` when pressed. `Style` picks normal, primary, quiet or danger, and `Size` the height. Without `OnPress` it is disabled.",
		},
		{
			Name: "interact.Pressable", Also: []string{"interact.PressState"}, Category: "actions", Scene: sceneInteractPressable, Act: hover("Open project"),
			JA: "独自の見た目で押せる領域を作るための土台。`Build` は hover・押下・フォーカスの状態を表す `PressState` を受け取り、中身を返す。",
			EN: "The base for a pressable area with a look of your own: `Build` gets the hover, press and focus `PressState` and returns the content.",
		},
		{
			Name: "control.Link", Category: "actions", Scene: sceneControlLink, Act: hover("Turn it on"),
			JA: "本文に添えて置く、押せる短いテキスト。アクセントの色で表示され、ポインタを重ねると下線が付く。Enter キーでは押せるが、Space キーでは押せない。`OnPress` を指定しないと無効になる。",
			EN: "A short text beside the text it belongs to that does something when pressed. It is set in the accent ink and underlined under the pointer. Enter presses it, Space does not. Without `OnPress` it is disabled.",
		},

		// Input.
		{
			Name: "field.Input", Also: []string{"field.InputController"}, Category: "input", Scene: sceneFieldInput,
			JA: "1 行または複数行のテキスト入力欄。ラベルは欄の上に表示される。IME、ルビ、縦書き、元に戻す操作に対応する。複数行の欄は、高さの決まっていない所では文字に合わせて伸び、`Expanded` などで高さを与えると、ラベルの有無にかかわらずその高さを埋める。`SubmitOnEnter` を指定した複数行の欄は、チャットの入力欄のように Enter で送信し、Shift+Enter で改行する。`Autofocus` を指定すると、ダイアログの中でも最初に現れたときにキーボードを受け取る。長い文書には `editor.Editor` を使う。",
			EN: "A single or multiline text field with its label above. It handles IMEs, ruby, vertical text and undo. A multiline field grows with its text where nothing sets its height, and fills the height that an `Expanded` or the like gives it, with or without a label. With `SubmitOnEnter`, a multiline field sends on Enter and breaks the line on Shift+Enter, as the box of a chat does. `Autofocus` gives it the keyboard when it first appears, also in a dialog. For long documents, use `editor.Editor`.",
		},
		{
			Name: "field.Search", Category: "input", Scene: sceneFieldSearch,
			JA: "検索用のテキスト入力欄。虫眼鏡のアイコンと消去ボタンが付き、Escape キーで内容を消去する。",
			EN: "A text field for searching, with a magnifier and a clear button. Escape empties it.",
		},
		{
			Name: "field.Autocomplete", Category: "input", Scene: sceneFieldAutocomplete, Act: focus(semantics.RoleTextField, "City", "a"),
			Width: 360, Height: 340,
			JA: "入力に応じて候補を欄の下に表示するテキスト入力欄。矢印キーで候補を選び、Enter か Tab で確定する。",
			EN: "A text field that suggests choices below itself as you type. The arrows move through them; Enter or Tab takes one.",
		},
		{
			Name: "editor.Editor", Also: []string{"editor.Controller", "editor.Decoration", "editor.TextPosition", "editor.TextRange", "editor.FindOptions"},
			Category: "input", Scene: sceneEditorEditor,
			JA: "コードや長文を編集するためのエディタ。行番号、色付けなどの装飾、検索と置換に対応し、表示中の行だけをレイアウトする。検索バーを開いている間は、バーの分の余白を先頭の行の上に取り、一致した箇所をバーの下に表示する。`Controller.SetSource` に渡した行のソース（ファイルなど）は、表示する行だけを読み、編集した行だけをメモリに持つので、数 GB でもすぐに開ける。",
			EN: "An editor for code and long text, with line numbers, decorations and find and replace. It lays out only the lines in view. While the find bar is open, the text keeps the room of the bar above its first line, and a match shows below the bar. Lines given through `Controller.SetSource`, as of a file, are read only where they are in view, and only the lines edited are held in memory, so that a file of gigabytes opens at once.",
		},
		{
			Name: "control.NumberStepper", Category: "input", Scene: sceneControlNumberStepper,
			JA: "マイナスとプラスのボタンで整数を増減するウィジェット。`Format` で単位を付けて表示できる。ウィザードの手順を示すものは `nav.Steps` である。",
			EN: "Changes a whole number with minus and plus buttons. `Format` adds a unit. For the steps of a wizard, see `nav.Steps`.",
		},
		{
			Name: "control.Slider", Category: "input", Scene: sceneControlSlider,
			JA: "範囲の中から値を選ぶスライダー。`Snap` を指定すると、値が `Step` の刻みに吸着する。矢印キーでも操作できる。",
			EN: "Picks a value in a range. `Snap` holds it to the grid of `Step`. The arrow keys move it too.",
		},
		{
			Name: "control.RangeSlider", Category: "input", Scene: sceneControlRangeSlider,
			JA: "2 つのつまみで範囲を選ぶ `Slider`。つまみの間の帯をドラッグすると、幅を保ったまま範囲全体が動く。つまみ同士は `MinGap` より近づかず、それぞれが Tab キーでの移動先になる。",
			EN: "A `Slider` with two thumbs that picks a range; dragging the band between them moves both. The thumbs stay `MinGap` apart, and each is a stop of Tab.",
		},
		{
			Name: "field.NumberField", Category: "input", Scene: sceneFieldNumberField,
			JA: "数値の入力欄。直接入力するほか、上下の矢印キー、フォーカス中のマウスホイール、右端のつまみのドラッグで値を増減できる。全角数字も受け付け、桁区切りと小数点の記号はロケールに従う。Enter キーで入力した文字を値として確定する。`FormItem` の中では、続けて周りの `Form` を新しい値で送信する。`FormItem` の外では確定するだけで、何も送信しない。",
			EN: "A field for a number, typed or stepped with the arrow keys, the wheel while it has the focus, or a drag of the handle at its end. It takes full-width digits, and the separators follow the locale. Enter takes the typed text as the value; inside a `FormItem` it then submits the `Form` around with the new value, and outside one it submits nothing.",
		},
		{
			Name: "field.TokenField", Category: "input", Scene: sceneFieldTokenField,
			JA: "タグや宛先のような短い文字列の並びを編集する入力欄。Enter か区切り文字で入力を確定すると、チップとして表示される。IME で変換中の Enter ではトークンを作らない。先頭で Backspace を 2 回押すと、直前のトークンを削除する。",
			EN: "Edits a list of short texts such as tags or addresses, each shown as a chip. Enter or a separator makes a token, but not while an IME converts. Two Backspaces at the start remove the last token.",
		},
		{
			Name: "field.Form", Also: []string{"field.FormItem", "field.FormController"}, Category: "input", Scene: sceneFieldForm,
			Act:    click(semantics.RoleButton, "Create account", 0, primary),
			Height: 460,
			JA:     "配下の `FormItem` をまとめて検証し、送信するフォーム。エラーは、送信時と欄からフォーカスが外れたときに、その欄の下に表示され、欄の枠が赤くなる。`Required` のエラーは中の部品に合わせて変わり、入力欄では「入力してください」、`DropdownMenu` や `DatePicker` などの選択では「選択してください」、`Checkbox` では「チェックしてください」になる。`RequiredMessage` で差し替えられる。`FormItem` の中の 1 行の入力欄で Enter キーを押すと、フォームを送信する。",
			EN:     "Checks and submits the `FormItem`s below it together. Errors show below the fields, whose borders turn red, on submit and when a field loses the focus. The error of `Required` follows the control inside: Required for a text field, Choose one for a choice such as a `DropdownMenu` or a `DatePicker`, Check this box for a `Checkbox`; `RequiredMessage` replaces it. Enter in a single-line field of a `FormItem` submits the form.",
		},
		{
			Name: "field.TimePicker", Also: []string{"field.HourCycle"}, Category: "input", Scene: sceneFieldTimePicker,
			JA: "時刻の入力欄。時・分・秒・午前午後の部分ごとに、矢印キーか数字で編集する。12 時間制か 24 時間制かはロケールに従う。",
			EN: "A field for a time of day, edited part by part with the arrows or digits. The locale picks the 12 or 24 hour clock.",
		},

		// Selection.
		{
			Name: "control.Checkbox", Category: "selection", Scene: sceneControlCheckbox,
			JA: "オンとオフを切り替えるチェックボックス。ラベルを含めた全体を押せる。",
			EN: "A box that turns an option on and off. The label is part of the target.",
		},
		{
			Name: "control.Switch", Category: "selection", Scene: sceneControlSwitch,
			JA: "すぐに反映される設定のオンとオフを切り替えるスイッチ。つまみはスプリングのアニメーションで動く。",
			EN: "Turns a setting on and off at once, with a knob that moves on a spring.",
		},
		{
			Name: "control.RadioGroup", Also: []string{"control.RadioOption"}, Category: "selection", Scene: sceneControlRadioGroup,
			JA: "複数の選択肢から 1 つを選ぶラジオボタンのグループ。Tab キーではグループ全体で 1 か所として止まり、選択肢の間は矢印キーで移動する。",
			EN: "A set of radio buttons that picks one choice. It is one stop for Tab, and the arrows move between the choices.",
		},
		{
			Name: "control.Radio", Category: "selection", Scene: sceneControlRadio,
			JA: "単独のラジオボタン。`RadioGroup` では実現できない配置のときに使い、グループの名前は `Semantics` で付ける。",
			EN: "A single radio button, for layouts a `RadioGroup` cannot give. Name the set with a `Semantics`.",
		},
		{
			Name: "control.SegmentedControl", Category: "selection", Scene: sceneControlSegmentedControl,
			JA: "2〜4 個の選択肢から 1 つを選ぶセグメントコントロール。選択中の項目を示す板が、スプリングのアニメーションで滑る。",
			EN: "Picks one of two to four choices, with a plate that slides on a spring.",
		},
		{
			Name: "field.DropdownMenu", Also: []string{"field.DropdownOption"}, Category: "selection", Scene: sceneFieldDropdownMenu,
			Act: expand(semantics.RoleComboBox, "Plan"), Width: 360, Height: 300,
			JA: "選択肢の一覧を下に開いて 1 つを選ぶ欄。文字を入力すると、その文字で始まる選択肢へ移動する。",
			EN: "A field that opens a list below itself to pick one choice. Typing a letter jumps to the choices that start with it.",
		},
		{
			Name: "control.Chip", Category: "selection", Scene: sceneControlChip,
			JA: "小さな角丸のラベル。`OnChange` を指定すると絞り込みの切り替えに、`OnPress` を指定するとボタンになり、どちらも無ければ単なるタグとして表示される。",
			EN: "A compact pill: a toggling filter with `OnChange`, a button with `OnPress`, or a plain tag with neither.",
		},
		{
			Name: "field.Calendar", Category: "selection", Scene: sceneFieldCalendar,
			JA: "月のカレンダーから日付を選ぶウィジェット。週の始まりの曜日と曜日名はロケールに従う。`Marks` で日付の下に点を表示できる。見出しを押すとその年の月の一覧に、さらに年の見出しを押すと 10 年分の年の一覧に切り替わるので、生年月日のような遠い日付も数回の操作で選べる。`Value` が空のときは `InitialMonth` の月から表示する。",
			EN: "Picks a day from a month. The first day of the week and the names follow the locale. `Marks` puts dots under days. The title opens the months of the year, and the year the years of a decade, so a day far away, such as a birth date, takes a few presses. While `Value` is zero it opens on `InitialMonth`.",
		},
		{
			Name: "field.DatePicker", Category: "selection", Scene: sceneFieldDatePicker,
			Act: steps(geom.Size{W: 400, H: 440},
				click(semantics.RoleComboBox, "", 0, primary),
				click(semantics.RoleButton, "November 2026", 0, primary),
				click(semantics.RoleButton, "2026", 0, primary),
			),
			Width: 400, Height: 440,
			JA: "日付を表示する欄。開くと、下に `Calendar` を表示する。カレンダーの見出しから月と年の一覧に切り替えられる（図は年の一覧）。生年月日の欄には、30 年ほど前の `InitialMonth` と今日の `Max` を指定するとよい。`FormItem` の中では、エラーのときに枠が赤くなる。",
			EN: "A field that shows a date and opens a `Calendar` below itself, whose title switches to the months and the years (the picture shows the years). For a birth date, give an `InitialMonth` some thirty years back and a `Max` of today. In a `FormItem` its border turns red with an error.",
		},
		{
			Name: "field.ColorPicker", Category: "selection", Scene: sceneFieldColorPicker, Width: 400, Height: 460,
			JA: "彩度と明度の平面、色相と透明度の帯、hex の入力欄で色を選ぶカラーピッカー。値を HSV で保持するため、黒や灰色を経由しても色相が失われない。`OnChange` を指定した `collection.PropertyColor` を押すと、これが開く。",
			EN: "Chooses a color with a plane of saturation and brightness, strips of hue and opacity, and a hex field. It keeps HSV, so the hue survives black and gray. A `collection.PropertyColor` with `OnChange` opens one.",
		},

		// Display.
		{
			Name: "basic.Text", Category: "display", Scene: sceneBasicText,
			JA: "文字列を `Style` の規則に従って行分割し、表示するウィジェット。和文の禁則処理、縦書き、両端揃えに対応する。`Style.MaxLines` で行数を制限すると、収まらない分を省略記号「…」で切り詰める。スクリーンリーダーは全文を読む。",
			EN: "Shows a string broken into lines by the rules of its `Style`, with Japanese line breaking, vertical text and justification. `Style.MaxLines` cuts it short with an ellipsis; a screen reader still reads it whole.",
		},
		{
			Name: "basic.RichText", Category: "display", Scene: sceneBasicRichText,
			JA: "見た目の異なる区間を 1 つの段落に混在させて表示するウィジェット。ルビと傍点も付けられ、`Background` で検索の一致などに背景色を敷ける。`Style.MaxLines` は `Text` と同じく働く。",
			EN: "Mixes spans of different looks in one paragraph, with ruby and emphasis dots, and a `Background` behind a span, such as a match of a search. `Style.MaxLines` works as for `Text`.",
		},
		{
			Name: "field.SelectionArea", Category: "display", Scene: sceneFieldSelectionArea,
			Act: dragText("Upload failed", "dial tcp 192.0.2.10:443: i/o timeout"), Width: 440, Height: 160,
			JA: "配下の `Text` と `RichText` を、マウスとキーボードで選択してコピーできるようにする。選択は段落をまたぐことができ、ルビの部分は親文字だけがコピーされる。",
			EN: "Makes the `Text` and `RichText` below it selectable with the mouse and the keyboard, and copyable, across paragraphs; ruby copies as its base text.",
		},
		{
			Name: "basic.Icon", Category: "display", Scene: sceneBasicIcon,
			JA: "24 単位の格子の上に定義したベクター図形を描くアイコン。`Label` が無い場合は装飾として扱われる。よく使う 24 種の図形は `icons` パッケージにある（フォルダ、ファイル、検索、閉じる、山形、歯車、ごみ箱など）。",
			EN: "Draws a vector shape on a 24 unit grid. Without a `Label` it is decorative. Package `icons` holds 24 common shapes: folder, file, search, close, chevrons, gear, trash and more.",
		},
		{
			Name: "basic.Image", Category: "display", Scene: sceneBasicImage,
			JA: "画像を表示するウィジェット。`Fit` で枠への収め方、`Radius` で角丸、`Quality` で拡大縮小の補間方法を選ぶ。",
			EN: "Shows a picture. `Fit` picks how it fills its box, `Radius` rounds it and `Quality` sets the filtering.",
		},
		{
			Name: "basic.SVG", Category: "display", Scene: sceneBasicSVG,
			JA: "SVG 画像を正方形に収めて描画するウィジェット。`Color` は、アイコンセットが描画に使う currentColor の値になる。",
			EN: "Draws an SVG drawing fitted into a square. `Color` is the currentColor icon sets draw with.",
		},
		{
			Name: "basic.AnimatedImage", Category: "display", Scene: sceneBasicAnimatedImage,
			JA: "GIF、APNG、WebP のアニメーション画像を再生するウィジェット。動きを減らす設定が有効なときは、最初のフレームで止まる。",
			EN: "Plays an animated GIF, APNG or WebP. When the OS asks for less motion it shows the first frame.",
		},
		{
			Name: "control.Badge", Category: "display", Scene: sceneControlBadge,
			JA: "未読数、短い語、点で注意を促すバッジ。子を指定すると、その右上に重ねて表示される。",
			EN: "A small mark for a count, a word or a dot. With a child it sits on the child's top right corner.",
		},
		{
			Name: "control.Card", Category: "display", Scene: sceneControlCard,
			JA: "1 つのまとまった内容を載せるカード。`OnPress` を指定すると、カード全体がボタンになる。",
			EN: "A surface that holds one piece of content. With `OnPress` the whole card is a button.",
		},
		{
			Name: "control.Disclosure", Category: "display", Scene: sceneControlDisclosure,
			JA: "見出しを押すと下の内容が開閉するウィジェット。閉じている間は中身がレイアウトされず、Tab キーでの移動や支援技術からも飛ばされる。`OnChange` を指定すると開閉の状態を親が管理するようになり、いくつか並べてアコーディオンにできる。",
			EN: "A header that opens and closes the content below it. Closed content is not laid out, and Tab and assistive technology pass it by. With `OnChange` the parent keeps the state, which makes several in a row an accordion.",
		},
		{
			Name: "interact.InteractiveViewer", Also: []string{"interact.ViewerController", "interact.ViewerBoundary", "interact.ViewerKeys"}, Category: "display", Scene: sceneInteractInteractiveViewer,
			Width: 480, Height: 320,
			JA: "子をパンとズームで表示するウィジェット。ドラッグで移動し、Ctrl+ホイールかピンチで、ポインタの位置を中心に拡大縮小する。`WheelZooms` を指定すると、画像ビューアのようにホイールだけでも拡大縮小する。矢印キーは動かせる余地があるときだけ使い、それ以外はアプリのショートカットに渡す。`Keys` で矢印キーを使わないことも、フォーカスを受け取らないこともできる。子のレイアウトは 1 度だけ行われ、中のボタンもそのまま押せる。`ViewerController` を使うと、外から表示を動かせる。",
			EN: "Shows its child through a pan and a zoom: drag to move, Ctrl with the wheel or a pinch to zoom about the pointer. `WheelZooms` zooms with the plain wheel, as image viewers do. The arrow keys pan only while there is room to, and otherwise go on to the shortcuts of the app; `Keys` leaves them out, or keeps the viewer out of the focus. The child is laid out once, and the buttons in it keep working. `ViewerController` moves the view from outside.",
		},
		{
			Name: "basic.Divider", Category: "display", Scene: sceneBasicDivider,
			JA: "内容の区切りに引く細い線。`Indent` で端に余白を空け、`Axis` で縦線にもできる。",
			EN: "A hairline between groups of content. `Indent` keeps it short of the edge, and `Axis` stands it up.",
		},
		{
			Name: "control.Sparkline", Also: []string{"control.SparklineKind"}, Category: "display", Scene: sceneControlSparkline, Width: 420, Height: 200,
			JA: "表のセルに収まる小さな折れ線グラフと棒グラフ。軸も凡例も描かない。NaN の値は欠損として途切れ、棒はゼロを基準に上下へ伸びる。描画時にメモリを割り当てない。",
			EN: "A line or bar chart the size of a table cell, without axes or legend. NaN leaves a gap, and bars rise and fall from zero. Painting does not allocate.",
		},

		// Navigation.
		{
			Name: "control.Tabs", Also: []string{"control.TabPage"}, Category: "navigation", Scene: sceneControlTabs,
			JA: "ページを切り替えるタブ。下線付きのタブの並びの下に、選択したページを表示する。",
			EN: "Switches between pages, with the chosen page below a row of underlined tabs.",
		},
		{
			Name: "collection.Sidebar", Also: []string{"collection.SidebarItem"}, Category: "navigation", Scene: sceneCollectionSidebar, Height: 380,
			JA: "ウィンドウの左端に置く、移動先の一覧。背景と右端の区切り線を自分で描くので、`CrossStretch` の `Row` の先頭に置く。幅は `Width` で決まり、ゼロなら 208。行が収まらないときは行だけがスクロールし、タイトル、その下の `Header`（検索欄など）、下端に固定する `Footer`（ボタンなど）は動かない。選択中の行は `AccentTint` の背景になる。`SidebarItem` の `Count` は、未読の件数などを行の右端に控えめな丸い枠で表示し、スクリーンリーダーには行の値として 1 度だけ伝える。`Detail` は、チャットの最後のメッセージのような 2 行目を小さく `Ink2` で表示し（選択中の行では、件数とともに、色合いの上で読める `theme.Colors.OnTint` の色）、1 行に収まらない分は省略記号で切る。`Detail` のある行は 46 の高さになり、スクリーンリーダーには詳細の全文を行の説明として 1 度だけ伝える。`Leading` は、メールのラベルの色の点のようなウィジェットを `Icon` の代わりに行頭に置く。選択中の行では `OnTint` のテーマの下で作るので、テーマの色を使う部分は色合いの上で読め、点のように自分で決めた色はそのまま残る。",
			EN: "The column of places at the left of a window. It draws its own ground and the hairline at its right edge; put it first in a `Row` with `CrossStretch`. `Width` sets its width, 208 when zero. When the rows do not fit, they alone scroll: the title, the `Header` under it, such as a search field, and the `Footer` pinned at the bottom, such as buttons, stay. The selected row takes the `AccentTint` ground. The `Count` of a `SidebarItem`, such as of unread messages, shows in a quiet pill at the end of the row, and a screen reader hears it once, as the value of the row. A `Detail` is a smaller second line in `Ink2` (on the selected row in the colors of `theme.Colors.OnTint`, which read on the tint, as is its count), such as the last message of a chat, cut to one line with an ellipsis; a row with one is 46 high, and a screen reader hears the whole detail once, as the description of the row. `Leading` puts a widget in place of the `Icon`, such as the colored dot of a mail label; on the selected row it is built under the `OnTint` theme, so that what takes the colors of the theme reads on the tint, while a color of its own, as that of the dot, stays.",
		},
		{
			Name: "nav.Breadcrumb", Also: []string{"nav.BreadcrumbItem"}, Category: "navigation", Scene: sceneNavBreadcrumb,
			JA: "現在地までの道筋を示すパンくずリスト。収まらない場合は、中ほどの項目をメニューに畳む。",
			EN: "The trail to the current place. Where it does not fit, the middle folds into a menu.",
		},
		{
			Name: "nav.Steps", Category: "navigation", Scene: sceneNavSteps, Height: 160,
			JA: "ウィザードや購入手続きのように、いくつかの手順を順に進む流れで、今どこにいるかを示す。手順ごとに番号の丸と名前を並べ、間を線でつなぐ。`Current` より前の手順は済んだものとしてチェックを、現在の手順はアクセントの色で塗った丸と太字の名前を表示する。`OnStepPress` を指定すると、済んだ手順が押せるようになり、Tab キーで移動して Enter か Space で戻れる。スクリーンリーダーには手順の一覧として、名前、位置（「3 件中 2 番目」など）、「現在の手順」か「完了」を伝える。数値を増減するものは `control.NumberStepper` である。",
			EN: "Shows where a flow of several steps is, such as a wizard or a checkout: a numbered circle and a name for each step, joined by lines. The steps before `Current` are done and show a check; the current one has a circle in the accent and a bold name. With `OnStepPress`, the done steps can be pressed to go back, and Tab and Enter or Space reach them. A screen reader hears a list of the steps, each with its name, its place, such as 2 of 3, and \"Current step\" or \"Completed\". For the minus and plus buttons of a number, see `control.NumberStepper`.",
		},
		{
			Name: "nav.Navigator", Also: []string{"nav.NavigatorController", "nav.PageRoute", "nav.RoutePattern", "nav.RouteParams"},
			Category: "navigation", Scene: sceneNavNavigator,
			JA: "ページを積み重ねて画面を遷移させるウィジェット。名前のパターンに対応するルートでページを作り、Alt+Left で前のページに戻る。",
			EN: "A stack of pages. Routes build pages from name patterns, and Alt+Left goes back.",
		},
		{
			Name: "nav.BackButton", Category: "navigation", Scene: sceneNavBackButton, Show: sceneNavNavigator,
			Act: click(0, "Trip to Kyoto", 0, primary),
			JA:  "最も近い `Navigator` の前のページに戻るボタン。最初のページでは何も表示しない。",
			EN:  "Goes back to the previous page of the nearest `Navigator`. On the first page it shows nothing.",
		},

		// Feedback and overlays.
		{
			Name: "control.ProgressBar", Category: "feedback", Scene: sceneControlProgressBar,
			JA: "処理の進み具合を、左から伸びるバーで示すウィジェット。値が変わると、新しい位置まで滑らかに移動する。`Indeterminate` にすると、長さの分からない処理を、バーを繰り返し横切る帯で示し、スクリーンリーダーには値の無い処理中の表示として伝える。帯は見えている間だけ動き、動きを減らす設定が有効なときは中央で止まる。",
			EN: "Shows how much is done as a bar that fills from the left. A new value slides into place. With `Indeterminate` it shows work of unknown length as a band that crosses the bar again and again, and a screen reader hears a busy indicator with no value. The band moves only while it is seen, and stands in the middle when the window asks for less motion.",
		},
		{
			Name: "control.ProgressRing", Category: "feedback", Scene: sceneControlProgressRing,
			JA: "処理の進み具合を、12 時の位置から時計回りに満ちていくリングで示すウィジェット。中央に子を置ける。`Indeterminate` にすると、長さの分からない処理のためのスピナーになり、弧がリングの上を回る。スクリーンリーダーには値の無い処理中の表示として伝え、ラベルが無ければウィンドウの言語で「読み込み中」と読む。弧は見えている間だけ回り、動きを減らす設定が有効なときは 12 時の位置で止まる。",
			EN: "Shows how much is done as a ring that fills clockwise from twelve, with a child in its middle. With `Indeterminate` it is a spinner for work of unknown length: an arc turns around the ring. A screen reader hears a busy indicator with no value, named Loading in the language of the window when it has no label. The arc turns only while it is seen, and stands at twelve when the window asks for less motion.",
		},
		{
			Name: "control.Skeleton", Also: []string{"control.SkeletonShape", "control.SkeletonText", "control.SkeletonGroup"}, Category: "feedback", Scene: sceneControlSkeleton,
			JA: "読み込み中の内容の代わりに表示するプレースホルダー。光の帯はウィンドウ全体で 1 本だけ流れ、スケルトンが見えている間だけ動く。動きを減らす設定が有効なときは止まる。`SkeletonGroup` は、読み込み中であることを支援技術に伝える。",
			EN: "Stands in for content that is still loading. One band of light crosses the window, only while a skeleton is seen, and stops when the OS asks for less motion. `SkeletonGroup` tells assistive technology that it is loading.",
		},
		{
			Name: "overlay.Tooltip", Category: "feedback", Scene: sceneOverlayTooltip, Act: tabs(1), Width: 420, Height: 160,
			JA: "子にポインタを重ねるか、キーボードでフォーカスを移すと表示される短い説明。支援技術には子の説明として伝わる。",
			EN: "A short label that shows when the pointer rests on its child or the keyboard moves to it. Assistive technology gets it as the description.",
		},
		{
			Name: "overlay.Toast", Category: "feedback", Scene: sceneOverlayToast, Act: click(semantics.RoleButton, "Move to trash", 0, primary),
			Width: 480, Height: 200,
			JA: "ウィンドウの下部に数秒間表示される短い通知。「元に戻す」のような操作を 1 つ付けられる。`ShowToast` で表示する。",
			EN: "A short message at the bottom of the window for a few seconds, with an optional action such as Undo. Show it with `ShowToast`.",
		},
		{
			Name: "control.InfoBar", Also: []string{"control.InfoSeverity"}, Category: "feedback", Scene: sceneControlInfoBar,
			JA: "ページ内に表示され続ける通知の帯。情報・成功・警告・エラーに応じたアイコンと色で表示し、閉じるボタンと操作を 1 つ付けられる。表示するかどうかは、親が `Open` で決める。",
			EN: "A notice that stays in the page: info, success, warning or error, with a close button and one action. The parent shows it with `Open`.",
		},
		{
			Name: "overlay.Dialog", Category: "feedback", Scene: sceneOverlayDialog, Act: click(semantics.RoleButton, "Delete", 0, primary),
			Width: 520, Height: 260,
			JA: "作業を中断して応答を求めるカード。`ShowDialog` で表示し、フォーカスはダイアログの中に閉じ込められる。`Glass` を指定すると背景をぼかす。",
			EN: "A card that stops the work for an answer. Show it with `ShowDialog`; the focus stays inside. `Glass` blurs what is behind.",
		},
		{
			Name: "overlay.Sheet", Also: []string{"overlay.SheetEdge"}, Category: "feedback", Scene: sceneOverlaySheet,
			Act: click(semantics.RoleButton, "Share", 0, primary), Width: 520, Height: 260,
			JA: "ウィンドウの下端か横からせり出すパネル。`ShowSheet` で表示する。",
			EN: "A panel that slides in from the bottom or the side of the window. Show it with `ShowSheet`.",
		},

		// Data.
		{
			Name: "collection.List", Category: "data", Scene: sceneCollectionList,
			JA: "任意の件数を扱える縦方向のリスト。表示範囲付近の行だけを構築する。行ごとに高さが異なってもよい。",
			EN: "A vertical list of any length that builds only the rows near the view. Rows may differ in height.",
		},
		{
			Name: "collection.GridView", Category: "data", Scene: sceneCollectionGridView, Width: 500, Height: 320,
			JA: "任意の個数のタイルを並べるグリッド。表示範囲付近の行だけを構築する。列数は幅から決まり、選択とフォーカスは `TableController` が管理し、`ItemKey` を与えると項目の並びが変わっても同じ項目に残る。矢印キーでタイル間を移動し、Enter で開く。",
			EN: "A grid of tiles of any number that builds only the rows near the view. The width decides the columns, and a `TableController` holds the selection and the focus, which stay on the same items when they move given an `ItemKey`. The arrows move between tiles and Enter opens one.",
		},
		{
			Name: "collection.ReorderableList", Category: "data", Scene: sceneCollectionReorderableList,
			JA: "ドラッグか Ctrl+Shift+矢印キーで項目を並べ替えられる `List`。データ自体は保持せず、`OnReorder` で移動を依頼する。スクリーンリーダーには移動後の位置を伝える。",
			EN: "A `List` whose items move by drag or with Ctrl+Shift and an arrow. It does not own the items: `OnReorder` asks for the move. A screen reader hears the new place.",
		},
		{
			Name: "collection.Table", Also: []string{"collection.TableColumn", "collection.TableController", "collection.TableSort", "collection.TableFilter", "collection.TableEdit", "collection.TableEditor", "collection.CellKind", "collection.CellTone", "collection.RowRange", "collection.SortOrder", "collection.TableSelectMode"},
			Category: "data", Scene: sceneCollectionTable,
			JA: "並べ替え、絞り込み、範囲選択、セルの編集、列の固定に対応した表。セルには文字のほか、バッジ、点、バー、チェックボックスを表示できる。`RowKey` を与えると、並べ替えや行の削除の後も選択が同じ行に残る。並べ替えた列の見出しは、スクリーンリーダーに昇順か降順かを伝える。ファイルの行のように別の場所にある行は、スライスにせず `RowCount` と `Row` で渡せる。",
			EN: "A table with sorting, filters, range selection, cell editing and frozen columns. Cells show text, badges, dots, bars or check boxes. With `RowKey` the selection stays on the same rows when they are sorted or deleted. The header of the sorted column tells a screen reader its order. Rows that live elsewhere, as the lines of a file, come through `RowCount` and `Row` without a slice of them.",
		},
		{
			Name: "collection.Tree", Also: []string{"collection.TreeController", "collection.TreeMove", "collection.TreeDrop"}, Category: "data", Scene: sceneCollectionTree, Act: tabs(1),
			JA: "ノードを開閉できるツリー。複数選択、遅延読み込み、名前の変更、ドラッグによる移動に対応する。",
			EN: "A tree of nodes that open and close, with multiple selection, lazy loading, renaming and moves by drag.",
		},
		{
			Name: "collection.PropertyGrid", Also: []string{"collection.Property", "collection.PropertyCategory", "collection.PropertyEditor", "collection.PropertyText", "collection.PropertyBool", "collection.PropertyChoice", "collection.PropertyInt", "collection.PropertyNumber", "collection.PropertyColor", "collection.PropertyCustom"},
			Category: "data", Scene: sceneCollectionPropertyGrid, Height: 480,
			JA: "名前と値の 2 列で属性を編集するインスペクタ。カテゴリごとに折りたためる。",
			EN: "An inspector that edits properties in two columns of names and values, in categories that fold.",
		},
		{
			Name: "collection.Group", Also: []string{"collection.GroupRow"}, Category: "data", Scene: sceneCollectionGroup,
			JA: "角丸のカードに行を並べるウィジェット。設定画面やフォームは、これをいくつか並べて作る。行の区切り線は、左端まで届かずに止まる。",
			EN: "A card of rows, the stuff of settings pages and forms. The lines between rows stop short of the left edge.",
		},

		// Desktop.
		{
			Name: "overlay.MenuBar", Also: []string{"overlay.MenuItem", "interact.Action", "interact.Actions", "interact.Shortcut"}, Category: "desktop", Scene: sceneOverlayMenuBar,
			Act: click(0, "Edit", 0, primary), Width: 480, Height: 300,
			JA: "ウィンドウ上部のメニューバー。各 `Action` に、表示名、ショートカット、チェックの有無、実行できるかどうかをまとめて指定する。",
			EN: "The menu bar at the top of a window. Each `Action` holds the label, the shortcut, the check mark and whether it can run.",
		},
		{
			Name: "overlay.ContextMenu", Category: "desktop", Scene: sceneOverlayContextMenu,
			Act: click(0, "Right-click here", 40, secondary), Width: 480, Height: 340,
			JA: "右クリックかメニューキーで、子の上にメニューを開くウィジェット。",
			EN: "Opens a menu over its child on a right click or the menu key.",
		},
		{
			Name: "nav.Toolbar", Also: []string{"nav.ToolbarItem"}, Category: "desktop", Scene: sceneNavToolbar,
			JA: "ボタンを横に並べるツールバー。収まらない項目は末尾のメニューに移る。項目の間は矢印キーで移動する。",
			EN: "A row of buttons. Items that do not fit move into a menu at the end; the arrow keys move between items.",
		},
		{
			Name: "nav.StatusBar", Also: []string{"nav.StatusItem"}, Category: "desktop", Scene: sceneNavStatusBar,
			JA: "ウィンドウ下端のステータスバー。左寄せと右寄せの項目を、区切り線で分けて並べる。幅が狭いときは、優先度の低い項目から隠す。",
			EN: "The band along the bottom of a window, with leading and trailing items. When narrow, items of low priority hide first.",
		},
		{
			Name: "field.CommandPalette", Also: []string{"field.PaletteItem"}, Category: "desktop", Scene: sceneFieldCommandPalette,
			Act: click(semantics.RoleButton, "Open palette", 0, primary), Width: 600, Height: 440,
			JA: "登録した Action をあいまい検索して実行するコマンドパレット。Ctrl+Shift+P で開く。",
			EN: "Finds a registered Action by fuzzy search and runs it. Ctrl+Shift+P opens it.",
		},
		{
			Name: "nav.TabBar", Also: []string{"nav.TabItem", "nav.TabGroup", "nav.TabTearOff"}, Category: "desktop", Scene: sceneNavTabBar,
			JA: "文書を切り替えるタブ。タブは閉じる、並べ替える、別のウィンドウへ切り離すことができる。保存していない文書のタブには点が付き、ポインターを載せると閉じるボタンに変わる。",
			EN: "Document tabs that close, reorder, and tear off into another window. A tab with unsaved changes shows a dot, which turns into the close button under the pointer.",
		},
		{
			Name: "control.SplitPane", Also: []string{"control.SplitCollapse"}, Category: "desktop", Scene: sceneControlSplitPane,
			JA: "2 つのペインを、ドラッグできる境界で分割するウィジェット。片方のペインは折りたためる。",
			EN: "Two panes divided by a draggable divider. One of them can fold away.",
		},
		{
			Name: "nav.DockLayout", Also: []string{"nav.DockPanel", "nav.DockController", "nav.DockArrangement", "nav.DockNode", "nav.DockZone"},
			Category: "desktop", Scene: sceneNavDockLayout, Width: 600, Height: 340,
			JA: "IDE のようにタブのグループを配置するウィジェット。境界をドラッグすると大きさが変わり、タブをドラッグするとグループ間や端へ移動できる。移動してもパネルの状態は保たれる。配置は `DockArrangement` として保存し、復元できる。",
			EN: "Arranges groups of tabs as an IDE does. Dragging a divider resizes, and dragging a tab moves it between groups or to an edge, keeping its state. The arrangement saves and restores as a `DockArrangement`.",
		},

		// Layout.
		{
			Name: "basic.Row", Category: "layout", Scene: sceneBasicRow,
			JA: "子を横に並べるレイアウト。`Main` で主軸方向、`Cross` で交差軸方向の揃え方を決め、`Spacing` で子の間隔を空ける。",
			EN: "Lays out its children in a row. `Main` and `Cross` align them, and `Spacing` puts gaps between.",
		},
		{
			Name: "basic.Column", Category: "layout", Scene: sceneBasicColumn,
			JA: "子を縦に並べるレイアウト。`Size: layout.MainMin` を指定すると、中身の高さまで縮む。",
			EN: "Lays out its children in a column. `Size: layout.MainMin` shrinks it to its content.",
		},
		{
			Name: "basic.Flex", Category: "layout", Scene: sceneBasicFlex,
			JA: "`Axis` で方向を選べる、`Row` と `Column` の基になるレイアウト。並べる方向を実行時に変えたいときに使う。",
			EN: "The `Row` and `Column` with its axis as a field, for a direction chosen at run time.",
		},
		{
			Name: "basic.Expanded", Category: "layout", Scene: sceneBasicExpanded,
			JA: "`Row` や `Column` の残りの領域を、`Flex` の比率で分け合って埋める。",
			EN: "Fills the rest of a `Row` or `Column`, shared by the weights of `Flex`.",
		},
		{
			Name: "basic.Flexible", Category: "layout", Scene: sceneBasicFlexible,
			JA: "残りの領域を分け合うが、子は割り当てより小さくてもよい。`Fit: layout.FitTight` を指定すると `Expanded` と同じ動作になる。",
			EN: "Shares the free space, but the child may stay smaller than its share. `Fit: layout.FitTight` makes it an `Expanded`.",
		},
		{
			Name: "basic.Stack", Also: []string{"basic.Positioned"}, Category: "layout", Scene: sceneBasicStack,
			JA: "子を重ねて描くレイアウト。端からの位置は `Positioned` で指定する。",
			EN: "Paints its children on top of each other. `Positioned` places a child from the edges.",
		},
		{
			Name: "basic.PositionedFill", Category: "layout", Scene: sceneBasicPositionedFill,
			JA: "子を `Stack` 全体に広げる。`Stack` の大きさは、ほかの子によって決まる。",
			EN: "Stretches a child over the whole `Stack`, whose size the other children decide.",
		},
		{
			Name: "basic.Wrap", Category: "layout", Scene: sceneBasicWrap,
			JA: "収まらない子を次の行に折り返す `Row`。タグやチップを並べるのに向く。",
			EN: "A `Row` that starts a new run when the next child does not fit, for tags and chips.",
		},
		{
			Name: "basic.Grid", Category: "layout", Scene: sceneBasicGrid,
			JA: "同じ幅の列にタイルを並べるレイアウト。列数は固定するか、最小幅から決める。",
			EN: "Lays out tiles in columns of the same width: a fixed number, or as many as a minimum width allows.",
		},
		{
			Name: "basic.Padding", Category: "layout", Scene: sceneBasicPadding,
			JA: "子の周りに余白を空ける。",
			EN: "Puts room around its child.",
		},
		{
			Name: "basic.Align", Category: "layout", Scene: sceneBasicAlign,
			JA: "与えられた領域の中に子を配置する。位置は各軸 0〜1 で指定する（Flutter のような -1〜1 ではない）。",
			EN: "Places its child within the space it is given, from 0 to 1 on each axis (not -1 to 1 as in Flutter).",
		},
		{
			Name: "basic.Center", Category: "layout", Scene: sceneBasicCenter,
			JA: "子を中央に配置する。",
			EN: "Places its child in the middle.",
		},
		{
			Name: "basic.SizedBox", Category: "layout", Scene: sceneBasicSizedBox,
			JA: "幅、高さ、またはその両方を固定する。子が無ければ空白になる。",
			EN: "Forces a width, a height or both. Without a child it is an empty gap.",
		},
		{
			Name: "basic.ConstrainedBox", Category: "layout", Scene: sceneBasicConstrainedBox,
			JA: "子の大きさに、最小値と最大値の制約を加える。",
			EN: "Adds minimum and maximum sizes to its child.",
		},
		{
			Name: "basic.Container", Category: "layout", Scene: sceneBasicContainer,
			JA: "大きさ、余白、色、グラデーション、枠線、角丸、影をまとめて指定できる箱。",
			EN: "A box that combines size, padding, color, gradient, border, corners and shadows.",
		},
		{
			Name: "basic.DecoratedBox", Category: "layout", Scene: sceneBasicDecoratedBox,
			JA: "子の背後に `layout.Decoration` を描く。",
			EN: "Paints a `layout.Decoration` behind its child.",
		},
		{
			Name: "basic.Scroll", Category: "layout", Scene: sceneBasicScroll,
			JA: "自分より大きい子を、ホイールとスクロールバーで動かして表示する。親が大きさを制約する必要がある。`Reverse` を指定すると右端（縦方向では下端）から始まり、長い縦書きの文章が最初の列から見える。`ScrollController` でコードから位置を読み書きできる。",
			EN: "Shows a child larger than itself, moved by the wheel and a scroll bar. Its parent must bound its size. `Reverse` starts it at the right end, or the bottom, so that a long vertical text opens at its first column. A `ScrollController` reads and sets the offset from code.",
		},
		{
			Name: "basic.CustomLayout", Category: "layout", Scene: sceneBasicCustomLayout,
			JA: "`layout.Delegate` のアルゴリズムで子を配置する。",
			EN: "Arranges its children with the algorithm of a `layout.Delegate`.",
		},
		{
			Name: "basic.LayoutBuilder", Category: "layout", Scene: sceneBasicLayoutBuilder,
			JA: "親から渡された制約で中身を構築する。幅が狭いときは縦に、広いときは横に並べるといった、使える大きさに応じた切り替えに使う。中身を構築し直すのは、制約が変わったときと、親が構築し直したときだけである。",
			EN: "Builds its content with the constraints its parent gives it, to change with the room it has, such as one pane above the other when narrow and side by side when wide. The content is built again only when the constraints change or the parent rebuilds.",
		},
		{
			Name: "basic.AspectRatio", Category: "layout", Scene: sceneBasicAspectRatio,
			JA: "子を `Ratio`（幅 ÷ 高さ）の比の大きさにする。使える幅いっぱいに広がり、高さが足りなければ高さに合わせる。tight な制約の下では比が効かないため、`Center` の中に置く。",
			EN: "Sizes its child to a width over height of `Ratio`. It takes all the width it may, or all the height where that is too tall. Tight constraints leave no room for the ratio, so put it in a `Center`.",
		},

		// Effects and painting.
		{
			Name: "basic.Opacity", Category: "effects", Scene: sceneBasicOpacity,
			JA: "子を一様な透明度で描く。0 のときは、ポインタ、フォーカス、支援技術の対象からも外れる。",
			EN: "Paints its child with a uniform transparency. At 0 the pointer, the focus and assistive technology skip it.",
		},
		{
			Name: "basic.Blend", Category: "effects", Scene: sceneBasicBlend,
			JA: "子を 1 枚の画像として、下に描かれたものと合成する。CSS の mix-blend-mode と同じ働きをする。",
			EN: "Combines its child as one picture with what is below, as mix-blend-mode does in CSS.",
		},
		{
			Name: "basic.BackdropFilter", Category: "effects", Scene: sceneBasicBackdropFilter,
			JA: "自分の下に描かれたものをぼかしてから、子を描く。すりガラスのような面に使う。",
			EN: "Blurs what is painted below its box, then paints its child: frosted glass.",
		},
		{
			Name: "basic.ImageFiltered", Category: "effects", Scene: sceneBasicImageFiltered,
			JA: "子をぼかして描く。CSS の filter: blur() と同じ働きをする。",
			EN: "Draws its child blurred, as filter: blur() does in CSS.",
		},
		{
			Name: "basic.ShaderMask", Category: "effects", Scene: sceneBasicShaderMask,
			JA: "子を独自のシェーダーに通して描く。きらめき、波紋、色調の変更などに使う。",
			EN: "Draws its child through a custom shader: shimmers, ripples, tints.",
		},
		{
			Name: "basic.Transform", Category: "effects", Scene: sceneBasicTransform,
			JA: "子をアフィン変換で回転、傾斜、伸縮させる。ポインタと支援技術も、変換後の形に従う。",
			EN: "Turns, slants or stretches its child with an affine transform. The pointer and assistive technology follow the picture.",
		},
		{
			Name: "basic.Translate", Category: "effects", Scene: sceneBasicTranslate,
			JA: "子を、自分の大きさに対する割合だけずらして描く。遷移に使う軽量な移動。",
			EN: "Paints its child moved by a fraction of its size: the cheap move of a transition.",
		},
		{
			Name: "basic.ClipRRect", Also: []string{"basic.ClipRect"}, Category: "effects", Scene: sceneBasicClipRRect,
			JA: "子の描画を自分の箱の中に切り取る。`ClipRRect` は角を丸め、半径が 0 ならテーマのカードの角になる。ポインタも切り取った形の中でしか子に届かない。",
			EN: "Cuts what its child paints to its box; `ClipRRect` rounds the corners, to the theme's card corners when Radius is 0. The pointer reaches the child only inside the cut shape.",
		},
		{
			Name: "basic.Scale", Category: "effects", Scene: sceneBasicScale,
			JA: "`Origin` を中心に、子を拡大縮小して描く。遷移に使う軽量な拡大。",
			EN: "Paints its child scaled around `Origin`: the cheap zoom of a transition.",
		},
		{
			Name: "control.CustomPaint", Category: "effects", Scene: sceneControlCustomPaint,
			JA: "`layout.Painter` で自由に描画する。グラフやメーターなどに使う。",
			EN: "Paints with a `layout.Painter`, for charts, meters and other shapes no widget offers.",
		},
		{
			Name: "basic.RepaintBoundary", Category: "effects", Scene: sceneBasicRepaintBoundary,
			JA: "子の描画を周囲から切り離し、片方が変化しても、もう片方を描き直さないようにする。",
			EN: "Keeps the picture of its child apart, so a change on one side does not repaint the other.",
		},
		{
			Name: "interact.FocusRing", Category: "effects", Scene: sceneInteractFocusRing,
			JA: "標準のコントロールと同じ、淡い 3px のフォーカスリングを描く。独自のコントロールに使う。",
			EN: "Draws the pale 3px focus ring of the standard controls, for controls of your own.",
		},

		// Animation.
		{
			Name: "basic.AnimatedContainer", Category: "animation", Scene: sceneBasicAnimatedContainer,
			JA: "値が変わると、色、大きさ、余白、角丸、枠線、影を補間しながら変化する `Container`。",
			EN: "A `Container` that moves to new colors, sizes, padding, corners, borders and shadows over time.",
		},
		{
			Name: "basic.AnimatedOpacity", Category: "animation", Scene: sceneBasicAnimatedOpacity,
			JA: "新しい透明度へフェードする `Opacity`。",
			EN: "An `Opacity` that fades to a new value.",
		},
		{
			Name: "basic.AnimatedAlign", Category: "animation", Scene: sceneBasicAnimatedAlign,
			JA: "新しい位置へ移動する `Align`。",
			EN: "An `Align` that moves to a new alignment.",
		},
		{
			Name: "basic.AnimatedPositioned", Category: "animation", Scene: sceneBasicAnimatedPositioned,
			JA: "`Stack` の中で新しい位置へ移動する `Positioned`。",
			EN: "A `Positioned` that moves to a new place in its `Stack`.",
		},
		{
			Name: "basic.AnimatedSwitcher", Also: []string{"basic.SwitcherTransition"}, Category: "animation", Scene: sceneBasicAnimatedSwitcher,
			JA: "子が替わると、クロスフェードやスライドで切り替える。同じ型の子は `core.Keyed` で区別する。",
			EN: "Crossfades or slides from an old child to a new one. Tell children of one type apart with `core.Keyed`.",
		},
		{
			Name: "nav.Hero", Category: "animation", Scene: sceneNavHero, Show: sceneNavNavigator,
			Act: click(0, "Reading list", 0, primary),
			JA:  "`Navigator` がページを切り替えるとき、同じ `Tag` を付けた子が、元の位置から新しい位置へ飛ぶように移動する。画像は遷移後のページを写したもの。",
			EN:  "When a `Navigator` changes pages, the child with the same `Tag` flies from one place to the other. The picture shows the page after landing.",
		},

		// Input handling and accessibility.
		{
			Name: "interact.GestureDetector", Category: "behavior", Scene: sceneInteractGestureDetector, NoPicture: true,
			JA: "子へのタップ、ダブルタップ、右クリック、長押し、ドラッグ（開始・移動・終了）、ピンチを通知する。少し動いただけの押下はタップのままで、ドラッグにはならない。`OnTapDownAt`、`OnTapAt`、`OnSecondaryTapAt` は、位置とともに押していた修飾キーも渡すので、Shift や Ctrl を押しながらのクリックも扱える。",
			EN: "Reports taps, double taps, secondary taps, long presses, drags (start, update, end) and pinches on its child. A press that moves only a little stays a tap. `OnTapDownAt`, `OnTapAt` and `OnSecondaryTapAt` tell the modifier keys held along with the place, for a click with Shift or Ctrl.",
		},
		{
			Name: "interact.MouseRegion", Category: "behavior", Scene: sceneInteractMouseRegion, NoPicture: true,
			JA: "ポインタの出入りを通知し、子の上でのカーソルの形を決める。",
			EN: "Reports the pointer entering and leaving, and sets the cursor over its child.",
		},
		{
			Name: "interact.Listener", Category: "behavior", Scene: sceneInteractListener, NoPicture: true,
			JA: "子の上で起きたポインタのイベントをすべて渡す。ペンの筆圧も届くため、手描きの入力に使える。",
			EN: "Hands every pointer event over its child to a function, with the pressure of a pen, for drawing.",
		},
		{
			Name: "interact.Focus", Category: "behavior", Scene: sceneInteractFocus, NoPicture: true,
			JA: "子がキーボード入力を受け取れるようにする。Tab キーは描画順にフォーカスを移す。",
			EN: "Lets its child take the keyboard. Tab moves in the order of painting.",
		},
		{
			Name: "basic.IgnorePointer", Category: "behavior", Scene: sceneBasicIgnorePointer, NoPicture: true,
			JA: "ポインタを子に当てず、その下にあるものへ通す。",
			EN: "Lets the pointer pass through its child to what is below.",
		},
		{
			Name: "interact.Draggable", Category: "behavior", Scene: sceneInteractDraggable, NoPicture: true,
			JA: "子からデータをドラッグで持ち出す。同じウィンドウの中、別のウィンドウ、ほかのアプリへ渡せる。",
			EN: "Drags data out of its child: to a `DragTarget`, another window or another program.",
		},
		{
			Name: "interact.DragTarget", Category: "behavior", Scene: sceneInteractDragTarget, NoPicture: true,
			JA: "子にドロップされたテキストやファイルを受け取る。エクスプローラーやブラウザーなど、ほかのアプリからのドロップも受け付ける。",
			EN: "Takes text or files dropped on its child, from this window or from programs such as Explorer or a browser.",
		},
		{
			Name: "interact.Semantics", Category: "behavior", Scene: sceneInteractSemantics, NoPicture: true,
			JA: "子の意味を支援技術に伝え、スクリーンリーダーからの操作を受け付ける。標準のコントロールは、これを自前で備えている。",
			EN: "Describes its child to assistive technology and does what a screen reader asks. The standard controls have their own.",
		},
		{
			Name: "interact.ExcludeSemantics", Category: "behavior", Scene: sceneInteractExcludeSemantics, NoPicture: true,
			JA: "子を支援技術から隠す。Web の aria-hidden に当たる。アバターの頭文字のような、飾りにすぎないテキストに使う。画面への描画とポインタの扱いは変わらない。",
			EN: "Hides its child from assistive technology, as aria-hidden does, for text that only decorates, such as the initial in an avatar. It is still drawn and still takes the pointer.",
		},
		{
			Name: "interact.FocusTraversalOrder", Category: "behavior", Scene: sceneInteractFocusTraversalOrder, NoPicture: true,
			JA: "HTML の tabindex のように、Tab キーで移動する順番を決める。",
			EN: "Sets where the controls below it come in the order of Tab, as tabindex does in HTML.",
		},

		// Tree services.
		{
			Name: "basic.Builder", Category: "infrastructure", Scene: sceneBasicBuilder,
			JA: "その位置の `BuildContext` で中身を構築する。上位にあるテーマなどを読み取るのに使う。",
			EN: "Builds its content with the `BuildContext` of its place, to read what is above, such as the theme.",
		},
		{
			Name: "basic.Theme", Category: "infrastructure", Scene: sceneBasicTheme,
			JA: "配下のウィジェットの見た目を決める。ウィンドウの一部だけアクセントの色を変えたいときに使う。",
			EN: "Gives the widgets below it their look, for a part of the window with another accent.",
		},
		{
			Name: "basic.MediaQuery", Also: []string{"basic.MediaQueryData"}, Category: "infrastructure", Scene: sceneBasicMediaQuery,
			JA: "ウィンドウの明るさ、表示倍率、動きを減らす設定、ロケール、内容の領域の大きさを配下に伝える。大きさは `basic.WindowSizeOf` で読むと、ウィンドウの大きさが変わったときにそのウィジェットだけが再構築される。この例ではロケールを英国にして、週を月曜から始めている。",
			EN: "Tells the widgets below about the window: brightness, scale, reduced motion, locale and the size of its content area. A widget that reads the size with `basic.WindowSizeOf` is rebuilt when the window is resized, and only it. Here the British locale starts the week on Monday.",
		},
		{
			Name: "basic.Localizations", Also: []string{"basic.Strings"}, Category: "infrastructure", Scene: sceneBasicLocalizations, NoPicture: true,
			JA: "標準のウィジェットが使う文言の表を渡す。日本語と英語は同梱されており、ほかの言語は `Strings` を追加する。",
			EN: "Hands the standard widgets the words of their locale. Japanese and English are bundled; add `Strings` for others.",
		},
		{
			Name: "overlay.Overlay", Also: []string{"overlay.Entry", "overlay.Host"}, Category: "infrastructure", Scene: sceneOverlayOverlay, NoPicture: true,
			JA: "子の上に項目を重ねて表示する場所。ダイアログやメニューもここに表示される。",
			EN: "A place to show entries on top of its child. Dialogs and menus show there too.",
		},
	}
}
