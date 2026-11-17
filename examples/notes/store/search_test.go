package store

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bankai-tensei/gowaku/text"
)

// hitString writes a hit as file:line:offset before[match]after.
func hitString(dir string, h Hit) string {
	rel, err := filepath.Rel(dir, h.Note.Path)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s:%d:%d %s[%s]%s", filepath.ToSlash(rel), h.Line, h.Offset, h.Before, h.Match, h.After)
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "old.txt", "Go言語\nhello GO world\n", 1)
	writeNote(t, dir, "new.txt", "題\nＧｏ は全角\n二つ go go\n", 5)
	writeNote(t, dir, "kana.txt", "ｶﾀｶﾅ と カタカナ\n", 3)
	writeNote(t, dir, "sub/case.txt", "ÉCOLE and école\n", 2)
	l := Library{Dir: dir}

	tests := []struct {
		query string
		limit int
		want  []string
	}{
		{"go", 10, []string{
			"new.txt:1:4 [Ｇｏ] は全角",
			"new.txt:2:28 二つ [go] go",
			"new.txt:2:31 二つ go [go]",
			"old.txt:0:0 [Go]言語",
			"old.txt:1:15 hello [GO] world",
		}},
		{"ＧＯ", 2, []string{
			"new.txt:1:4 [Ｇｏ] は全角",
			"new.txt:2:28 二つ [go] go",
		}},
		{"カタカナ", 10, []string{
			"kana.txt:0:0 [ｶﾀｶﾅ] と カタカナ",
			"kana.txt:0:17 ｶﾀｶﾅ と [カタカナ]",
		}},
		{"ｶﾀ", 10, []string{
			"kana.txt:0:0 [ｶﾀ]ｶﾅ と カタカナ",
			"kana.txt:0:17 ｶﾀｶﾅ と [カタ]カナ",
		}},
		{"École", 10, []string{
			"sub/case.txt:0:0 [ÉCOLE] and école",
			"sub/case.txt:0:11 ÉCOLE and [école]",
		}},
		{"o w", 10, []string{"old.txt:1:16 hello G[O w]orld"}},
		{"\n", 1, []string{"new.txt:0:3 題[\n]Ｇｏ は全角"}},
		{"none", 10, nil},
		{"", 10, nil},
		{"go", 0, nil},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q/%d", tt.query, tt.limit), func(t *testing.T) {
			hits, err := l.Search(tt.query, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, h := range hits {
				got = append(got, hitString(dir, h))
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("Search =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestSearchOffsetsPointAtMatch(t *testing.T) {
	dir := t.TempDir()
	text := "Ａｂｃ abc ABC\r\nｱｲｳ アイウ\r\n終わりのabc"
	path := writeNote(t, dir, "a.txt", bom+text, 0)
	d, err := ReadDoc(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"abc", "アイウ", "わりのａ"} {
		hits, err := Library{Dir: dir}.Search(q, 100)
		if err != nil || len(hits) == 0 {
			t.Fatalf("%q: %v, %v", q, hits, err)
		}
		for _, h := range hits {
			if !strings.HasPrefix(d.Text[h.Offset:], h.Match) {
				t.Errorf("%q: text at %d is %q, match %q", q, h.Offset, d.Text[h.Offset:], h.Match)
			}
			if got := strings.Count(d.Text[:h.Offset], "\n"); got != h.Line {
				t.Errorf("%q: line %d, want %d", q, h.Line, got)
			}
		}
	}
}

func TestSearchCutsSnippets(t *testing.T) {
	dir := t.TempDir()
	left, right := strings.Repeat("左", 50), strings.Repeat("右", 50)
	writeNote(t, dir, "a.txt", "前の行\n"+left+"目印"+right+"\n次の行", 0)
	hits, err := Library{Dir: dir}.Search("目印", 1)
	if err != nil || len(hits) != 1 {
		t.Fatalf("%v, %v", hits, err)
	}
	h := hits[0]
	if h.Before != strings.Repeat("左", 30) || h.After != strings.Repeat("右", 30) || h.Line != 1 {
		t.Fatalf("hit = %+v", h)
	}
	if h.Offset != len("前の行\n"+left) {
		t.Fatalf("Offset = %d", h.Offset)
	}
}

func TestSearchRuby(t *testing.T) {
	skipWithoutAozora(t)
	dir := t.TempDir()
	text := "冒頭\n昔の｜漢字《かんじ》と漢字《かんじ》を書く\n※《漢字》は括弧\nかんじ"
	path := writeNote(t, dir, "a.txt", text, 0)
	d, err := ReadDoc(path)
	if err != nil {
		t.Fatal(err)
	}
	l := Library{Dir: dir}

	tests := []struct {
		query string
		want  []string
	}{
		// The base matches, the ruby does not, and the snippets are plain.
		{"漢字", []string{
			"a.txt:1:16 昔の[漢字]と漢字を書く",
			"a.txt:1:40 昔の漢字と[漢字]を書く",
			"a.txt:2:77 《[漢字]》は括弧",
		}},
		{"かんじ", []string{"a.txt:3:96 [かんじ]"}},
		// A match across the end of a ruby.
		{"字と漢", []string{"a.txt:1:19 昔の漢[字と漢]字を書く"}},
		{"《漢", []string{"a.txt:2:74 [《漢]字》は括弧"}},
	}
	for _, tt := range tests {
		hits, err := l.Search(tt.query, 10)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, h := range hits {
			got = append(got, hitString(dir, h))
			if !strings.HasPrefix(d.Text[h.Offset:], string([]rune(h.Match)[:1])) {
				t.Errorf("%q: text at %d is %q", tt.query, h.Offset, d.Text[h.Offset:])
			}
		}
		if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
			t.Errorf("Search(%q) =\n%s\nwant\n%s", tt.query, strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
		}
	}
}

// TestIndexMarks checks the index of a text with ruby against marks written
// by hand, so that the mapping is tested before text/aozora finds marks.
func TestIndexMarks(t *testing.T) {
	raw := "昔の｜漢字《かんじ》と漢字《かんじ》を書く\n※《漢字》"
	marks := []text.RubyMark{
		{Start: 6, BaseStart: 9, BaseEnd: 15, RubyStart: 18, RubyEnd: 27, End: 30},
		{Start: 33, BaseStart: 33, BaseEnd: 39, RubyStart: 42, RubyEnd: 51, End: 54},
	}
	x := indexMarks(raw, marks)
	if x.plain != "昔の漢字と漢字を書く\n《漢字》" {
		t.Fatalf("plain = %q", x.plain)
	}
	tests := []struct {
		query string
		want  []string
	}{
		{"漢字", []string{"0:9 昔の[漢字]と漢字を書く", "0:33 昔の漢字と[漢字]を書く", "1:70 《[漢字]》"}},
		{"字と漢", []string{"0:12 昔の漢[字と漢]字を書く"}},
		{"昔の漢", []string{"0:0 [昔の漢]字と漢字を書く"}},
		{"字を", []string{"0:36 昔の漢字と漢[字を]書く"}},
		{"《", []string{"1:67 [《]漢字》"}},
		{"かんじ", nil},
		{"｜", nil},
	}
	for _, tt := range tests {
		var got []string
		for _, h := range x.search(Note{}, fold(tt.query), nil, 10) {
			got = append(got, fmt.Sprintf("%d:%d %s[%s]%s", h.Line, h.Offset, h.Before, h.Match, h.After))
		}
		if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
			t.Errorf("search(%q) =\n%s\nwant\n%s", tt.query, strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
		}
	}
}

// TestIndexMapping checks the spans of the index against a slow mapping, on
// text with folds that change the length of runes.
func TestIndexMapping(t *testing.T) {
	alphabet := []rune("aAｚＺ0０ｶカあ漢\nÉé ")
	r := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		var b strings.Builder
		for range r.IntN(40) {
			b.WriteRune(alphabet[r.IntN(len(alphabet))])
		}
		raw := b.String()
		x := makeIndex(raw)
		f := 0
		for i, c := range raw {
			p, rr := x.at(f)
			if rr != i || p != i {
				t.Fatalf("%q: folded %d maps to %d, %d, want %d", raw, f, p, rr, i)
			}
			f += len(string(foldRune(c)))
		}
		if p, rr := x.at(len(x.folded)); p != len(raw) || rr != len(raw) {
			t.Fatalf("%q: end maps to %d, %d", raw, p, rr)
		}
	}
}

func TestSearchConcurrent(t *testing.T) {
	dir := t.TempDir()
	for i := range 50 {
		writeNote(t, dir, fmt.Sprintf("n%02d.txt", i), fmt.Sprintf("note %d\nkeyword here\n", i), i)
	}
	l := Library{Dir: dir}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5 {
				hits, err := l.Search("KEYWORD", 1000)
				if err != nil || len(hits) != 50 {
					t.Errorf("Search = %d hits, %v", len(hits), err)
					return
				}
				if _, err := l.Notes(); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
}
