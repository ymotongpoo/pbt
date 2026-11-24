# 標的型探索と存在性
<!-- summary: 効用に向けて実行を導くMaximizeとMinimize、複数の効用の同時探索、証拠が存在するかを問うプロパティのためのCheckExists。 -->

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
