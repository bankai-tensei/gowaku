package field

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
)

func TestFuzzy_Match(t *testing.T) {
	cases := []struct {
		name, query, label string
		ok                 bool
		ranges             []int
	}{
		{"空の問い合わせ", "", "何でも", true, nil},
		{"空白だけ", " 　", "Open", true, nil},
		{"前方一致", "op", "Open File", true, []int{0, 2}},
		{"語頭を拾う", "of", "Open File", true, []int{0, 1, 5, 6}},
		{"大文字小文字", "OPEN", "open", true, []int{0, 4}},
		{"順序が違う", "fo", "of", false, nil},
		{"含まない", "xyz", "Open File", false, nil},
		{"問い合わせが長い", "abcd", "abc", false, nil},
		{"漢字", "保存", "名前を付けて保存", true, []int{18, 24}},
		{"飛び飛びの日本語", "新ノ", "新しいノート", true, []int{0, 3, 9, 12}},
		{"全角英数字", "ａｂｃ", "ABC", true, []int{0, 3}},
		{"半角英数字で全角を", "abc", "ＡＢＣ", true, []int{0, 9}},
		{"半角カナ", "カナ", "ｶﾅ", true, []int{0, 6}},
		{"空白を無視", "o f", "Open File", true, []int{0, 1, 5, 6}},
		{"ひらがなとカタカナは別", "かな", "カナ", false, nil},
	}
	var f Fuzzy
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f.SetQuery(c.query)
			_, ok := f.Match(c.label)
			if ok != c.ok {
				t.Fatalf("Match(%q, %q) ok = %v, want %v", c.query, c.label, ok, c.ok)
			}
			if ok && !slices.Equal(f.Ranges(), c.ranges) && (len(f.Ranges()) != 0 || len(c.ranges) != 0) {
				t.Errorf("Ranges = %v, want %v", f.Ranges(), c.ranges)
			}
		})
	}
}

func TestFuzzy_Ranking(t *testing.T) {
	cases := []struct {
		name, query, better, worse string
	}{
		{"完全一致が勝つ", "保存", "保存", "名前を付けて保存"},
		{"語頭が勝つ", "ns", "New Session", "Constants"},
		{"連続が勝つ", "file", "File", "Fix Inline Lens Error"},
		{"文字種の境目は語頭", "開", "ファイルを開く", "公開する"},
		{"キャメルケース", "gc", "getConfig", "magic"},
		{"前に近い方が勝つ", "x", "x axis", "the x"},
	}
	var f Fuzzy
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f.SetQuery(c.query)
			a, okA := f.Match(c.better)
			b, okB := f.Match(c.worse)
			if !okA || !okB {
				t.Fatalf("both should match: %v %v", okA, okB)
			}
			if a <= b {
				t.Errorf("%q scores %d, %q %d; want the first higher", c.better, a, c.worse, b)
			}
		})
	}
}

func TestFuzzy_PicksTheBestPlacement(t *testing.T) {
	var f Fuzzy
	// The greedy first 'f' and 'i' would make "f-i" across two words; the best
	// placement keeps "fi" together at the start of the second word.
	f.SetQuery("fil")
	if _, ok := f.Match("Of Find Files"); !ok {
		t.Fatal("no match")
	}
	if got, want := f.Ranges(), []int{8, 11}; !slices.Equal(got, want) {
		t.Errorf("Ranges = %v, want %v", got, want)
	}
}

func TestFuzzy_LongLabels(t *testing.T) {
	var f Fuzzy
	long := strings.Repeat("あ", FuzzyMaxLabel) + "z"
	f.SetQuery("z")
	if _, ok := f.Match(long); ok {
		t.Error("a rune past the limit matched")
	}
	f.SetQuery(strings.Repeat("a", FuzzyMaxQuery+1))
	if _, ok := f.Match(strings.Repeat("a", 100)); ok {
		t.Error("an over-long query matched")
	}
	// A match on the last searched rune ends there, not at the end of the
	// label.
	long = strings.Repeat("あ", FuzzyMaxLabel-1) + "zzz"
	f.SetQuery("z")
	if _, ok := f.Match(long); !ok {
		t.Fatal("the last searched rune did not match")
	}
	at := 3 * (FuzzyMaxLabel - 1)
	if got, want := f.Ranges(), []int{at, at + 1}; !slices.Equal(got, want) {
		t.Errorf("Ranges = %v, want %v", got, want)
	}
}

func TestFuzzySpans_SkipsRangesOutsideTheLabel(t *testing.T) {
	base, hit := text.Style{Weight: 400}, text.Span{Text: "ignored", Style: text.Style{Weight: 600}, Background: geom.Hex(0xffe066)}
	spans := FuzzySpans(nil, "abc", []int{1, 2, 5, 9}, base, hit)
	var texts []string
	for _, s := range spans {
		texts = append(texts, s.Text)
	}
	if !slices.Equal(texts, []string{"a", "b", "c"}) || spans[1].Style != hit.Style || spans[1].Background != hit.Background ||
		spans[0].Background != (geom.Color{}) {
		t.Errorf("spans %+v", spans)
	}
	if got := FuzzySpans(nil, "", nil, base, hit); len(got) != 1 || got[0].Text != "" {
		t.Errorf("empty label: %+v", got)
	}
	if got := FuzzyHighlight(text.Style{Weight: 700}, geom.Color{}); got.Style.Weight != 700 {
		t.Errorf("a bold label marks its matches in weight %v", got.Style.Weight)
	}
}

func TestFuzzySpans_DoesNotAllocate(t *testing.T) {
	var f Fuzzy
	f.SetQuery("of")
	label := "Open File"
	f.Match(label)
	ranges := f.Ranges()
	spans := make([]text.Span, 0, 8)
	base := text.Style{}
	hit := FuzzyHighlight(base, geom.Color{B: 1, A: 1})
	allocs := testing.AllocsPerRun(100, func() {
		spans = FuzzySpans(spans[:0], label, ranges, base, hit)
	})
	if allocs != 0 {
		t.Errorf("FuzzySpans allocates %v times per run", allocs)
	}
}

func ExampleFuzzy() {
	var f Fuzzy
	f.SetQuery("of")
	for _, label := range []string{"Open File", "Copy", "ファイルを開く"} {
		if score, ok := f.Match(label); ok {
			r := f.Ranges()
			fmt.Println(label, score, r, label[r[0]:r[1]], label[r[2]:r[3]])
		}
	}
	// Output:
	// Open File 52 [0 1 5 6] O F
}

// TestFuzzy_RepeatedRunes matches queries whose runes repeat, so that a later
// query rune also matches the first column of the label.
func TestFuzzy_RepeatedRunes(t *testing.T) {
	cases := []struct {
		query, label string
		ranges       []int
	}{
		{"aa", "Add All", []int{0, 1, 4, 5}},
		{"aa", "aa", []int{0, 2}},
		{"aaa", "a a a", []int{0, 1, 2, 3, 4, 5}},
		{"ll", "All", []int{1, 3}},
		{"ss", "Save as", []int{0, 1, 6, 7}},
	}
	var f Fuzzy
	for _, c := range cases {
		f.SetQuery(c.query)
		if _, ok := f.Match(c.label); !ok {
			t.Errorf("Match(%q, %q) = no match, want a match", c.query, c.label)
			continue
		}
		if !slices.Equal(f.Ranges(), c.ranges) {
			t.Errorf("Match(%q, %q): Ranges = %v, want %v", c.query, c.label, f.Ranges(), c.ranges)
		}
	}
}

// TestFuzzy_RandomLabels matches random queries against random labels over a
// small alphabet, where repeated runes are common, and checks that a label
// matches exactly when the query is a subsequence of it and that the ranges
// are ordered and inside the label.
func TestFuzzy_RandomLabels(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	const alphabet = "aAb -"
	word := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[r.IntN(len(alphabet))]
		}
		return string(b)
	}
	var f Fuzzy
	for range 5000 {
		query, label := word(1+r.IntN(4)), word(r.IntN(10))
		f.SetQuery(query)
		_, ok := f.Match(label)
		if want := isSubsequence(query, label); ok != want {
			t.Fatalf("Match(%q, %q) ok = %v, want %v", query, label, ok, want)
		}
		if !ok {
			continue
		}
		rs, prev := f.Ranges(), 0
		for i := 0; i+1 < len(rs); i += 2 {
			if rs[i] < prev || rs[i] >= rs[i+1] || rs[i+1] > len(label) {
				t.Fatalf("Match(%q, %q): Ranges = %v, want ordered ranges inside the label", query, label, rs)
			}
			prev = rs[i+1]
		}
	}
}

// isSubsequence reports whether the bytes of query other than spaces appear
// in label in order, ignoring ASCII case.
func isSubsequence(query, label string) bool {
	q := strings.ToLower(strings.ReplaceAll(query, " ", ""))
	l := strings.ToLower(label)
	i := 0
	for j := 0; j < len(l) && i < len(q); j++ {
		if l[j] == q[i] {
			i++
		}
	}
	return i == len(q)
}

func BenchmarkFuzzy_100Labels(b *testing.B) {
	labels := []string{
		"新規作成", "開く…", "名前を付けて保存", "上書き保存", "閉じる", "元に戻す", "やり直し",
		"切り取り", "コピー", "貼り付け", "すべて選択", "検索", "置換", "縦書き", "横書き",
		"文字を大きく", "文字を小さく", "明朝体", "ゴシック体", "コマンドパレット",
		"Open File", "Save As", "Toggle Sidebar", "Go to Line", "Format Document",
	}
	for len(labels) < 100 {
		labels = append(labels, labels[len(labels)%25]+"（複製）")
	}
	var f Fuzzy
	f.SetQuery("保存")
	for _, l := range labels {
		f.Match(l)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, l := range labels {
			f.Match(l)
		}
	}
}

func TestFuzzy_DoesNotAllocate(t *testing.T) {
	var f Fuzzy
	f.SetQuery("ファイ")
	labels := []string{"ファイルを開く", "Open File", "ファイル名を変更", "印刷"}
	for _, l := range labels {
		f.Match(l)
	}
	allocs := testing.AllocsPerRun(100, func() {
		for _, l := range labels {
			f.Match(l)
		}
	})
	if allocs != 0 {
		t.Errorf("Match allocates %v times per run", allocs)
	}
}
