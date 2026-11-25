# 標的型探索と存在性
<!-- summary: 効用に向けて実行を導くMaximizeとMinimize、測った仕事量や時間による遅い入力の探索、go test -coverでの実験的なCoverageGuided探索、証拠が存在するかを問うプロパティのためのCheckExists。 -->

ランダムな入力は、システムのよくある状態にはすぐ到達しますが、まれな状態には遅いか、到達しません。
標的型プロパティベーステストは効用関数を加えます。
プロパティが、ケースが興味深い何かにどれだけ近づいたかを報告し、pbtは新しいケースを引く代わりに効用の高いケースを探します。
PropErでは `?MAXIMIZE` です。

## Maximize

<!-- snippet: targeted_test.go#maximize -->

`n` 歩のランダムウォークは原点から `sqrt(n)` ほどの距離で終わるので、100回のランダムな歩みが50に届くことはまずありません。
`t.Maximize("position", pos)` があると、最初のケースが効用を報告した時点で実行は探索に変わります。
pbtはそれまでで最良のケースの選択を変異させ、効用が高ければその変異を残し、局所的な最大から抜けるために探索の早い段階では悪い変異も時々残し、ときどき新しいケースからやり直します。
探索は、チェック数ではなく `SearchSteps` か `-pbt.searchsteps` で決めたステップ数（デフォルトは1000）だけ続きます。

次の報告では、歩みの列を `…` で短くし、再現の行を省いています。

```text
[pbt] property failed after 246 passed checks and 2039 shrink steps (size 33)
[pbt] utility position = 51
[pbt] steps (SliceOf(SampledFrom(2 values))) = []int{1, 1, 1, 1, …}
[pbt] error: the walk reached 51
```

探索は246ケースで50を超える歩みを見つけ、縮小器がそれを右への51歩、つまり失敗する最小の入力まで減らしました。
プロパティが通過したときは、要約に観測した最良の効用が載ります。
`t.Minimize` は逆向きに探索します。
同じ名前で `Maximize` を再び呼ぶと、そのケースの値が置き換わります。

「近さ」の概念があるプロパティで探索を使ってください。
木の深さ、キューの長さ、再試行の回数、境界までの距離などです。
大半の入力で平らな効用は、探索に登る手がかりを与えません。

## 複数の目的

1つのケースは、名前の異なる複数の効用を報告できます。
このとき探索は効用の名前ごとに目的を分け、目的ごとに現在のケースと最良のケースを持ちます。
目的は交代で探索されます。
各ステップは、目的が最初に現れた順に、1つの目的の現在のケースを変異させます。
変異を残すかどうかは、その目的の効用だけで判断します。

<!-- snippet: targeted_test.go#objectives -->

```text
[pbt] property failed after 41 passed checks and 3005 shrink steps (size 26)
[pbt] utility queue = 1
[pbt] utility wait = 101
[pbt] jobs (SliceOf(IntRange(1, 10))) = []int{3, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 1}
[pbt] workers (IntRange(1, 4)) = 1
[pbt] error: a job waited 101
[pbt] reproduce: go test -run 'TestPool' -pbt.seed=0x1
```

報告には、失敗したケースのすべての効用が載ります。
通過した実行では、要約に各目的の最良の値が載ります。
目的どうしで探索のステップを分け合うので、目的が複数ある実行には `SearchSteps` で多めのステップを与えてください。
目的が1つのプロパティは、以前とまったく同じように探索します。

## 遅い入力を探す

性能のプロパティでは、テスト対象の呼び出しにかかった時間を最大化できます。
先に入力を取り出し、呼び出しの直前に `time.Now()` を記録し、`time.Since(start).Seconds()` を `t.Maximize("seconds", elapsed)` に渡します。
こうすると、生成にかかった時間が効用に入りません。

経過時間にはばらつきがあります。
スケジューリング、ガベージコレクション、キャッシュによって、同じ入力でも時間が変わります。
遅いケースを性能のバグの証拠とみなす前に、そのケースを繰り返し実行してください。
探索の目的には、決定的な仕事量の数のほうが向いていることが多くあります。

ガイドの例は、2重ループの走査で行った比較の回数を測ります。
早い位置に重複がある入力はすぐに終わり、異なる値が多い入力ほど比較が増えます。

<!-- snippet: search_test.go#slow-input -->

`TestSearchFindsExpensiveScan` は、このプロパティを `Seed(1)` で実行します。
報告は次のとおりです。

```text
[pbt] property failed after 194 passed checks and 213 shrink steps (size 42)
[pbt] utility comparisons = 21
[pbt] xs (SliceOfN(IntRange(0, 9), 0, 10)) = []int{0, 1, 2, 3, 5, 4, 4}
[pbt] error: scan used 21 comparisons, limit 20
[pbt] reproduce: go test -run 'TestScanWork' -pbt.seed=0x1
```

実際のプロジェクトでは、経過時間も同じように扱えます。
測った時間を最大化し、許容できる上限をアサーションにします。
`Maximize` だけではケースは失敗しません。
通過した探索は最良の効用を報告しますが、反例は保存しません。
上限のアサーションがあれば遅いケースが失敗になり、通常の実行と同じように縮小されて保存されます。
ガイドのテストは、仕込んだ失敗をテストスイートに書き込まないように、記録用のTBを通して `NoSave` を付けています。

## コードカバレッジで探索する

`CoverageGuided()` か `-pbt.coverage` は、`testing.Coverage` を使う実験的な探索を有効にします。
デフォルトでは無効です。
`go test -cover` でビルドし、`-coverpkg` でカバレッジを数えるパッケージを選んでください。
計装されていなければ、pbtはヒントをログに出して通常の探索を使います。

生成したケースのうち、通過して実行中のブロック単位のカバレッジを上げたものが、最大64個の種のプールに入ります。
選択を持たないケースは、変異の種になれません。
プールに種が入ると、新しいケースと、ランダムに選んだ種の変異を交互に試します。
プールがいっぱいになると最も古い種を捨て、プールは1回の `Check` の間だけ保たれます。
変異は種のサイズを使い、新しいケースと同じく `Checks` に数えます。

この探索は、入れ子になった比較の連鎖で役に立ちます。
比較を1つ多く通過したケースは別のブロックに届き、次の比較を試す種になれます。
次の例は、4バイトのヘッダーを判定します。

<!-- snippet: coverage_fixture.go#coverage-fixture -->

<!-- snippet: search_test.go#coverage-guided -->

Goは `_test.go` のファイルを計装しないので、判定する関数は `coverage_fixture.go` に置いています。
テストは、バイナリが計装されていなければ自分で飛ばします。
リポジトリのルートから、このテストだけを実行してください。

```sh
go test -count=1 -v -cover -coverpkg=go.ymotongpoo.com/pbt/docs/guide/snippets ./docs/guide/snippets -run '^TestCoverageGuidedHeader$'
```

出力には次の行が含まれます。

```text
[pbt] coverage-guided: 4 of 200 passing cases reached new code, 4 seeds kept, coverage 0.0% -> 60.0%
[pbt] OK, passed 200 checks (seed 0x1)
```

割合は `testing.Coverage` による実行中の推定値です。
`go test` が最後に出す、文の数で重み付けした割合とは異なります。
コンパイラーの計装や先に走ったテストによって出力が変わるので、テストは割合の値ではなく報告の形を確かめています。

カバレッジによる誘導には限界があります。

- カウンターはプロセス全体のものです。先に走ったテストが役に立つ手がかりを使い切ることがあり、並行するテストや別のゴルーチンが、関係のないカバレッジをケースに帰属させることがあります。
- 手がかりになるのは到達したブロックで、エッジや実行回数はありません。すでに到達したブロックの中の比較からは、新しい手がかりが得られません。
- 1回の読み取りでカウンターを走査するので、計装したコードが多いほど手間が増えます。テスト対象のパッケージを選んでください。pbt自身を計装すると、pbtの処理も手がかりに入ります。
- シードだけでは、カバレッジの履歴が異なる実行で同じ探索の順序を再現できません。保存した反例は、これまでどおり選択を再実行します。

この手がかりを使うときは、プロパティを単独で実行し、`t.Parallel` を避けてください。
標的型探索が始まると、`Maximize` か `Minimize` が優先され、`SearchSteps` の上限が使われます。
`CheckExists` はカバレッジによる誘導を無視します。

## 存在性

`CheckExists` は問いを逆にします。
すべてのケースで成り立つべきプロパティではなく、プロパティが成り立つケースを1つ探します。
失敗したケースは却下された候補で、最初に通過したケースが証拠になります。
pbtはその証拠を縮小してログに出します。

<!-- snippet: targeted_test.go#exists -->

```text
[pbt] witness found after 98 rejected cases and 29 shrink steps (seed 0x1) (size 42)
[pbt] a (IntRange(1, 50)) = 3
[pbt] b (IntRange(1, 50)) = 4
[pbt] c (IntRange(1, 70)) = 5
```

チェック数の範囲で証拠が見つからないとテストは失敗します。
`CheckExists` の中でも `Maximize` は働きます。
通過するケースまでの距離を測る効用が、探索をそこへ導きます。
存在性のプロパティでは `testdata/pbt` に何も保存されません。
証拠は回帰ケースではないからです。
