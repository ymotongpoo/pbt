# プロパティと実行
<!-- summary: 実行のオプションとフラグ、Assumeと破棄、SetupとCleanup、Conjunction、Deadline、OnFail、Recover、NoPanic、MaxFailuresとSignatureによる1回の実行での複数の失敗、シードと保存されたケースの再生と陳腐化、探索の時間予算、JSONの実行報告とTycheが読むOpenPBTStatsの観測記録。 -->

実行の形を決める要素を扱います。
ケースの数、サイズ、破棄したケースの扱い、ケースの前後に動くもの、プロパティの組み合わせ、失敗の再現です。

## オプションとフラグ

ほとんどの設定に、`go test` の実行全体に有効なフラグと、1回の `Check` に有効なオプションがあります。
オプションがフラグより優先されます。
ただしヘルスチェックは例外で、フラグとオプションの指定が足し合わされます。

| フラグ | オプション | デフォルト | 意味 |
| --- | --- | --- | --- |
| `-pbt.checks` | `Checks(n)` | 100 | 必要な通過ケース数 |
| `-pbt.seed` | `Seed(s)` | ランダム | 実行のシード。すべての報告に表示 |
| `-pbt.startsize`、`-pbt.maxsize` | `Sizes(start, max)` | 1、42 | 最初のケースのサイズと、増加の上限 |
| `-pbt.budget` | `Budget(d)` | 0（無制限） | 探索に使える経過時間。再実行と縮小は含まない |
| `-pbt.shrinktime` | `ShrinkTime(d)` | 30s | 失敗1件の縮小に使う時間 |
| `-pbt.failures` | `MaxFailures(n)` | 1 | 集めて縮小し報告する、種類の異なる失敗の数 |
| `-pbt.causality` | `Causality(n)` | 0（無効） | 逐次のプログラムで原因の候補を探すときの、接頭辞ごとの継続の数（5章） |
| `-pbt.coverage` | `CoverageGuided()` | false | 実行中のブロック単位のカバレッジで探索を導く（8章） |
| `-pbt.json` | `ReportJSON(path)` | 空（無効） | 実行ごとに1行のJSONの報告を追記する |
| `-pbt.observe` | `Observations(dir)` | 空（無効） | OpenPBTStatsの記録をテストごとに1ファイルへ追記する |
| `-pbt.searchsteps` | `SearchSteps(n)` | 1000 | 標的型探索のケース数 |
| `-pbt.nosave` | `NoSave()` | false | 失敗したケースを保存しない |
| `-pbt.replay` | `ReplayFile(path)` | なし | 探索の前に再実行する保存済みケース |
| `-pbt.v` | `Verbose()` | false | 通過した実行の要約と縮小の各ステップをログに出す |
| | `TestdataDir(dir)` | `testdata/pbt` | 失敗したケースの保存先 |
| | `NoReplay()` | false | 保存済みケースを再実行しない |
| | `Setup(f)` | なし | 各ケースの前に `f` を実行 |
| | `CheckCoverage()` | false | 不足か未決の `Cover` 要求で失敗 |
| `-pbt.warnhealth` | `WarnHealth(checks...)` | なし | オプションのヘルスチェックを警告として有効にする。今は `HealthDuplicate` だけ |
| `-pbt.checkhealth` | `CheckHealth(checks...)` | なし | 通過した実行を失敗にするヘルスチェック（2章） |
| `-pbt.ignorehealth` | `IgnoreHealth(checks...)` | なし | 隠すヘルスチェック |
| | `ReportLimit(chars)`、`ReportDepth(levels)` | 2000、3 | 報告する値の文字数と、入れ子の値の深さ |

<!-- snippet: properties_test.go#options -->

`MakeCheck` はプロパティを `func(*testing.T)` に包みます。
プロパティの表やサブテストに向いています。

## 時間予算

`Budget(d)` か `-pbt.budget=d` は、探索に使える経過時間を制限します。
デフォルトのゼロは、時間による制限をかけません。
時計は保存したケースの再実行の後に動き始めます。
pbtは新しいケースの前に毎回時間を確かめます。
破棄したケース、標的型探索のステップ、`CheckCoverage` が延長した実行も対象です。

ケースの数の上限はそのまま有効なので、`Budget` は最低限の実行時間を約束しません。
指定した時間ほど走らせたいときは `Checks` を大きくしてください。
`Maximize` か `Minimize` を呼ぶプロパティでは、かわりに `SearchSteps` を大きくします。

```sh
go test -count=1 -run '^TestQueue$' -pbt.checks=1000000 -pbt.budget=30s
```

実行中のケースは、予算が尽きても最後まで走ります。
止まるおそれのあるプロパティには `go test -timeout` を使ってください。
再実行と縮小はこの予算の外にあり、失敗や証拠ごとの縮小は `ShrinkTime` が別に制限します。

時間が尽きると、`Check` は `Checks` より少ないケースで通過することがあります。
集めた失敗は、そのまま失敗として縮小され、報告に載ります。
有効にしたヘルスチェックと `CheckCoverage` も実行を失敗にでき、未決のカバレッジの要求は失敗のままです。
`CheckExists` は、証拠が見つからなければ失敗します。

予算を使い切って通過した実行は、`Verbose` がなくても必ず要約を出します。
`TestSearchBudget` は `testing/synctest` を使い、1秒かかるケースを3つ、実際には待たずに走らせます。

```text
[pbt] OK, passed 3 checks in the 3s budget (seed 0x1)
```

## ケースを破棄する

`t.Assume(cond)` は条件が偽のとき現在のケースを破棄します。
PropErの `?IMPLIES` と同じです。
破棄したケースは通過にも失敗にも数えず、`Check` は別のケースを引きます。
`Skip`、`Skipf`、`SkipNow` も同じ働きです。
そのケースですでに失敗を記録していれば、後から破棄しても失敗は残ります。

<!-- snippet: properties_test.go#assume -->

破棄には予算があります。
破棄がチェック数の10倍を超えると、最後に破棄した理由を示して実行は失敗します。
大半のケースを破棄するプロパティは、主張しているより少ないことしか検査していないからです。
`Assume(b > 0)` より `IntMin(1)` のように、有効な入力を作る生成器があるならそちらを選んでください。

## セットアップとクリーンアップ

`Setup` オプションは各ケースの前に関数を実行します。
その関数は後片付けの関数を返してもよく、返した関数はケースの後に実行されます。
`t.Cleanup` はプロパティの中から同じ目的の関数を登録します。
クリーンアップは、ケースが失敗したかどうかに関係なく、ケースの後に登録と逆の順序で実行されます。

あるクリーンアップでパニックや破棄が起きても、残りのクリーンアップは実行されます。
プロパティがケースを破棄していても、クリーンアップで失敗すればケースは失敗になります。
クリーンアップで破棄しても、プロパティの失敗やパニックは消えません。
プロパティとクリーンアップの両方でパニックが起きた場合、報告にはプロパティのパニックが残ります。

<!-- snippet: properties_test.go#setup -->

## プロパティを組み合わせる

`Conjunction` は名前付きの複数のプロパティを同じケースで実行し、失敗したものの名前を示します。
後続のプロパティがケースを破棄しても、先行するプロパティの失敗は消えません。
`Deadline` は所定の時間より長くかかったケースを失敗にします。

<!-- snippet: properties_test.go#combinators -->

連言の失敗には、失敗した要素の名前が付きます。

```text
[pbt] error: non-empty: empty
[pbt] error: conjunction: [non-empty] failed
```

Goは実行中の関数を中断できないので、`Deadline` はプロパティが戻った後に経過時間を検査します。
戻ってこないプロパティは `go test -timeout` が捕まえます。
`Timeout` は `Deadline` の以前の名前で、引き続き使えます。

## 失敗を説明する

`t.OnFail(f)` は、報告されるケースについてだけ、縮小の後に実行される関数を登録します。
そこでログに出した内容は報告に載ります。
テスト対象のシステムの状態や計算の途中の値を表示する場所で、PropErの `?WHENFAIL` に当たります。

<!-- snippet: properties_test.go#onfail -->

## パニック

プロパティの中のパニックはケースを失敗にします。
報告にはパニックの値と、あなたのコードのフレームが載ります。
縮小器は同じ場所でパニックする最小のケースを探します。
パニックそのものを扱うプロパティのために、補助の関数が2つあります。

<!-- snippet: properties_test.go#panics -->

`Recover` は関数を実行し、パニックの値か `nil` を返すので、不正な入力でパニックすることをプロパティで主張できます。
`NoPanic` は関数を受け取り、その引数を `Make` で生成します。
`Make` は9章で説明します。
`WithType` と `WithField` のオプションで引数を絞れます。

## 1回の実行での複数の失敗

実行は通常、最初の失敗で止まり、それを縮小して報告します。
そのため、バグが2つある関数では、バグが1つずつしか見えません。
1つ目を直して再び実行して、ようやく2つ目に出会います。
`MaxFailures(n)` か `-pbt.failures=n` を指定すると、失敗の後も実行を続けます。
仕方の異なる失敗を最大 `n` 件集め、それぞれを縮小し、保存し、報告します。

<!-- snippet: failures_test.go#failures -->

```text
[pbt] failure 1 of 2: property failed after 6 passed checks and 9 shrink steps (size 4)
[pbt] s (StringOf(SampledFrom(10 values))) = "0"
[pbt] error: parsePort("0") accepted 0
[pbt] reproduce: go test -run 'TestPort' -pbt.seed=0x1 -pbt.failures=2
[pbt] failure 2 of 2: property failed after 6 passed checks and 90 shrink steps (size 8)
[pbt] s (StringOf(SampledFrom(10 values))) = "7000"
[pbt] error: parsePort("7000") rejected a valid port
[pbt] reproduce: go test -run 'TestPort' -pbt.seed=0x1 -pbt.failures=2
```

デフォルトでは、失敗した行が違うか、パニックの型か発生位置が違うとき、2つの失敗は異なるものとみなされます。
この区別は、3章で縮小を1つのバグにとどめていたものと同じです。
縮小器が途中で新しい仕方の失敗に出会えば、それも `n` 件のうちの1件になります。
どの失敗にも、`ShrinkTime` の時間がそれぞれ丸ごと割り当てられます。
2つ目の最小値7000は局所的な最小です。
本当の境界は6554ですが、そこへ至るには後ろの桁をより大きな数字にする必要があり、縮小器はそれを試しません。
保存した失敗はまとめて再実行され、保存済みのケースが再実行で失敗したときは、それ以上の失敗を探しません。
`CheckExists` はこのオプションを無視します。

いくつかの原因が1つのアサーションを共有するときは、原因ごとに安定した名前を `t.Signature` に渡してください。
すべての失敗が同じ行に届いても、`MaxFailures` は名前ごとに分けて集めます。
[3章の名前付きのポートの例](03-shrinking.md#失敗は同じものに保たれる)は、パーサーが受け付けてしまう不正なポートと、拒んでしまう正しいポートを分けています。

## シード、保存したケース、再実行

すべての報告は、実行のシードを含む `reproduce:` の行で終わります。
そのシードを `-pbt.seed` に渡すと、プロパティのコードとpbtのバージョンが同じであれば、失敗したケースを含む同じケースの列が繰り返されます。
pbtを更新すると、同じシードから生成されるケースが変わることがあります。
v0.3.2では整数生成の修正により、一部のシードで生成結果が変わります。
v0.4.0では、一部の再帰的な型について `Make` が生成する値が変わり、縮小器が別の、より小さなケースに到達することがあります。
保存済みのケースは、その選択の列を、インストールされているバージョンの生成器に通して再生します。
pbtを更新した後は、再帰的な型に `Make` を使うプロパティの保存済みケースが、別の値を作ることがあります。

失敗したケースは `testdata/pbt/<テスト名>/<ハッシュ>` にもテキストファイルとして保存されます。
サイズ、コメントとして取り出した値、そして選択の列が入っています。
同じテストの以後の `Check` は、新しいケースを生成する前に保存済みのケースを再実行し、まだ失敗するなら失敗にします。
回帰ケースがコードとともにバージョン管理に入るわけです。
バグを直した後も、このファイルは回帰テストのケースとして残してください。
削除すればその再実行はなくなりますが、実行を通すためだけに削除しないでください。
この挙動を調整するオプションが3つあります。
`NoSave` は実行に書き込みをさせません。
`NoReplay` は保存済みケースを飛ばします。
`ReplayFile` またはフラグ `-pbt.replay=FILE` は、どのテストでも保存済みファイルを1つ再実行します。
あるテストのケースを別のテストに渡すときや、CIのログにある失敗を手元で再現するときに使います。

`HealthStale` は、保存した取り出しのコメントが今の再実行の取り出しと異なるとき、デフォルトで警告します。
生成器が変わると、同じ選択が別の入力になることがあります。
警告にある保存時の文字列と今の文字列を比べ、元の値をまだ試す必要があるなら、その値を明示的にテストしてください。
陳腐化したケースの再実行が通過しても、元の回帰が直ったことの証明にはなりません。

比較は文字列として描画した値で行い、値の深い比較は行いません。
切り詰めで変化が隠れることがあり、ポインターや独自の書式のように描画が安定しない取り出しは比較しません。
`CheckHealth(HealthStale)` か `-pbt.checkhealth=stale` を付けると、この警告で通過した実行を失敗にします。
`IgnoreHealth(HealthStale)` は警告を隠します。
再実行は保存したコメントを書き換えません。

## 機械可読の出力

`ReportJSON(path)` か `-pbt.json=path` は、`Check` か `CheckExists` のたびにJSONを1行追記します。
デフォルトの空のパスでは出力しません。
親ディレクトリは先に作っておいてください。
スキーマの名前は `go.ymotongpoo.com/pbt/report/v1` で、読む側は知らないフィールドを無視してください。

| フィールド | 内容 |
| --- | --- |
| `test`、`seed`、`kind`、`status`、`errors` | テスト名、16進数のシード、`check` か `exists`、結果、エラーメッセージ |
| `checks_passed`、`rejected`、`discards` | 探索の件数。再実行と縮小は含まない。`rejected` は失敗した候補の数 |
| `budget_exhausted`、`durations_seconds` | 時間で探索が終わったか。全体、探索、縮小の所要時間 |
| `failures`、`witness` | 失敗の識別子、描画した取り出し、保存先のパス、再現のコマンド。`witness` は `CheckExists` が成功したときだけ |
| `health`、`statistics`、`objectives` | 診断、ラベル、カバレッジの要求、測定値、最良の効用 |

`status` は `passed`、`failed`、`gave_up`、`no_witness`、`error` のどれかです。
取り出しの値は描画した文字列で、`ReportLimit` と `ReportDepth` に従います。
有限でない数は、文字列の `NaN`、`+Inf`、`-Inf` で表します。

<!-- snippet: run_output_test.go#machine-output -->

`TestMachineReadableOutput` は両方のファイルを確かめます。
次はそのJSONの報告から、所要時間、統計、空のコレクションを省いたものです。
載せたフィールドの値は、取得したままです。

```json
{"schema":"go.ymotongpoo.com/pbt/report/v1","test":"TestRecordedRun","seed":"0x1","kind":"check","status":"passed","checks_passed":3,"rejected":0,"discards":0,"budget_exhausted":false}
```

`Observations(dir)` か `-pbt.observe=dir` は、Tycheが読むOpenPBTStatsの記録を追記します。
デフォルトの空のディレクトリでは記録しません。
pbtはディレクトリを作り、`observations/TestRecordedRun.jsonl` のように、整えたテスト名ごとに1ファイルを書きます。
サブテストの名前はサブディレクトリになります。
整えた名前は衝突することがあり、同じファイルに入った記録は `property` と `run_start` で区別します。

`test_case` の記録には、入力、結果、生成の段階、特徴量、所要時間が入ります。
特徴量には、ラベル、測定値、カバレッジの要求に当たったかどうか、標的の効用が入ります。
所要時間は、入力の生成とテストの実行を分けて記録します。
再実行と縮小の試行も記録になります。
`metadata.final=true` の付いた `minimal` の記録はスナップショットで、もう一度実行したものではありません。
実行の最後には `info` の記録が入ります。
`coverage` のフィールドは、`CoverageGuided` を使っても今は `null` です。

```sh
go test -count=1 ./... -pbt.json=runs.jsonl -pbt.observe=observations
```

ログの転送エージェントでこれらのファイルを読み、記録や集計した件数をログやメトリクスのバックエンドへ送れます。
pbtはファイルを手元に書くだけで、ネットワークには送りません。
観測記録はバッファーにためて実行の最後に書き出すので、異常終了するとバッファーの中の記録が失われることがあります。
書き込みに失敗するとテストは失敗します。
求められた出力を届けられなかったからです。
