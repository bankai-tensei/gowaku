# 0083: macOS の OS 統合（メニューバー、タイトルバー、トレイ、カスタムシェーダー）

決定日: 2026-11-10

状態: 採用

## 背景

ADR 0082 で macOS の土台を作ったときに「まだ無いもの」として残した項目のうち、メニューバー、カスタムタイトルバー、トレイと通知、Metal のカスタムシェーダー、Metal の golden を埋める。DnD は ADR 0050 の続き、VoiceOver は ADR 0085、sbix 絵文字は ADR 0084、arm64 の FMA は ADR 0086 に分けた。

## 決定

### 1. MenuBar は macOS では NSMenu になる（ADR 0046 のとおり）

- core に任意のインターフェース `NativeMenuHost`（`SetNativeMenu([]NativeMenuItem) bool`）を足した。`VerticalIMEHost` と同じ「Host が実装していれば使う」形で、`BuildContext.Host()` から引く。
- platform には `MenuBarWindow`（`SetMenuBar([]MenuItem, on func(id int))`）。ルートの `windowHost` が橋渡しし、項目の `OnInvoke` に ID を振って、選ばれたら post キューで走らせる。
- `MenuBar` は Build のたびに項目を渡し、渡せたら場所を取らない（`SizedBox`）。ショートカットのラッパーは残すので、⌘ の無いショートカット（F5 など）はこれまでどおりアプリが受ける。⌘ 付きは NSMenu の key equivalent が先に取る。Dispose で `SetNativeMenu(nil)`。
- メニューバーはキーウィンドウのものに入れ替わる。アプリ名メニュー（隠す・終了）と Window メニューは常に両端に置く。
- ラベルのニーモニックは Mac に無いので落とす（「ファイル(&F)」→「ファイル」、「&Open」→「Open」、「&&」→「&」）。Ctrl は ⌘ に写す（ADR 0082 の決定と同じ向き）。

### 2. CustomFrame は透明なタイトルバーと全面のコンテンツ

`NSWindowStyleMaskFullSizeContentView`、`titlebarAppearsTransparent`、タイトル非表示にし、信号機ボタンは隠す。ボタンは他の OS と同じくアプリの `TitleBar` が描く。`ClientDecorated` は true、`Maximized` は `isZoomed`。移動は `BeginMove`（`performWindowDragWithEvent:`）、縁のリサイズは OS のまま。信号機を残して並べる案もあったが、TitleBar のボタンと二重になるので採らなかった。

### 3. トレイは NSStatusItem、通知は UserNotifications

- 項目があればクリックでメニュー、無ければクリックで `TrayActivate`。アイコンは高さ 18pt に縮める。メニュー項目は tag で（トレイ番号 × 2^20 + 項目の順）見分ける。
- 通知は `UNUserNotificationCenter`。これはバンドルに入ったアプリにしか使えず、素の実行ファイルで呼ぶと落ちるので、`bundleIdentifier` が無ければ `ErrNoNotifications` を返す。通知のクリックは最初のトレイに `TrayNotificationClick` として届く。

### 4. Metal のカスタムシェーダー

gowaku-shader の MSL（spirv-cross の `main0`）は入力に `[[user(locnN)]]` を付けて出てくる。`sdf.metal` の Varyings に同じ番号を付けたので、`sdfVertex` とそのままつながる。`PrepareShader` で実行時にコンパイルし、ソースオーバーのパイプラインを作って入出力の食い違いをその場で報告する。他のブレンドとパスは初めて要るときに作る。

### 5. Metal を golden に照らす

`go test -tags gpu ./internal/golden` が macOS（arm64）でも走る。AppKit はメインスレッドでしか動かないので、`TestMain` がテストの処理をメインスレッドに回す。倍率 2 の Retina では奇数の物理サイズの窓を作れず 1px 大きくなるので、そのときは場面の大きさでサーフェスを作る（CAMetalLayer は窓と独立に描画サイズを持てる）。

### 6. 見えない窓は描かない

画面がスリープしている間、他の窓に完全に隠れている間、別の操作スペースにある間は、`CAMetalLayer` が垂直同期を待たずに drawable を渡すので、アニメーション中のアプリが 60Hz の画面で 120fps 近く回っていた（macOS のベンチで見つかった）。`windowDidChangeOcclusionState:` で `NSWindowOcclusionStateVisible` が落ちたら、最小化と同じくサイズ 0 の `EventResize` を送る。アプリは描くのをやめ、ティッカーだけを `tickWhileHidden` の間隔で回す。見えるようになったら本当のサイズと `EventRedraw` を送る。Metal の側で間隔を空ける案は、描いても誰も見ないフレームを作ることに変わりないので採らなかった。

## 影響

- 確認は GPU タグのテスト（`TestPlatform_MenuBar`、`TestPlatform_CustomFrame`、`TestPlatform_Tray`）と `examples/shader -png`。メニューの見た目、トレイのアイコン、通知の表示は、画面収録の権限が無いので目では見ていない。
- 通知を出したいアプリは .app に入れる必要がある。README に書く。
