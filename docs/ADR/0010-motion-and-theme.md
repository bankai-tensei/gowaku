# 0010: 動き、MD3 テーマ、OS 追従

決定日: 2026-11-07

状態: 一部を ADR 0011 で置き換え

> MD3 に関する部分（配色の処方、状態レイヤーとリップル、Button 5 種）は ADR 0011 で置き換えた。motion、Canvas のスケール、Ticker の眠り、Overlay、OS テーマの判断は今も有効。

## 背景

Phase 6 で当初の計画の残り（モーションとテーマ）を実装した。計画のとおりに進まなかった点と、書かれていなかった形式を残す。

## 決定

### スケールは Canvas が CPU で掛ける。シェーダーは変えない

`Canvas` の状態を「平行移動 + 一様スケール」にして、`DrawRRect` などの座標・角丸・ボーダー幅・ブラーを記録時に掛ける。インスタンスも HLSL も変更なし。パスと形のあるクリップは頂点の生成時に `Op.Scale` を掛ける。`Canvas.Transform` は平行移動と一様スケールだけ受け付け、回転は panic する（当初の計画の v2 項目）。

### グリフはサンプラーを足さず、ラスタサイズを量子化する

ADR 0007 は「Phase 6 のスケール遷移にはサンプラーとパディングが要る」としていた。やめた。

| 案 | 長所 | 短所 |
|---|---|---|
| サンプラー + パディング | 任意の倍率でなめらか | 全グリフにパディング、シェーダー変更、等倍の鮮明さを失う恐れ |
| 毎フレーム実サイズでラスタ | 変更なし | 遷移 1 回でアトラスにサイズが 20 種類増える |
| **整数ピクセルに量子化** | シェーダー変更なし、サイズは 2〜3 種類 | 遷移中は字の大きさが最大 0.5 物理ピクセルずれる |

遷移は 300ms 以内で必ず 1.0 に戻るので、量子化で足りる。グリフの位置は正確に掛ける。アイコン（アトラス経由の小さいパス）はスケール中だけステンシル経路を通す。任意倍率のズームが要るなら v2 の Affine と一緒に作る。

### Opacity は図形ごと

`RenderOpacity` は `PushOpacity` を使う。重なった図形は下が透ける。真のグループ不透明度にはレイヤーが要り、それは当初の計画の v2。ダイアログのフェードでは影と本体の重なりがわずかに濃さを変えるが、300ms の遷移では見えない。

### Ticker に「眠り」を足した

キャレットの点滅に毎フレームの Ticker を使うと、フォーカスがあるだけで 60fps で描き続ける。軸 1 の「変化がなければ描かない」に反する。`Ticker.SleepUntil` を足し、`Tree.Due` と `Tree.NextWake` で `waitTime` が次の起床まで眠る。点滅は 1 秒に 2 フレームで済む。ウィンドウがフォーカスを失うと止まる。スクロールバーの「止まってから 0.9 秒で消える」も同じ仕組み。

### 暗黙アニメーションは 1 コントローラ + 複数の値

`motion.Implicit` が 1 本の `AnimationController` と `Value[T]` の集まりを持つ。`Set` は目標を記録するだけで、`Commit` が全部の値を「いまの値」から張り直す。Flutter の `ImplicitlyAnimatedWidgetState.forEachTween` と同じ考え方。目標が変わらなかった値も張り直さないと、コントローラの巻き戻しで飛ぶ。

`Commit` は `DidUpdateWidget` や Build の中で呼ばれるので、Duration 0 のときは `SetValue` に落としてコールバックを呼ばない（Build 中の `SetState` は panic）。

`Curve` は `func(float32) float32` にした。interface にしないのは、`motion.EaseOutCubic` をパッケージ変数にせず関数のまま渡せるから（CODING_STYLE §3.2）。

### theme は core を知らない。ThemeOf は widgets に置いた

当初の計画は `theme.go` に `ThemeOf(ctx)` を置いていたが、CODING_STYLE §3.1 は `theme → text, geom`。規約を優先した。`widgets.ThemeOf` と `gowaku.ThemeOf`。`CopyWith` は作らない。`Theme` は値の構造体なので `t := *th; t.Shape = ...` で足りる（当初の計画の「`copyWith` の乱用を持ち込まない」）。

`ThemeFromSeed` は light と dark の組 `theme.Set` を返す。`App.Theme` に組で渡すので、利用者のコードに分岐なしで OS に追従できる。

### 配色は CorePalette の処方 + 中立色の彩度 6

`ThemeFromSeed(0x6750a4)` が m3.material.io の基準トークンに最も近くなる組を選んだ。primary 彩度 max(48, seed)、secondary 16、tertiary hue+60 彩度 24、error hue 25 彩度 84、中立色 6、中立バリアント 8。上流 CorePalette の中立色は 4 だが、19 トークンの最大誤差の合計が 51 から 12 に減る。

既知の差:

- 公開トークンとの差は大半が 1/255、最大 3/255（surfaceContainer）。
- **error 系は公開値と違う**（例 B3261E に対して BA1A1A）。公開値は手で選ばれたパレットで、HCT の処方からは出ない。上流の処方のままにした。
- ライトの OnContainer 系は tone 10。最新のトークンは tone 30 に移った（onPrimaryContainer 4F378B）。`lightScheme` の 4 行で変えられる。
- ダークの OnErrorContainer は上流 `Scheme.dark` の 80 ではなく 90。他の OnContainer と揃い、公開トークンとも合う。

出典: material-color-utilities（Apache-2.0、commit 194144d）から HCT と solver を移植。`theme/LICENSE.material-color-utilities` に原文。テストベクタは `theme/testdata`。影の数値と照合用トークンは material-web（Apache-2.0、commit d872455）。

`text.Style` に字間が無いので、MD3 の letter spacing は入れていない。

### Button は FilledButton に置き換えた

`Button` は仮の実装だった。v1 公開前なので別名を残さず 5 種の型にした。中身は `materialButton` 1 つで、見た目だけが違う。`AnimatedContainer{Duration: ...}` と書けるように、`Duration` と `Curve` は埋め込みではなく直接のフィールド（埋め込みの昇格フィールドは複合リテラルで指定できない）。

### 状態レイヤーとリップル

当初の計画のとおり enabled / hover / focus / press / disabled に絞った。`layout.RenderInk` がコンテナと中身の間に状態レイヤー、リップル、フォーカスリングを描く。リップルは `ClipRRect` の中の円で、押している間だけステンシルを 1 回使う。

### キーは親へ伝わる

`DispatchKey` はフォーカスを持つものから親へ `FocusTarget` をたどる。ボタンにフォーカスがあるときの Escape をダイアログが受けるため。Tab はどれも使わなかったときだけフォーカスを動かす。

### モーダルは `core.FocusScope` と Stack の順

`Overlay` は Stack に項目を足す。ヒットテストは最後の子から見るので、全面の項目が下を遮る。Tab の範囲は `FocusScope.Modal()` の最後のものに限る。ダイアログは開いたときに自分にフォーカスを取り、閉じたら `RestoreFocus` で戻す。

スケール中のダイアログのヒットテストは等倍のまま。遷移は短く、途中のクリックが少しずれても害はない。

### ScrollBar は Scroll と ListView が自動で持つ

当初の計画は独立したウィジェットとしていたが、位置と全長を知っているのはビューだけなので `RenderScrollFrame` が重ねる。MD3 に仕様が無いので見た目は Flutter に寄せた。スクロールできないときは `HitTest` が false を返し、下のクリックを奪わない。

### OS テーマ

`AppsUseLightTheme` を読み、`WM_SETTINGCHANGE` の `ImmersiveColorSet` で読み直す。ウィンドウ数だけ届くので、`app` が値を比べて重複を捨てる。タイトルバーは `DwmSetWindowAttribute(DWMWA_USE_IMMERSIVE_DARK_MODE)`。背景は `App.Background` がゼロなら surface に追従する。`App.ThemeMode` で固定できる。

`MediaQueryData` に Size を入れていない。Run のルートが再ビルドされると Home 全体の Build が走るので、リサイズのたびに起きると 4K リサイズのベンチに響く。Scale と Brightness だけにした。

## 結果

- `Canvas.Transform`、`motion`、`theme`、暗黙アニメーション 5 種、Overlay / Dialog、Button 5 種、TextField 2 種、Switch、Slider、ScrollBar、OS 追従が入った。
- シェーダーとインスタンスの形式は変更なし。

## 持ち越し

- MD3 テキストフィールドの disabled と error 状態。
- Slider はクリックではキーボードフォーカスを取らない（Tab のみ）。
- テーマ切替そのもののアニメーションはしない（即時）。
- Dialog の幅は既定 360 の固定。レイアウトに内在サイズの問い合わせが無いため。
- letter spacing。
