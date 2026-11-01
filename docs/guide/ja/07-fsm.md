# 状態機械
<!-- summary: fsmパッケージ。名前付きの状態、重み付きの遷移、テストの変数に置く状態データ、遷移ごとの統計。 -->

システムが名前の付いた状態の間を移り、状態ごとに許される操作が異なるなら、有限状態機械が自然なモデルです。
`fsm` パッケージは5章のコマンドの仕組みの上に組んであります。
状態に名前を付け、各状態から出る遷移を重み付きで列挙すると、パッケージが機械に従うコマンド列を生成します。
PropErの `proper_fsm` に当たるものです。

## システム

<!-- snippet: fsm_test.go#account -->

## 機械

機械は、状態名からその状態で可能な遷移への関数です。
`fsm.Commands(initial, init, transitions)` は、最初の状態の名前、状態データを初期状態に戻す関数、そしてこの関数を受け取ります。
状態データがある機械では、5章のモデルと同じくプロパティの中の変数にデータを置きます。
データがなければ `init` は `nil` で構いません。

<!-- snippet: fsm_test.go#machine -->

各 `Transition` は `Weight`、遷移先の状態 `To`、そしてコマンドの生成器 `Cmd` を持ちます。
`Cmd` の型は `pbt.CommandGen` です。
コマンドの `Pre`、`Next`、`Post` が見るのは状態データです。
名前を動かすのはパッケージです。
`verified` での2回目の `verify` のように同じ状態にとどまる遷移は、拒否されるべき操作を機械で検査する方法です。

プログラムは5章と同じく `RunSequential` で実行し、`fsm.ParallelCommands` は6章の並行の形を与えます。
報告には失敗したステップの前の状態名が載り、`pbt.ModelState` で渡したものがその横に並びます。

## 状態ごとの統計

コマンドは遷移元と遷移先の状態名を持つので、通過した実行の要約には機械がどの遷移を取ったかが現れます。

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  32.7% verified -> locked: lock
[pbt]  31.5% locked -> verified: unlock
[pbt]  17.2% verified -> verified: verify
[pbt]  15.1% locked -> locked: lock
[pbt]   2.8% new -> verified: verify
[pbt]   0.7% new -> new: lock
```

`new` の割合が低いのは当然です。
すべての列は1つか2つのコマンドでこの状態を離れ、二度と戻りません。
重要な状態に列がほとんど到達していないと統計が示したら、そこへ向かう遷移の重みを上げてください。
