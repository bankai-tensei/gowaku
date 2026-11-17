# 0043: アクセシビリティ（12b: Linux の AT-SPI2、動きを減らす）

決定日: 2026-11-09

状態: 採用

## 背景

アクセシビリティ（Phase 12）の後半。12a（ADR 0042）で作った `semantics` の木と `platform.Accessibility` を、Linux の AT-SPI2 につなぐ。検討して次を決めた。

- **確認方法：** godbus で書いた AT-SPI クライアントの自動テストと、WSL で動かした Orca のデバッグログの読み上げ文。NVDA と同じ手順にする。音は出さない。
- **画面：** 木の確認は X11 と Wayland の両方で行う。Orca のキー操作は Xvfb を主にし、ヘッドレスの sway でも試す。
- **ReduceMotion：** xdg-desktop-portal の Settings から `org.gnome.desktop.interface` の `enable-animations` を読み、変化も追う。ポータルが無いときと KDE では「減らさない」。
- **Text：** UIA と同じ最小限。文字列、キャレット、選択、文字・語・行の単位の `GetStringAtOffset`、EditableText の `SetTextContents`。属性と文字ごとの矩形は作らない。

## 決定

### 1. 置き場所

AT-SPI のサーバーは `platform/dbus` に置く（`atspi*_linux.go`）。X11 と Wayland のどちらも同じものを使う。godbus は純 Go なので、cgo も purego も要らない。`Window.Accessibility()` は X11 と Wayland のどちらでもこのブリッジを返す。

### 2. バスと登録

- アクセシビリティバスのアドレスは、セッションバスの `org.a11y.Bus` の `GetAddress` で得て、そこに別の接続を張る。セッションバスかアクセシビリティバスに繋がらなければ、ブリッジは何もせず、`Listening()` は偽のままにする（理由は `slog.Debug`）。
- アプリケーションの根は `/org/a11y/atspi/accessible/root`。`org.a11y.atspi.Registry` の `Socket.Embed` で登録する。プロセスに 1 つの接続とアプリケーションを持ち、窓はその子にする（GTK と同じ）。各ノードは `/org/a11y/atspi/accessible/<ID>`。窓のノードの ID は、窓ごとに上位ビットを分けて衝突を避ける。
- **`Listening()` は「アクセシビリティバスに繋がった」で真にする。** UIA と違って、問い合わせが来るのを待たない。AT-SPI のクライアントは、イベントを登録してから木を読むためである。誰も聞いていないデスクトップでは、`org.a11y.Status` の `IsEnabled` と `ScreenReaderEnabled` がどちらも偽なので、繋がない。

### 3. スレッド

godbus はメソッド呼び出しを自分のゴルーチンで呼ぶ。そこで、スナップショットは mutex で守り、読むだけにする。アクションは mutex つきの列に積み、`wake` で UI ゴルーチンを起こす。イベントループは、テーマの変化と同じ場所でその列を取り出し、`EventAccessibilityAction` として積む。シグナル（イベント）は `Update` の中、つまり UI ゴルーチンから送る。

### 4. インターフェース

| AT-SPI | 対応 |
|---|---|
| Accessible | 名前、説明（Hint）、親、子、ロール、状態、属性（`level`、`container-live` など）、`GetInterfaces` |
| Application | ツールキット名 `gowaku`、版 |
| Component | `GetExtents`（画面座標。Wayland は窓座標しか無いので窓の中の座標を返す）、`GetAccessibleAtPoint`、`GrabFocus` → ActionFocus |
| Action | Tap は `click`、Increase / Decrease、Expand / Collapse |
| Value | Range を持つノード。`CurrentValue` の設定 → ActionSetValue |
| Text | TextField。`CharacterCount`、`CaretOffset`、`GetText`、`GetStringAtOffset`（文字・語・行）、`GetTextAtOffset`（古い形）、`GetNSelections`、`GetSelection`、`SetSelection`、`SetCaretOffset` |
| EditableText | `SetTextContents` → ActionSetValue |

**オフセットの単位：** AT-SPI の Text のオフセットは文字（コードポイント）単位で、UIA の UTF-16 とは違う。`semantics.TextInfo` は UTF-16 のまま変えず、ブリッジで変換する。

**イベント：** `Object:StateChanged`（focused、checked、selected、enabled、expanded）、`Object:PropertyChange`（accessible-name、accessible-description、accessible-value）、`Object:ChildrenChanged`、`Object:TextChanged`、`Object:TextCaretMoved`、`Object:TextSelectionChanged`。ライブ領域は `Object:Announcement` で知らせる（Orca 45 以降が読む）。

### 5. ロールの対応

ADR 0042 §2 の表の AT-SPI 列のとおり。Switch は `toggle button`、Stepper は `spin button`、Heading は `heading` に `level` 属性を付ける。

### 6. ReduceMotion

`platform/dbus` に `ReadEnableAnimations` と `WatchEnableAnimations` を足す。`platform/internal/desktop` の `Theme` が、明るさと同じように変化を覚え、イベントループは `EventSettings` を積む。

## 実装で決めたこと

- **接続の強制と停止：** `GOWAKU_A11Y=1` で、`org.a11y.Status` が無効でも繋ぐ。`NO_AT_BRIDGE=1`（GTK と同じ名前）で繋がない。
- **接続は Platform ごとに 1 本。** パッケージ変数を置かないため。ノードのパスは `/org/a11y/atspi/accessible/<窓の番号 << 48 | ID>` にする。
- **窓の位置：** X11 は ConfigureNotify か `XTranslateCoordinates` で得る。聞き手がいる間だけ計算する。Wayland は原点を 0 にする。
- **ライブ領域は、変わったときだけ読む。** 現れたときには読まない（ARIA と同じ）。Orca の確認で、起動するとすぐ状態の行が読まれたため。UIA も同じにした。
- **細部：**
  - 選ばれた Radio は `checked`、Dialog は `modal`。
  - Increase と Decrease しか無いノードに `CurrentValue` を書くと、その向きに 1 段だけ動かす。
  - ロールの無いノードは、子があれば panel、ラベルがあれば label、どちらも無ければ filler にする。
  - 語の単位の `GetStringAtOffset` は、後ろの空白を含めない。

## 却下した案

- **AT-SPI のために `libatk-bridge` を purego で読み込む：** 木を ATK の形で持つ必要があり、二重になる。D-Bus のプロトコルを直接話すほうが小さい。
- **`org.a11y.atspi.Cache` の実装：** Orca は無くても個別の呼び出しで木を読む。必要が見えてから足す。
- **AT-SPI のオフセットを UTF-16 にそろえる：** プロトコルが文字単位なので、合わせないとクライアントが崩れる。
