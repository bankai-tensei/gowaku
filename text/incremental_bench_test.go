package text

import (
	"strings"
	"testing"
)

// bigText is about 100,000 characters: 1,000 lines of Japanese and English.
func bigText() string {
	line := "親譲りの無鉄砲で小供の時から損ばかりしている。Typesetting with gowaku, 2026. 小学校に居る時分学校の二階から飛び降りて一週間ほど腰を抜かした事がある。"
	var b strings.Builder
	for b.Len() < 0 || len([]rune(b.String())) < 100000 {
		b.WriteString(line)
		b.WriteByte('\n')
		if b.Len() > 400000 {
			break
		}
	}
	return b.String()
}

func BenchmarkKeystroke_100kChars(b *testing.B) {
	s := systemShaper(b)
	content := bigText()
	st := Style{Size: 16}
	p := s.Shape(content, st)
	l := p.Layout(600)
	mid := len(content) / 2
	mid = strings.LastIndexByte(content[:mid], '\n') + 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		edited := content[:mid] + string(rune('a'+i%26)) + content[mid:]
		p = s.Reshape(p, edited, st)
		l = p.Relayout(l, 600)
		content = edited
	}
}

func BenchmarkLayout_100kChars(b *testing.B) {
	s := systemShaper(b)
	p := s.Shape(bigText(), Style{Size: 16})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		p.Layout(600)
	}
}
