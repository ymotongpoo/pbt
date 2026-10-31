# 状態機械

システムが名前の付いた状態の間を移り、状態ごとに許される操作が異なるなら、有限状態機械が自然なモデルです。
`fsm` パッケージは5章のコマンドの仕組みの上に組んであります。
状態に名前を付け、各状態から出る遷移を重み付きで列挙すると、パッケージが機械に従うコマンド列を生成します。
PropErの `proper_fsm` に当たるものです。

## システム

<!-- snippet: fsm_test.go#account -->

## 機械

機械は `fsm.Machine[N, D, Sys]` を実装します。
`N` は状態名の型、`D` は名前とともに運ぶ状態データ、`Sys` はテスト対象のシステムです。
`Initial` は最初の状態を返し、`Transitions` はある状態で可能な遷移を返します。

<!-- snippet: fsm_test.go#machine -->

各 `Transition` は `Weight`、遷移先の状態 `To`、そして状態データに対するコマンドの生成器 `Cmd` を持ちます。
コマンドの `Pre`、`Next`、`Post` が見るのはデータで、名前ではありません。
名前を動かすのは機械です。
`verified` での2回目の `verify` のように同じ状態にとどまる遷移は、拒否されるべき操作を機械で検査する方法です。

`fsm.Commands(m)` はプログラムの生成器を返します。
そのモデル状態は名前とデータの組 `fsm.State[N, D]` です。
プログラムは5章と同じく `RunSequential` で実行し、`fsm.ParallelCommands` は6章の並行の形を与えます。

## 状態ごとの統計

コマンドには遷移元の状態のラベルが付くので、通過した実行の要約には機械がどの遷移を取ったかが現れます。

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  32.7% verified: lock
[pbt]  31.5% locked: unlock
[pbt]  17.2% verified: verify
[pbt]  15.1% locked: lock
[pbt]   2.8% new: verify
[pbt]   0.7% new: lock
```

`new` の割合が低いのは当然です。
すべての列は1つか2つのコマンドでこの状態を離れ、二度と戻りません。
重要な状態に列がほとんど到達していないと統計が示したら、そこへ向かう遷移の重みを上げてください。
