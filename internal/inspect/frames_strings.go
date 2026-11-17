//go:build gowakudebug

package inspect

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// framesStrings are the words of the frames panel that inspectStrings does
// not have. They live apart so that the panels can grow their words without
// touching each other's.
type framesStrings struct {
	graph, noFrames, paused, cpu, average, maximum, overBudget  string
	workShort                                                   string
	allocsAxis, more, moreEnd, yes, no, closedWindow, ms, frame string
	fps120, fps60                                               string
}

var framesJA = framesStrings{
	graph: "フレーム時間のグラフ", noFrames: "記録したフレームはまだありません", paused: "記録を停止中",
	workShort: "処理",
	cpu:       "CPU 合計", average: "平均", maximum: "最大", overBudget: "16.7 ms 超え",
	allocsAxis: "allocs", more: "ほか ", moreEnd: " 件", yes: "はい", no: "いいえ", closedWindow: "（閉じたウィンドウ）",
	ms: " ms", frame: "フレーム #", fps120: "120 fps", fps60: "60 fps",
}

var framesEN = framesStrings{
	graph: "Frame time graph", noFrames: "No frames recorded yet", paused: "Recording paused",
	workShort: "Work",
	cpu:       "CPU total", average: "avg", maximum: "max", overBudget: "over 16.7 ms",
	allocsAxis: "allocs", more: "+", moreEnd: " more", yes: "yes", no: "no", closedWindow: "(closed window)",
	ms: " ms", frame: "Frame #", fps120: "120 fps", fps60: "60 fps",
}

// framesStringsOf returns the words of the frames panel for the locale of the
// window.
func framesStringsOf(ctx core.BuildContext) *framesStrings {
	if base, _ := basic.LocaleOf(ctx).Base(); base == japanese {
		return &framesJA
	}
	return &framesEN
}
