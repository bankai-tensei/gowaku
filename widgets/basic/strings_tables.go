package basic

import (
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// The bundled tables are never written to; they are variables only because Go
// has no constant structs (CODING_STYLE §3.2). EnglishStrings and
// JapaneseStrings hand out copies.

// englishStrings is complete, and the fallback of every word another table
// leaves out. Its formats are nil: the methods of Strings write English.
var englishTable = Strings{
	Language: language.English,

	Months: [12]string{"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"},
	ShortMonths:    [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	Weekdays:       [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	ShortWeekdays:  [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	NarrowWeekdays: [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
	Today:          "Today", Yesterday: "Yesterday", Clear: "Clear", ChooseDate: "Choose date",
	PreviousMonth: "Previous month", NextMonth: "Next month",
	ChooseMonth: "Choose month", ChooseYear: "Choose year",
	PreviousYear: "Previous year", NextYear: "Next year",
	PreviousYears: "Previous years", NextYears: "Next years",
	Marked: "has events",

	AM: "AM", PM: "PM",
	Hour: "Hour", Minute: "Minute", Second: "Second", DayPeriod: "AM/PM",

	Close: "Close", Search: "Search", NoResults: "No results", Required: "Required",
	RequiredChoice: "Choose one", RequiredCheck: "Check this box",
	Minimize: "Minimize", Maximize: "Maximize", Restore: "Restore",
	Remove: "Remove", More: "More",

	// SplitPane, PropertyGrid, Breadcrumb
	PaneDivider: "Pane divider", FilterProperties: "Filter properties",
	Breadcrumb: "Breadcrumb", CurrentLocation: "Current location", HiddenPath: "Hidden locations",
	Steps: "Steps", StepCompleted: "Completed", CurrentStep: "Current step",

	// Tree
	Loading: "Loading…",

	// TabBar
	NewTab: "New tab", AllTabs: "All tabs", Modified: "Modified",

	// Editor
	Find: "Find", ReplaceWith: "Replace with", Replace: "Replace", ReplaceAll: "Replace all",
	MatchCase: "Match case", WholeWord: "Whole word", RegularExpression: "Regular expression",
	PreviousMatch: "Previous match", NextMatch: "Next match", BadPattern: "Invalid pattern",
	Searching: "Searching…",

	// Table
	Filter: "Filter", AllValues: "All",

	// Navigator
	Back: "Back", PageNotFound: "Page not found",

	// NumberField, RangeSlider
	InvalidNumber: "Enter a number", RangeMinimum: "Minimum", RangeMaximum: "Maximum",

	// SelectionArea
	Copy: "Copy", Copied: "Copied", SelectAll: "Select all",

	// ColorPicker
	ChooseColor: "Choose color", ColorPlane: "Saturation and brightness",
	Hue: "Hue", Saturation: "Saturation", Brightness: "Brightness", Opacity: "Opacity",
	HexCode: "Hex", Red: "Red", Green: "Green", Blue: "Blue", OriginalColor: "Original color",

	// Sparkline
	Minimum: "Minimum", Maximum: "Maximum", Latest: "Latest",

	// InfoBar, StatusBar, CommandPalette
	Information: "Information", Success: "Success", Warning: "Warning", Error: "Error",
	StatusBar: "Status bar", CommandPalette: "Command palette", SearchCommands: "Search commands",

	// DockLayout
	AutoHide: "Auto-hide", HiddenPanels: "Hidden panels",
}

// japaneseWeekdays and japaneseShortWeekdays stand alone so that the formats
// of the table can use them.
var (
	japaneseWeekdays      = [7]string{"日曜日", "月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日"}
	japaneseShortWeekdays = [7]string{"日", "月", "火", "水", "木", "金", "土"}
)

// japaneseMonthDay writes 11月16日, without spaces: the typesetting puts the
// room between the digits and the kanji (JLReq).
func japaneseMonthDay(t time.Time) string {
	return kit.Itoa(int(t.Month())) + "月" + kit.Itoa(t.Day()) + "日"
}

var japaneseTable = Strings{
	Language: language.Japanese,

	Months:         [12]string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	ShortMonths:    [12]string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	Weekdays:       japaneseWeekdays,
	ShortWeekdays:  japaneseShortWeekdays,
	NarrowWeekdays: japaneseShortWeekdays,
	Today:          "今日", Yesterday: "昨日", Clear: "クリア", ChooseDate: "日付を選択",
	PreviousMonth: "前の月", NextMonth: "次の月",
	ChooseMonth: "月を選択", ChooseYear: "年を選択",
	PreviousYear: "前の年", NextYear: "次の年",
	PreviousYears: "前の10年", NextYears: "次の10年",
	Marked: "予定あり",

	AM: "午前", PM: "午後", Hour24: true, PeriodFirst: true,
	Hour: "時", Minute: "分", Second: "秒", DayPeriod: "午前/午後",

	Close: "閉じる", Search: "検索", NoResults: "見つかりません", Required: "入力してください",
	RequiredChoice: "選択してください", RequiredCheck: "チェックしてください",
	Minimize: "最小化", Maximize: "最大化", Restore: "元に戻す",
	Remove: "削除", More: "その他",

	// SplitPane, PropertyGrid, Breadcrumb
	PaneDivider: "ペインの区切り", FilterProperties: "プロパティを検索",
	Breadcrumb: "パンくずリスト", CurrentLocation: "現在の場所", HiddenPath: "隠れている階層",
	Steps: "手順", StepCompleted: "完了", CurrentStep: "現在の手順",

	// Tree
	Loading: "読み込み中…",

	// TabBar
	NewTab: "新しいタブ", AllTabs: "すべてのタブ", Modified: "未保存",

	// Editor
	Find: "検索", ReplaceWith: "置換後の文字列", Replace: "置換", ReplaceAll: "すべて置換",
	MatchCase: "大文字と小文字を区別", WholeWord: "単語単位", RegularExpression: "正規表現",
	PreviousMatch: "前を検索", NextMatch: "次を検索", BadPattern: "パターンが正しくありません",
	Searching: "検索中…",

	// Table
	Filter: "絞り込み", AllValues: "すべて",

	// Navigator
	Back: "戻る", PageNotFound: "ページが見つかりません",

	// NumberField, RangeSlider
	InvalidNumber: "数値を入力してください", RangeMinimum: "最小", RangeMaximum: "最大",

	// SelectionArea
	Copy: "コピー", Copied: "コピーしました", SelectAll: "すべて選択",

	// ColorPicker
	ChooseColor: "色を選択", ColorPlane: "彩度と明度",
	Hue: "色相", Saturation: "彩度", Brightness: "明度", Opacity: "不透明度",
	HexCode: "16進コード", Red: "赤", Green: "緑", Blue: "青", OriginalColor: "元の色",

	// Sparkline
	Minimum: "最小", Maximum: "最大", Latest: "最新",

	// InfoBar, StatusBar, CommandPalette
	Information: "情報", Success: "成功", Warning: "警告", Error: "エラー",
	StatusBar: "ステータスバー", CommandPalette: "コマンドパレット", SearchCommands: "コマンドを検索",

	// DockLayout
	AutoHide: "自動的に隠す", HiddenPanels: "隠したパネル",

	FormatMonthYear: func(m time.Month, year int) string { return kit.Itoa(year) + "年" + kit.Itoa(int(m)) + "月" },
	FormatYear:      func(year int) string { return kit.Itoa(year) + "年" },
	FormatDate: func(t time.Time) string {
		return kit.Itoa(t.Year()) + "/" + kit.Pad2(int(t.Month())) + "/" + kit.Pad2(t.Day())
	},
	FormatShortDate: func(t time.Time) string {
		return japaneseMonthDay(t) + "（" + japaneseShortWeekdays[t.Weekday()] + "）"
	},
	FormatMonthDay: japaneseMonthDay,
	FormatSpokenDate: func(t time.Time) string {
		return kit.Itoa(t.Year()) + "年" + kit.Itoa(int(t.Month())) + "月" + kit.Itoa(t.Day()) + "日 " +
			japaneseWeekdays[t.Weekday()]
	},
	FormatTime: func(t time.Time) string { return kit.Itoa(t.Hour()) + ":" + kit.Pad2(t.Minute()) },
	FormatMatchCount: func(k, n int) string {
		if k > 0 {
			return kit.Itoa(k) + " / " + kit.Itoa(n) + " 件"
		}
		return kit.Itoa(n) + " 件"
	},
	FormatRemoveNamed: func(label string) string { return label + "を削除" },
	FormatCloseNamed:  func(title string) string { return title + "を閉じる" },
	FormatMovedTo: func(label string, k, n int) string {
		place := kit.Itoa(n) + " 件中 " + kit.Itoa(k) + " 番目に移動しました"
		if label == "" {
			return place
		}
		return label + "を " + place
	},
}
