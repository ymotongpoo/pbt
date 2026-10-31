# 次に読むもの

## 例題の移植

リポジトリの `examples` モジュールには、Fred Hebertの『[PropEr Testing](https://propertesting.com/)』の全章をGoに移植したものがあります。
この本は、pbtの背後にある考え方を学ぶのに最も良い長編の入門書で、移植はErlangの各例題がGoでどう読めるかを示しています。
書籍版は『Property-Based Testing with PropEr, Erlang, and Elixir』として刊行されており、その日本語訳が拙訳『[実践プロパティベーステスト ― PropErとErlang/Elixirではじめよう](https://www.lambdanote.com/products/proper)』（Fred Hebert著、ラムダノート、2023年）です。
ウェブ版は書籍になる前の草稿で、章の切り方が違い、標的型プロパティの章がありません。
表には両方の章番号を載せました。
日本語版の8章「標的型プロパティ」に対応する機能は、本ガイドの8章で扱っています。

| ウェブ版の章 | 日本語版の章 | パッケージ | 内容 |
| --- | --- | --- | --- |
| 1〜4 | 1〜3 | `ch01cash`、`ch04stateless` | 最初のプロパティ、モデル化、リストに関する演習 |
| 5 | 4 | `ch05generators` | 自作の生成器、`Collect` と `Aggregate`、再帰的な生成器と `Sized` |
| 6 | 5 | `ch06birthday` | CSVの往復、テンプレート、プロパティと並ぶ例示テスト |
| 7 | 6 | `ch07checkout` | 段階的な `Bind`、負のテスト、仕様のバグの発見 |
| 8 | 7 | `ch08shrinking` | `ShrinkTo`、`LetShrink`、`SampleShrink` |
| 9 | 9 | `ch09cache` | キャッシュの逐次と並行のステートフルテスト |
| 10 | 10 | `ch10bookstore` | データベースに対するステートフルテスト。SQLiteとPostgreSQL |
| 11 | 11 | `ch11breaker` | `fsm` パッケージによる状態機械 |

各パッケージのREADMEに、その章の演習への解答があります。
`examples` の中で `go test ./...` を実行してください。

## 実践的な例題

`examples/practical` には、小さいながら現実的なシステムを、プロジェクトでテストするのと同じやり方で検査する例題があります。
公開APIを通して操作し、結果をモデルと比べ、仕込んだバグの報告をテストで確かめています。
5章から7章の内容を前提にしています。

| パッケージ | 対象 | 見せること |
| --- | --- | --- |
| `httpapi` | `httptest` で動かす、楽観的並行制御つきのREST API | サーバーが採番したIDを `Var` で持つこと、ステータスコードを事後条件にすること、ロックの外でIDを採番するサーバーに対する並行実行 |
| `jobqueue` | リース、可視性タイムアウト、デッドレターを持つワーカーキュー | コマンドで進める擬似時計、失効したリースをモデルに残すこと、リースを確かめないack |
| `ratelimit` | クライアントごとのスライディングウィンドウ制限 | 「どの窓でもN件まで」という規則そのものをモデルにすること、クライアントを早く忘れすぎる掃除 |
| `sessions` | 失効、更新、ログアウトのあるセッションストア | 状態データと時計を伴う `fsm` パッケージ、行き先で分けた時間経過の遷移 |
| `ledger` | 口座間の送金 | 結果を予測するモデルと不変条件だけを確かめる監査の併用、並行実行で捕まえる残高超過の競合 |

`examples` の中で `go test -v -run Caught ./practical/...` を実行すると、仕込んだバグの報告を読めます。
例題に共通する書き方は、このディレクトリのREADMEに書いてあります。

## rapidから移行する

[MIGRATION.md](https://github.com/ymotongpoo/pbt/blob/main/MIGRATION.md) は、rapidのAPIをpbtに表で対応づけ、pbtが加えるものを列挙しています。
rapidのテストの大半は1行ずつ置き換えられます。
ステートフルテストは形が変わります。
pbtは列全体をモデルに対して生成してから実行するためです。
[BENCHMARK.md](https://github.com/ymotongpoo/pbt/blob/main/BENCHMARK.md) は同じプロパティで両者を比較しています。

## 参照資料

- pkg.go.devの[パッケージドキュメント](https://pkg.go.dev/go.ymotongpoo.com/pbt)は、すべての関数とオプションを、実行できる例とともに説明しています。
- [DESIGN.md](https://github.com/ymotongpoo/pbt/blob/main/DESIGN.md) は設計の判断と、PropEr、QuickCheck、Hypothesis、rapidとの比較を記録しています。日本語版はDESIGN.ja.mdです。
- [CHANGELOG.md](https://github.com/ymotongpoo/pbt/blob/main/CHANGELOG.md) は各リリースの変更を列挙しています。v1.0まではAPIが変わることがあります。
- 不具合の報告と質問は[GitHubリポジトリ](https://github.com/ymotongpoo/pbt)へお願いします。

## フラグの一覧

```text
go test ./... -pbt.checks=1000          # ケース数を増やす
go test ./... -pbt.seed=0x1c3           # 実行を繰り返す
go test ./... -pbt.startsize=10 -pbt.maxsize=100
go test ./... -pbt.shrinktime=2m
go test ./... -pbt.searchsteps=5000     # 標的型探索を長くする
go test ./... -pbt.nosave               # testdata/pbt に書き込まない
go test ./... -pbt.replay=testdata/pbt/TestQueue/3f2a… -run TestQueue
go test ./... -pbt.v                    # 要約と縮小のステップを表示する
```
