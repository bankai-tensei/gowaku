# gowaku へのコントリビュート

[English](CONTRIBUTING.md)

ご協力ありがとうございます。不具合の報告、小さな修正、ドキュメントの改善はいずれも歓迎します。新しいウィジェットの追加や公開 API の変更については、コードを書く前に issue を立て、先に設計をすり合わせてください。

## コードを書く前に

次の 2 つに目を通してください。

- [docs/ARCHITECTURE.ja.md](docs/ARCHITECTURE.ja.md): 層の分け方と、1 フレームが描画されるまでの流れ。
- [docs/CODING_STYLE.ja.md](docs/CODING_STYLE.ja.md): コーディング規約。

特に重要な決まりは次のとおりです。

- **依存は下向きのみ。** `core` は `layout`、`gpu`、`platform` を import しません。違反は `.golangci.yml` の `depguard` が検出します。
- **`unsafe`、`syscall`、`purego` は境界の内側だけで使う。** `platform/win32`、`platform/x11`、`platform/wayland`、`platform/cocoa`、`gpu/d3d11`、`gpu/vulkan`、`gpu/metal` 以外では使いません。
- **ホットパスではアロケーションしない。** reconcile、Layout、Paint、バッチ化では、ベンチマークで allocs/op = 0 を維持します。
- **Widget は値型で、ゼロ値のまま使える。** 関数オプションやメソッドチェーンは作りません。
- **エラーと panic。** 初期化の失敗は `error` で返します。実行中の契約違反は `gowaku: ` で始まるメッセージで panic します。ライブラリの内部では `recover` しません。
- **テストの大半は GPU なしで通る。** `core`、`layout`、`paint`、`text`、`theme`、`motion` は、GPU がなくても `go test ./...` が通ります。GPU が必要なテストには `//go:build gpu` を付けてください。

## コマンド

CI と同じ検査とテストは次のとおりです。

```bash
gofmt -s -l . && go vet ./... && golangci-lint run
```

```bash
GOOS=linux golangci-lint run && GOOS=darwin golangci-lint run
```

```bash
go test -race ./...
```

```bash
GOOS=linux CGO_ENABLED=0 go build ./...
```

GPU やウィンドウシステムが必要なテストは、手元でのみ実行します。

| コマンド | 実行環境 |
|---|---|
| `go test -tags gpu ./gpu/...` | Windows（D3D11 と Vulkan を比較する） |
| `go test -tags gpu ./internal/golden` | Windows。D3D11 と Vulkan を同じピクセルの golden と照合する |
| `go test -tags gpu ./platform/x11/` | Linux の X サーバー上 |
| `go test -tags gpu ./platform/wayland/` | Linux の Wayland コンポジタ上 |
| `go test -tags gpu ./gpu/metal/ ./platform/cocoa/` | Apple Silicon の macOS |

Linux のデスクトップ環境がない場合は、[scripts/wsl](scripts/wsl) を使うと、WSL 上の headless の sway と Mesa のソフトウェア Vulkan で Linux 向けの GPU テストを実行できます。`getdebs.sh`、`sway.sh`、`gputests.sh` の順に使ってください。使い方は各スクリプトの先頭に書いてあります。

生成されるファイルは次のとおりです。

```bash
go test ./internal/golden -update        # ピクセルの golden を作り直す。差分の PNG を見てからコミットする
go run ./internal/cmd/catalog            # ウィジェットカタログ（画像とページ）を作り直す
go generate ./gpu/...                    # シェーダーを作り直す
go generate ./platform/wayland/          # Wayland のバインディングを作り直す
```

カタログが古いままだと `go test ./internal/catalog/` が失敗します。新しく公開するウィジェットには、`internal/catalog/registry.go` への項目の追加とシーンの関数が必要です。詳しくは [docs/catalog/README.md](docs/catalog/README.md) を参照してください。

## markdown モジュール

[markdown/](markdown) も独立したモジュールです。go.mod には、このチェックアウトの中で作業するための `replace github.com/bankai-tensei/gowaku => ../` がありますが、markdown を使う側のモジュールではこの行は無視され、markdown の require に書いた版の gowaku が取ってこられます。そのため require には、実在し、markdown が使う API をすべて含む版を書きます。markdown が gowaku の新しい API を使い始めたら、先に gowaku を push し、`markdown/` で `go get github.com/bankai-tensei/gowaku@<コミット>` を実行して require をそのコミットにします。リリースでは両方にタグを付け（`vX.Y.Z` と `markdown/vX.Y.Z`）、markdown の require を `vX.Y.Z` にします。

## ウェブサイト

[website/](website) は、リポジトリ内の Markdown と `docs/catalog/catalog.json` から https://bankai-tensei.github.io/gowaku/ のサイトを生成します。サイト用ライブラリの依存が gowaku の go.mod に入らないよう、別モジュールにしています。変更を確認するには次のコマンドを実行します。

```bash
cd website && go run . -serve :7901
```

ページを再読み込みするたびにサイトが再生成されます。main にマージされると `.github/workflows/pages.yml` が公開します。トップページの文言は `website/words.go` に英語と日本語で書かれています。

## ベンチマーク

ホットパスを変更した場合は、そのパッケージのベンチマークを変更前と変更後に負荷の少ないマシンで実行し、両方の数値をプルリクエストに記載してください。

```bash
go test -bench . -benchmem -run '^$' ./core/ ./text/ ./render/
```

## ドキュメント

- doc コメントは英語で書き、公開する識別子にはすべて付けてください。
- 開発者向けのドキュメントは英語と日本語の両方で用意します。`X.md` が英語版、`X.ja.md` が日本語版です（ガイドは `docs/guide/en` と `docs/guide/ja`）。できれば両方を更新してください。片方しか書けない場合は、プルリクエストにその旨を書いてもらえれば、こちらで翻訳します。
- 設計上の判断は、ADR として `docs/ADR/NNNN-slug.md` に書きます。形式は「背景 / 選択肢 / 決定 / 結果」で、ADR は日本語で書きます。英語の要約をプルリクエストに書いてもらえれば、こちらで手伝います。

## コミットとプルリクエスト

- コミットメッセージは Go 本体の流儀に従い、`パッケージ名: 小文字で始まる要約` とします。例: `layout: fix the baseline of a wrapped row`。複数のパッケージにまたがる場合は `core, layout: ...`、全体に関わる場合は `all: ...` とします。
- 1 つのコミットには 1 つの意図だけを含めます。フォーマットだけの変更は別のコミットにしてください。
- 見た目が変わるプルリクエストにはスクリーンショットを添付してください。意味がある場合はライトとダークの両方をお願いします。
- CI が通っていることが条件です。

## 不具合の報告

issue には、OS とそのバージョン、GPU、Go のバージョン、不具合を再現できる最小のプログラム、期待した動作を書いてください。文字表示や IME の不具合であれば、使った入力メソッドと入力した文字も書いてください。セキュリティの問題については [SECURITY.md](SECURITY.md) を参照してください。

コントリビュートしたものは、[MIT ライセンス](LICENSE)のもとで提供されることに同意したものとみなします。
