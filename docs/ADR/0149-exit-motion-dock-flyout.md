# 0149: 出てくる物は戻っても動く（DockLayout の自動で隠れるパネル）と、閉じる動きの見直し

決定日: 2026-11-17

状態: 採用

## 背景

`nav.DockLayout` の自動で隠れるパネル（ADR 0102）は、開くときは 200 ms で端から滑り出るのに、閉じるときはその場で消えていた。開くときは `startSlide` が `slide.SetValue(0)` から `Forward()` を走らせるが、閉じるときは `hideFlyout` と `toggleFlyout` が `c.flyout` を空にして組み直すだけで、`startSlide` は空のときに何もせず、`Build` も空ならパネルを子に入れない。逆向きの動きを走らせる所がどこにも無かった。

入りだけ動いて出が飛ぶと、何が起きたのかが目で追えず、パネルがどこへ行ったのかも分からない。同じ形の取りこぼしがほかの部品にもないかを、あわせて調べた。

## 決定

### DockLayout

- 閉じたパネルの ID を `dockState.leaving` に残し、`slide.Reverse()` で端へ戻す。`leaving` の間は、そのパネルの地と見出し、中身、内側の端のつまみを描き続け、値が 0 になった `OnStatus(Dismissed)` で `leaving` を空にして組み直し、子から外す。
- 戻りは 150 ms（`dockFlyoutBack`、`ReverseDuration`）で、出る 200 ms より短い。DESIGN.md の「閉じるのは速く」に合わせた。曲線は出るときと同じ `motion.Standard` のままにする。`ReverseCurve` を変えると、途中で向きを変えたときに値が跳ぶからである。
- 戻っている間、パネルはポインターもキーボードもスクリーンリーダーも受けない。`dockSlotView` に `leaving` を足し、`renderDockSlot.Hidden()` は `hidden || leaving` を返す。描くかどうかは `hidden` だけで決める（`renderDockSlot.Paint` と、`renderDock.Paint` の `dockPainted`）。`core.Hider` が隠れていると言う間は、当たり判定、Tab の順、セマンティクスから外れ、フォーカスが中にあれば次のフレームで外される（`Tree.dropHiddenFocus`）。外を押したときの扱い（`looking`）は、開いているパネル（`c.flyout`）が描いているパネルと同じときだけ働くので、戻っている途中の押下はそのまま下の物に届き、パネルを再び隠そうともしない。
- `renderDock` は、置く場所を決めるのに `c.flyout` ではなく、描いているパネルの ID（`dockView.fly` → `renderDock.flyID`）を使う。開いている間は `c.flyout` と同じで、戻っている間は `leaving` になる。
- 戻っている途中で同じパネルをまた開くと、その場所から `Forward()` で向きを変える。別のパネルを開くと、戻っていたパネルはすぐに外れ、新しいパネルが端から出る（`SetValue(0)` から）。戻っている途中でそのパネルがピンで戻されたり、閉じられたり、`Panels` から消えたりしたら（`canLeave` が偽）、すぐに外す。
- 動きを減らす設定（`basic.ReduceMotionOf`）では、今までどおりその場で消す。出るときにも滑らないので、揃っている。

### ほかの部品

ライブラリ全体で、出るときに動く物（`motion.AnimationController` の `Forward`、出現時の暗黙の動き、`SpringValue`）を洗い、閉じるときの扱いを確かめた。DockLayout のほかに、入りだけ動いて出が飛ぶ物は無かった。

| 部品 | 入り | 出 | 判断 |
|---|---|---|---|
| ダイアログ（`overlay.ShowDialog`）、コマンドパレット | 280 ms、0.96 → 1 とフェード | `Reverse()` で 120 ms のフェード、`Dismissed` でエントリーを外す。閉じている間は `ModalScope` を外す | そのまま |
| シート、引き出し（`overlay.ShowSheet`） | 280 ms で滑り込む | `Reverse()` で 160 ms、`Dismissed` で外す | そのまま |
| トースト（`overlay.ShowToast`） | 280 ms | `Reverse()` で 160 ms。まだ何も見えていないときだけすぐ外す | そのまま |
| ツールチップ | 120 ms のフェード | `Reverse()` で 80 ms、終わってからエントリーを外す | そのまま |
| `control.InfoBar` | 240 ms | `Reverse()` で 160 ms、`Dismissed` になってから空にする | そのまま |
| `nav.Navigator` のページ | 入りの動き | `Reverse()` で 200 ms、`Dismissed` で外す | そのまま |
| `basic.AnimatedSwitcher`、`control.Tabs` の中身 | 新しい子が入る | 古い子は `Reverse()` で出て、`Dismissed` で外れる | そのまま |
| `control.Disclosure` | 200 ms で開く | 同じ 200 ms で閉じ、高さが 0 になってから中身を隠す | そのまま |
| メニュー、コンテキストメニュー、メニューバー、`DropdownMenu`、`Autocomplete`、`DatePicker` の popover、プロパティの色、`DataTable` の絞り込み、`PopupList` | 動かない（すぐ出る） | すぐ消える | 揃っているので、そのまま |
| `collection.Tree` の開閉 | 矢印だけが両向きに回る、行はすぐ出る | 行はすぐ消える | 揃っているので、そのまま |
| `nav.TabBar` のタブの追加と削除 | 新しいタブはその場に出る | 閉じたタブはすぐ消え、隣がばねで詰める | 揃っているので、そのまま |

ポップアップとメニューに入りと出の動きを付けるかどうかは、この ADR では決めない。付けるなら `kit.PopupLayer` とメニューの `menuLayer` の 2 か所になるが、どちらも閉じるときに `entry.Remove()` をすぐ呼んでいるので、ツールチップのように終わるまでエントリーを残す形に直す必要があり、出の取りこぼしではなく見た目の新しい決定になる。

## 結果

- 自動で隠れるパネルは、出るときと同じ道を 150 ms で戻る。戻っている間に下の物を押せ、Tab もスクリーンリーダーもそれを見つけない。
- `dockState` に `leaving` と `slideStatus`、`canLeave` が増え、`dockSlotView` と `renderDockSlot` に `leaving`、`dockView` と `renderDock` に描いているパネルの ID が増えた。公開の API は変わらない。
- テスト（`widgets/nav/docklayout_slide_test.go`、gowakutest のスクリーンショット）は、閉じ始めて 90 ms でパネルがまだ途中まで描かれていること、そのときスクリーンリーダーにパネルが見えないこと、パネルの描かれている所を押すと下のボタンが押され、パネルが再び出ないこと、終わると消えていること、途中で同じパネルを開くとその場所から戻ること、別のパネルを開くと前のパネルがすぐに消えて新しいパネルが出ること、動きを減らす設定ではすぐ消えることを確かめる。動きを減らす設定で動く今までのテストは、そのまま通る。
- カタログとゴールデンの絵は、止まった状態しか描かないので変わらない。
