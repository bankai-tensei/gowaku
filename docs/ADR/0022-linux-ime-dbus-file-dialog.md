# 0022: Linux の IME、D-Bus、ファイルダイアログ

決定日: 2026-11-08

状態: 一部を ADR 0053 で置き換え

## 背景

Wayland 対応の後半（10b）。Wayland の `text-input-v3` で IME の変換中の文字を窓の中に出し、D-Bus でテーマの変更に追従し、ファイルダイアログを開く。検討して次を決めた。

- ファイルダイアログは公開 API を作り、Linux（ポータル）と Windows（共通ダイアログ）の両方に実装する。
- ポータルが無い Linux では `ErrNoFileDialog` を返す。自前の FileDialog ウィジェットはデスクトップ向けウィジェットの Phase 16 に回す（当初の計画の「自前ウィジェットに退避」を変えた）。
- ibus は GNOME を headless で動かして確かめる。

## 決定

### text-input-v3

- `zwp_text_input_v3` を seat に 1 つ持つ。`enter` が来た窓が `SetIMEEnabled(true)` なら `enable`、内容の種類、キャレットの矩形を送って `commit`。`leave` か `SetIMEEnabled(false)` で `disable`。
- `done` で、その前の `preedit_string` と `commit_string` をまとめて適用する。順序はプロトコルの通り: 古い変換が消え、確定が入り、新しい変換が出る。`EventComposition(nil)`、`EventText`、`EventComposition` の順に積む。計算はタグ無しの `textinput.go` に置き、Windows でもテストする。
- **プロトコルに文節は無い。** カーソルの範囲（`cursor_begin` から `cursor_end`）が広いときは、Mozc が変換中の文節を選んでいるので、そこを `ClauseTarget`、前後を `ClauseConverted` にした。Windows と同じ注目文節の強調が出る。範囲が無ければ全体が `ClauseInput`。
- `CancelComposition` はプロトコルに無いので、`disable` と `enable` で入力メソッドをやり直させる。
- `delete_surrounding_text` は無視する。周りの文字（`set_surrounding_text`）を送っていないので、来ないはず。core の Host に周りの文字を渡す口が要るので、送るのは後回し。

### platform/dbus

- 当初の計画の通り godbus（純 Go）。セッションバスに繋げなければ何もせず `slog.Debug`。
- **テーマ:** xdg-desktop-portal の `Settings` から `org.freedesktop.appearance` の `color-scheme` を読み、`SettingChanged` を見張る。古いバックエンド（xdg-desktop-portal-gtk）は `org.gnome.desktop.interface` に gsettings の文字列で出すので、両方を受ける。好みが無い（0）ときは今まで通り gsettings のテーマ名で決める。変更は別の goroutine に来るので、フラグを立てて `Wake` し、イベントループが `EventBrightness` を積む。X11 と Wayland で共通（`platform/internal/desktop`）。
- **ファイルダイアログ:** ポータルの `FileChooser.OpenFile` / `SaveFile`。応答の `Response` シグナルは、呼び出しが返る前に来ることがあるので、決まった形の要求パスに先に購読する（ポータルの Request の説明の通り）。URI はローカルのパスに直し、それ以外（マウントできなかったリモート）は落とす。

### 公開 API

```go
gowaku.ShowFileDialog(gowaku.FileDialog{Save: true, Name: "メモ.txt"}, func(paths []string, err error) { ... })
(*WindowHandle).ShowFileDialog(d, done)
gowaku.ErrNoFileDialog
```

- 結果はコールバックで、必ず UI goroutine で呼ぶ（`Post` を通す）。キャンセルは `nil, nil`。
- パッケージ関数はフォーカスのある窓の前に出す。Widget のコールバックから窓のハンドル無しに呼べるようにするため。
- `platform.Window.ShowFileDialog(opts, done)` の `done` はどの goroutine から呼んでもよい、とした。Windows は UI スレッドで同期、Linux は goroutine から返る。
- **Windows:** `IFileOpenDialog` / `IFileSaveDialog` を COM で呼ぶ（実装はしない）。モーダルで、ダイアログが自分のメッセージループを回す。STA の初期化はそのスレッドで 1 回。保存では最初のフィルタの拡張子を既定にする。
- **Linux の親:** X11 は `x11:XID`、Wayland は `xdg-foreign-v2` で窓を書き出したハンドルで `wayland:HANDLE`。

## 結果

| 確認 | 環境 | 結果 |
|---|---|---|
| fcitx5 + Mozc で「にほんご」→ 変換 → 確定 → 追加 | headless sway | 変換中の下線、注目文節の強調、確定まで出る（`ime-fcitx5.png`） |
| 候補ウィンドウの位置 | headless sway | 確かめられない。sway 1.9 に入力メソッドのポップアップが無い（1.10 から） |
| ibus + Mozc | headless gnome-shell 46 | **一部。** 窓は CSD、フォーカス、text-input の enter / enable / 矩形 / done まで正しい。キーは ibus に渡らず英字のまま入った（mozc_server が起動しない）。RemoteDesktop の仮想キーボードの経路の問題か環境の問題かは切り分けていない |
| ポータルのファイルダイアログ | headless sway + xdg-desktop-portal-gtk | 保存ダイアログが出て、保存で `<folder>/メモ.txt` が返る。その間も窓はコンポジタに応える |
| Windows の共通ダイアログ | Windows 11 | 保存で日本語のパスが返る。キャンセルで nil |
| テーマの追従 | 偽のポータル（godbus で書いた） | 読み取り、2 つの名前空間の変更、関係無い設定の無視 |

- xdg-desktop-portal-gtk 1.15 は Settings を外に出さないので、本物のポータルでの追従は GNOME か KDE で確かめる必要がある。
- xdg-desktop-portal-gtk（GTK3）は xdg-foreign の v1 しか読まないので、v2 のハンドルではダイアログが窓の前に固定されない。GNOME（portal-gnome、GTK4）と KDE は v2。v1 の書き出しは足していない。

## 検討してやめたこと

- ポータルが無いときの自前ダイアログ: ヒアリングで Phase 16 へ。
- Windows でダイアログを別スレッドで出す: オーナー窓が別スレッドになり、モーダルの扱いが難しくなる。UI スレッドで同期にした。
- 周りの文字を送る（`set_surrounding_text`）: 再変換に要るが、core に口が無い。
