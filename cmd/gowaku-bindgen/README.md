# gowaku-bindgen

Wayland のプロトコル XML から、cgo 無しの Go バインディングを生成する。gowaku 自身の `platform/wayland/protocol_linux.go` もこれで作っている。gowaku が扱わないプロトコル（特定のコンポジタだけが持つものなど）を、アプリから使うためのもの。

Vulkan のバインディングは手書きで、vk.xml の生成器は無い。

## 使い方

```
go run github.com/bankai-tensei/gowaku/cmd/gowaku-bindgen -out foo_linux.go foo-v1.xml
```

| フラグ | 既定 | 意味 |
|---|---|---|
| `-out` | （必須） | 書き出す Go ファイル |
| `-pkg` | 出力先のディレクトリ名 | パッケージ名 |
| `-runtime` | `github.com/bankai-tensei/gowaku/platform/wayland` | 生成コードが呼ぶパッケージ。空にすると、そのパッケージ自身の中に置く生成物になる（gowaku 内部用） |
| `-table` | `interfaces` | `wl_interface` の表の配列の名前。同じパッケージに 2 つ生成するときに変える |
| `-tags` | `linux` | 出力のビルド制約 |
| `-skip` | 無し | 生成しない interface（カンマ区切り） |

interface ごとに次ができる。名前はすべて非公開（`example_shortcut_v1` → `exampleShortcutV1`）。

- proxy の型（`wayland.Proxy` が下）と、request ごとのメソッド。第 1 引数は `*wayland.Conn`
- event ごとのメソッドを持つ `…Listener` interface と、全部を無視する `…Nop`（埋め込んで一部だけ受ける）
- `listen…(conn, proxy, listener)`
- enum の定数、`…Version`、`…Since`

生成コードは `unsafe` も purego も使わない。C のメモリに触るのは `platform/wayland` の中だけ。

## 例

[testdata/example-shortcut-v1.xml](testdata/example-shortcut-v1.xml) は架空の「グローバルショートカット」のプロトコル。[testdata/shortcut](testdata/shortcut) にその生成物と使う側のコードがある。

```go
//go:generate go run github.com/bankai-tensei/gowaku/cmd/gowaku-bindgen -out shortcut_linux.go example-shortcut-v1.xml
```

```go
func Register(p *wayland.Platform, w platform.Window, name string, pressed func()) (*Shortcut, bool) {
	h, ok := w.NativeHandle().(platform.WaylandHandle)
	if !ok {
		return nil, false
	}
	for _, g := range p.Globals() {
		if g.Interface != "example_shortcut_manager_v1" {
			continue
		}
		c := p.Conn()
		m := exampleShortcutManagerV1(p.Bind(g, exampleShortcutManagerV1Interface, exampleShortcutManagerV1Version))
		s := &Shortcut{c: c, pressed: pressed}
		s.proxy = m.register(c, wayland.Proxy(h.Surface), name, "")
		listenExampleShortcutV1(c, s.proxy, s) // Shortcut は exampleShortcutV1Nop を埋め込み、Pressed だけ持つ
		m.destroy(c)
		return s, true
	}
	return nil, false
}
```

- グローバルは `Platform.Globals` で探し、`Platform.Bind` で bind する。版は指定と、コンポジタの版の小さい方。
- 窓の `wl_surface` は `wayland.Proxy(h.Surface)`（`h` は `NativeHandle()` の `platform.WaylandHandle`）。
- event は `Platform.WaitEvent` の中で、UI ゴルーチンで listener に届く。gowaku 自身の event と同じディスパッチャを通る。

`go test ./cmd/gowaku-bindgen/` がこの例を生成し直して、コミット済みのものと比べ、`GOOS=linux` でビルドと vet をする。

## 制限

- **`gowaku.Run` のアプリからは使えない。** `*wayland.Platform` に届かないため。今は `wayland.New()` で Platform を自分で作り、イベントループを自分で回すプログラム向け。
- 他のプロトコルの interface（`wl_surface`、`wl_seat` など）は型の無い `wayland.Proxy` になる。gowaku が bind した `wl_seat` や `wl_output` は渡せないので、要るなら `wayland.xml` も入力に入れて自分で bind する（`-skip` で要らないものを外す）。
- new_id で作る interface は入力に含まれていなければならない（無いと生成がエラーになる。libwayland が proxy を作るのに表を要るため）。
- null を許す string に null は渡せない（空文字列になる）。null の object は 0。
- 後から来たグローバル（出力の追加など）は通知されない。`Globals` を呼び直す。
- `Platform.Destroy` の後に proxy を使ってはいけない。すべて UI ゴルーチンから呼ぶ。

設計は [ADR 0021](../../docs/ADR/0021-wayland-platform.md) と [ADR 0077](../../docs/ADR/0077-wayland-bindgen.md)。
