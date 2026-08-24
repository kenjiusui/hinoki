# hinoki

Hugo で書いた記事を [standard.site](https://standard.site/) のレコード（AT Protocol の `site.standard.publication` / `site.standard.document`）としてあなたの PDS（Personal Data Server、例: bsky.social）に同期する CLI ツール。

standard.site は AT Protocol 上で長文コンテンツを公開するための共有スキーマです。記事はあなたの PDS 上のレコードとして保存され、対応するインデクサー／アプリから発見・購読できるようになります。

## インストール

```
go build -o hinoki ./cmd/hinoki
```

配布用バイナリは各OS/アーキテクチャ向けにクロスコンパイルできます:

```
GOOS=darwin  GOARCH=arm64 go build -o hinoki-darwin-arm64  ./cmd/hinoki
GOOS=linux   GOARCH=amd64 go build -o hinoki-linux-amd64   ./cmd/hinoki
GOOS=windows GOARCH=amd64 go build -o hinoki-windows.exe   ./cmd/hinoki
```

## クイックスタート

Hugo プロジェクトのルート（`content/` がある場所）で:

```
hinoki init
```

対話形式で Bluesky ハンドルや `content/` ディレクトリなどを入力すると `hinoki.yaml` が作成されます（詳細は [設定](#設定-hinokiyaml) 参照）。

Bluesky の 設定 → アプリパスワード でアプリパスワードを発行し環境変数に設定してから同期します。

```
export HINOKI_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
hinoki sync
```

環境変数を設定していない場合は `sync` 実行時に対話的にプロンプト入力できます。**アプリパスワードは `hinoki.yaml` には保存されません。**

## コマンド

### `hinoki init`

カレントディレクトリに `hinoki.yaml` を対話形式で作成します。

### `hinoki sync`

`content/` 以下の Markdown 記事を走査し`site.standard.document` レコードとして PDS に作成・更新・削除します。初回実行時に `site.standard.publication` レコードも自動作成され、その rkey が `hinoki.yaml` に保存されます。

- **作成・更新**: 記事の内容ハッシュを `.hinoki-state.json` に保持し、前回から変更があった記事だけを送信します。
- **削除**: 以前に同期済みだった記事が `content/` から見つからなくなった場合（ファイル削除`exclude_dirs` / `exclude_files` への追加など）対応する PDS 上のレコードも自動的に削除します。
- **スキップ**: `draft: true` の記事はデフォルトで同期されません（`include_drafts: true` で含められます）。

### `hinoki sync --force`

内容の変更有無に関わらず全記事を強制的に再送信します。`hinoki` 自体をアップデートして PDS へのマッピング内容（保存するフィールドなど）が変わった場合、記事のソース側は変わっていないため通常の `sync` では再送信されません。そのようなときに使います。

## 設定 (`hinoki.yaml`)

設定例は [hinoki.example.yaml](hinoki.example.yaml) を参照してください。

| キー | 必須 | 説明 |
|---|---|---|
| `handle` | ○ | Bluesky ハンドル |
| `pds` | | PDS URL（デフォルト `https://bsky.social`） |
| `content_dir` | | Hugo の content ディレクトリ（デフォルト `content`） |
| `site_url` | ○ | 公開先サイトのベース URL |
| `site_name` | ○ | サイト名 |
| `site_description` | | サイトの説明 |
| `include_drafts` | | `true` で `draft: true` の記事も同期対象にする |
| `exclude_dirs` | | `content/` からの相対パスのリスト。指定したディレクトリ以下を再帰的に同期対象から除外 |
| `exclude_files` | | ファイル名のグロブパターンのリスト。ベース名または `content/` からの相対パスにマッチしたファイルを除外 |
| `publication_rkey` | | `site.standard.publication` レコードの rkey。初回 `sync` 時に自動設定される |

`exclude_dirs` / `exclude_files` は `hinoki init` でも入力できます。例:

```yaml
exclude_dirs:
  - docs
  - books
exclude_files:
  - about.md
  - privacy.md
  - "_draft-*.md"
```

## 対応している front matter

YAML (`---`) / TOML (`+++`) 形式に対応。`_index.md`（Hugo のセクションインデックス）は常に除外されます。以下のフィールドをマッピングします。

| Hugo front matter | site.standard.document |
|---|---|
| `title` | `title` |
| `date` | `publishedAt` |
| `lastmod`（無ければ `date`） | `updatedAt` |
| `description` / `summary` | `description` |
| `tags` / `categories` | `tags` |
| `slug`（無ければファイルパスから自動生成） | `path` |
| 本文（Markdown 生テキスト） | `textContent`および [at.markpub.markdown](https://markpub.at/) 形式で `content` |

`content` フィールドは standard.site 対応のビューアがリッチな整形表示に対応している場合に使われます。

## 注意事項

- アプリパスワードは `hinoki.yaml` には保存されません（環境変数か対話入力のみで扱います）。`hinoki.yaml` 自体は publication の rkey などを含むため `.gitignore` 済みですが機密情報は含みません。
