//go:build gowakudebug

package inspect

import (
	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// inspectStrings are the words of the Inspector. It speaks Japanese and
// English only: it is a tool for developers, not part of an application.
type inspectStrings struct {
	toolbar, pick, bounds, repaints, pause, window, untitled, noWindow string
	trees, widgetTree, renderTree, semanticsTree                       string
	details, displayList, noSelection                                  string
	findInTree, noMatch, refresh, detached, noNode, ofAncestor         string
	showInternal, viewTrail                                            string
	drawsNothing, notLaidOut                                           string
	frames, frameDetails, noFrame                                      string
	stageNames                                                         [StageCount]string
	gpu, allocs, rebuilt, laidOut, recorded, repainted                 string
}

var stringsJA = inspectStrings{
	toolbar: "Inspector のツール", pick: "選択", bounds: "レイアウト境界", repaints: "再描画",
	pause: "記録を止める", window: "ウィンドウ", untitled: "（無題）", noWindow: "調べるウィンドウがありません",
	trees: "ツリー", widgetTree: "Widget", renderTree: "RenderObject", semanticsTree: "Semantics",
	details: "詳細", displayList: "DisplayList", noSelection: "ツリーで選ぶか、「選択」でウィンドウをクリックします",
	showInternal: "内部も表示", viewTrail: "表示しているツリーの位置",
	findInTree: "名前で探す", noMatch: "一致するものがありません", refresh: "更新",
	detached: "ツリーから外れています", noNode: "ノードを開いていません", ofAncestor: "祖先 %s のノード",
	drawsNothing: "何も描きません", notLaidOut: "レイアウトの前です",
	frames: "フレーム", frameDetails: "フレームの詳細", noFrame: "棒をクリックするとそのフレームの詳細が出ます",
	stageNames: [StageCount]string{"build", "layout", "paint", "batch", "present"},
	gpu:        "GPU", allocs: "アロケーション", rebuilt: "再ビルド", laidOut: "レイアウト", recorded: "レイヤー記録",
	repainted: "全体を再描画",
}

var stringsEN = inspectStrings{
	toolbar: "Inspector tools", pick: "Pick", bounds: "Layout bounds", repaints: "Repaints",
	pause: "Pause recording", window: "Window", untitled: "(untitled)", noWindow: "No window to inspect",
	trees: "Trees", widgetTree: "Widget", renderTree: "RenderObject", semanticsTree: "Semantics",
	details: "Details", displayList: "DisplayList", noSelection: "Select in a tree, or click a window with Pick",
	showInternal: "Show internals", viewTrail: "Place of the tree shown",
	findInTree: "Find by name", noMatch: "No match", refresh: "Refresh",
	detached: "No longer in the tree", noNode: "Opens no node", ofAncestor: "Node of the ancestor %s",
	drawsNothing: "Draws nothing", notLaidOut: "Not laid out yet",
	frames: "Frames", frameDetails: "Frame details", noFrame: "Click a bar to see that frame",
	stageNames: [StageCount]string{"build", "layout", "paint", "batch", "present"},
	gpu:        "GPU", allocs: "Allocations", rebuilt: "Rebuilt", laidOut: "Laid out", recorded: "Layers recorded",
	repainted: "Whole picture repainted",
}

// stringsOf returns the words for the locale of the window.
func stringsOf(ctx core.BuildContext) *inspectStrings {
	if base, _ := basic.LocaleOf(ctx).Base(); base == japanese {
		return &stringsJA
	}
	return &stringsEN
}

var japanese, _ = language.Japanese.Base()
