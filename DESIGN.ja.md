# Go向けプロパティベーステストライブラリ pbt の設計（草案）

2026年10月31日時点の設計です。
英語版の [DESIGN.md](DESIGN.md) を日本語に訳したもので、英語版が更新されたときに追随させます。
内容が食い違う場合は英語版に従ってください。
ライブラリ名は pbt、importパスは `go.ymotongpoo.com/pbt` です。
必要な設定は末尾の「リポジトリとモジュールの設定」にまとめています。

## 目的

**プロパティベーステスト**（PBT）は、入力をランダムに生成して性質（プロパティ）が成り立つかを確かめ、失敗したら入力を最小化してから報告するテスト手法です。
Goには[rapid](https://github.com/flyingmutant/rapid)と[gopter](https://github.com/leanovate/gopter)がありますが、Erlangの[PropEr](https://proper-testing.github.io/)やHaskellの[QuickCheck](https://hackage.haskell.org/package/QuickCheck)に比べると機能の幅がかなり狭い状態です。
本ライブラリの到達点を次のように置きます。

- PropErの機能（統計、シンボリック変数を伴うステートフルテスト、並行コマンドテスト、標的型PBT、存在性プロパティ、関数の生成、型からの生成器導出）をGoで使えるようにする。
- rapidの長所（ジェネリクスによる型安全な生成器、ユーザーコード不要の縮小、失敗の保存と自動再実行）は維持する。
- 標準の `testing` パッケージの延長として自然に使える。
- Fred Hebert（『Learn You Some Erlang for Great Good!』の著者）による『PropEr Testing』（[propertesting.com](https://propertesting.com/)で公開されている版。以下「本書」）の例題と演習を、すべてGoで書ける。

gopterはジェネリクス以前の `interface{}` ベースのAPIなので、比較の基準にはrapidを使います。

## 既存ライブラリとの機能比較

rapidの列は[pkg.go.devのv1.3.0のインデックス](https://pkg.go.dev/pgregory.net/rapid)で確認した範囲です。
Hegelの列は、[hegel](https://github.com/hegeldev/hegel)モノレポのコミット980d45fにあるGo版で確認した範囲です。
HegelはHypothesisのエンジンの上に作られたライブラリで、ネイティブライブラリを呼び出して動きます。
ephyraの列は、Datadogが公開したRustのライブラリ[ephyra](https://github.com/datadog-labs/ephyra)のv0.0.1（コミット3983b2e）で確認した範囲です。
pbtの列はv0.5.0の内容です。

| 機能 | PropEr | QuickCheck | rapid v1.3.0 | Hegel（Go） | ephyra | pbt |
| --- | --- | --- | --- | --- | --- | --- |
| 型安全な生成器 | －（動的型） | ○ | ○ | ○ | ○ | ○ |
| ユーザーコード不要の縮小 | △（独自型には縮小関数が必要） | △（`shrink` の実装が必要） | ○ | ○ | △（組み込みと導出した生成器に限る。`map` は縮小関数を失うので、それ以外の形には `with_shrink` が必要） | ○ |
| Bind（`?LET`）を跨ぐ縮小 | △（内側を縮小して外側を再生成） | ×（`>>=` で縮小が失われる） | ○ | ○ | △（`dependent`。`and_then` では縮小が失われる） | ○ |
| 縮小の制御（`?SHRINK`、`?LETSHRINK`） | ○ | ○ | × | × | ○（手書きの縮小関数を `check_shrinker` で検査する） | ○ |
| サイズパラメーター（`sized`、`resize`） | ○ | ○ | × | × | ○（`sized`、`max_size`） | ○ |
| 重み付き選択（`frequency`、`weighted_union`） | ○ | ○ | ×（`OneOf` は均等） | ×（`OneOf` は均等） | ○（`frequency`、`weighted`） | ○ |
| 分布の検証（`cover`、`checkCoverage`） | × | ○ | × | × | △（`cover` は固定の5%を下回ると警告する） | ○ |
| 統計（`collect`、`classify`、`aggregate`、`measure`） | ○ | ○ | × | △（イベント） | △（`classify`、`label`、`feature`、`observe`） | ○ |
| プロパティの合成（`conjunction`、`?IMPLIES`、`?WHENFAIL`、`?TIMEOUT`） | ○ | ○ | × | △（`Assume`、`Note`） | △（`assume!`、`TestResult`） | ○ |
| ステートフルテスト | ○ | ×（別ライブラリ） | ○ | ○（ルールのメソッドをリフレクションで見つける） | △（操作の列を生成した値として扱う） | ○ |
| シンボリック変数 | ○ | － | × | △（値のプール） | × | ○ |
| 並行コマンドテスト（交互配置の検査） | ○ | × | × | ×（不変条件を検査する並行ルール） | × | ○ |
| 標的型PBT | ○ | × | × | ○（ラベル付きの目的） | ○（`target`、探索ループ） | ○（複数の目的） |
| 存在性プロパティ（`?EXISTS`） | ○ | × | × | × | × | ○ |
| 関数の生成 | ○ | ○ | × | × | × | ○ |
| 型からの生成器導出 | ○（型宣言から） | △（総称導出） | ○（`Make[T]`） | ○（`Default`） | ○（`derive(Arbitrary)`、`any::<T>()`） | ○（リフレクションとコード生成） |
| 規格が定める文字列の形式の生成器 | × | × | △（`StringMatching`） | ○（メール、URL、ドメイン、IP、日付、正規表現） | × | ○（`format` パッケージ） |
| 実行のヘルスチェック | × | × | × | ○ | ○ | ○（警告。`CheckHealth` で失敗にする） |
| 1回の実行での複数の失敗 | × | × | × | ○ | ○（`failures(n)`） | ○（`MaxFailures`。`Signature` で原因に名前を付ける） |
| 関数仕様の検査（`check_spec`） | ○ | × | × | × | × | △（panicしないこと等に限る） |
| 失敗の保存と自動再実行 | △（プロセス内のみ） | ×（シードの再指定のみ） | ○ | ○（データベース） | ○（regressionsファイル、`EPHYRA_REPLAY`） | ○ |
| カバレッジ誘導ファザーとの連携 | × | × | ○（`MakeFuzz`） | × | －（独自のカバレッジ誘導探索を持つ。下の行） | ×（対象外） |
| 実行内のカバレッジ誘導探索 | × | × | × | × | ○（LLVMのカウンター、行単位） | △（`CoverageGuided`。`go test -cover` でのブロック単位のカバレッジ） |
| ステートフルテストの失敗を招いた手の特定 | × | × | × | × | ○（`Trace` を実装した値に対する `loops::causality`） | ○（`Causality`。手はコマンドの区間から取る） |
| カバレッジによるフォールトローカライゼーション | × | × | × | × | ○（疑わしい関数を順位付けする） | × |
| 失敗の識別子への命名 | × | × | × | × | ○（`signature`） | ○（`Signature`） |
| 実行全体の時間予算 | × | × | × | × | ○（`budget`） | ○（`Budget`） |
| 機械可読の報告とケースごとの観測記録 | × | × | × | × | ○（JSONの報告、OpenPBTStats） | ○（`ReportJSON`、OpenPBTStats形式の `Observations`） |
| ネイティブライブラリを使わない純粋なGo | － | － | ○ | ×（libhegelを読み込む） | －（Rust。カバレッジには `-C instrument-coverage` が必要） | ○ |

## testing.F の流用可否

Goのファジングはユーザーからは `testing.F` として見えますが、本体は `internal/fuzz` にあります。
Go 1.27.1のソースで確認した事実と、そこから導いた判断を表にまとめます。

| 観点 | 確認した事実 | 判断 |
| --- | --- | --- |
| パッケージの可視性 | `internal/fuzz` はユーザーのモジュールからimportできない | コピーする以外に流用の手段がない |
| 縮小（minimize）の対象 | `isMinimizable` が真を返すのは `string` と `[]byte` だけで、整数や浮動小数点数は縮小しない。アルゴリズムは末尾の切り詰めと1バイトずつの削除 | 構造を持つ値の縮小には使えない |
| 変異（mutate） | 全プリミティブ型に対応している | 標的型PBTの近傍関数を設計するときの参考になる |
| カバレッジ | `go test -fuzz` を導くエッジのカウンターは、ファジング用にビルドしたバイナリにしかない。`go test -cover` でビルドしたバイナリでは、実行したブロックの割合を `testing.Coverage()` で実行中のカウンターから読める | ファザーのフィードバックには届かないが、`go test -cover` のブロック単位のカバレッジで探索を導ける。`CoverageGuided` がこれを使う |
| プロセス構成 | コーディネーターとワーカーの別プロセス構成で、共有メモリとRPCで通信する | 1秒以内に終わるPBTの実行には重い |
| `F.Fuzz` の引数 | プリミティブ型と `string`、`[]byte` に限る | 構造を持つ値は `[]byte` から自前で組み立てる必要がある |

2026年10月31日の決定として、`testing.F` は流用も接続もしません。
縮小器は自前で実装し、失敗の保存先には `testdata/fuzz` と同じ考え方で `testdata/pbt/` を使います。
選択列はバイト列なので、`func(*testing.T, []byte)` を返す関数を1つ足せば `F.Fuzz` へ後から接続できますが、本ライブラリの範囲には含めません。

## 中核の設計判断としての縮小方式

**縮小**（shrinking）は、失敗した入力をより単純な入力に置き換えながら、失敗が再現する最小の入力を探す処理です。
縮小の方式がAPIの形と機能の上限を決めるので、最初に決めます。
候補は3つあります。

| 方式 | 代表 | 仕組み | Bindを跨ぐ縮小 | ユーザーの負担 | 値レベルの指定 |
| --- | --- | --- | --- | --- | --- |
| 型主導 | QuickCheck、PropEr、gopter | 生成器ごとに「値から縮小候補の列を返す関数」を持つ | ×（QuickCheckの `>>=` は縮小を失う。PropErの `?LET` は内側を縮小して外側を再生成するため、外側で進めた縮小を保てない） | 独自型には縮小関数を書く | ○ |
| 統合縮小 | [Hedgehog](https://hedgehog.qa/)、[falsify](https://hackage.haskell.org/package/falsify) | 生成器が値ではなく「値と縮小候補の木（遅延評価）」を返し、木がBindを通って合成される | ○ | 不要 | △（木を加工する） |
| 選択列 | [Hypothesis](https://hypothesis.readthedocs.io/)、rapid | 生成器は乱数源から取り出した「選択の列」の決定的な関数で、縮小は選択列に対して行い、値はそのたびに再生成する | ○ | 不要 | ×（Hypothesisは提供しない。pbtは後述の `ShrinkTo` と `LetShrink` で補う） |

pbtは**選択列**（choice sequence）方式を採ります。
理由は3点です。
Bind、Filter、ユーザーが書く任意の生成コードを跨いで縮小が成立します。
ステートフルテストのコマンド列も「コマンドの選択」と「引数の選択」の列になるので、コマンドの削除と引数の縮小が同じ仕組みで動きます。
選択列はバイト列なので、失敗の保存とシードからの再現が同じ表現で済みます。
rapidが、この方式がGoで実用になることを示しています。

この方式はHypothesisに由来しますが、採るのは縮小の仕組みだけです。
Hypothesisに同意できないのは、次節で述べるとおり分布の制御をユーザーに渡さない方針であって、仕組みの側ではありません。
統合縮小への切り替えは、Go 1.23の `iter.Seq` で遅延評価の木を書けば可能です。
しかし木を保持するメモリを引き受けることになり、保存した失敗にも選択列とは別の表現が要るので、採りません。

選択列では直接は表現できないPropErの機能と、その補い方を挙げます。

- `?LETSHRINK`（木を部分木へ縮小する）は、選択列に「どの生成器がどの区間を消費したか」をラベル付きで記録し、縮小器が「同じラベルの子孫区間で親区間を置き換える」ことで実現します。Hypothesisの `pass_to_descendant` と同じ考え方です。明示的に指定したい場合のために `LetShrink` も用意しています。部品をすべて生成したうえで「結合するか、部品のどれかをそのまま返すか」を1つの選択として記録する生成器で、生成時は常に結合し、縮小時にその選択が部品のどれかへ変わります。本書8章の `A ++ B ++ C` と木の例に対応します。
- サイズパラメーターは、Hypothesisとrapidでは公開されていませんが、pbtではPropErと同じく明示的に持ちます。実行器がテスト番号に応じて開始サイズから最大サイズへ増やし、縮小中はサイズを固定して選択列だけを縮めます。
- `?SHRINK`（別の生成器の値へ縮小させる）は、本書8章では年を1970年前後へ寄せ、タイムゾーンの分を0、15、30、45へ寄せる例に使われています。pbtでは `ShrinkTo(g, alts...)` として提供します。先頭に重み0の代替生成器を並べ、末尾に本来の生成器 `g` を置いた `Frequency` と同じ構造です。生成時は `g` だけが選ばれ、縮小時に添字が小さくなって代替生成器へ移ります。数値を特定の値へ寄せる用途には `IntRangeToward(lo, hi, center)` があります。
- 任意の値から選択列への符号化を要する一般の値レベル縮小は提供しません。上の2つで本書の用例はすべて書けます。

縮小器は、出発点となった失敗の同一性を保ちます。
テストケースは最初に失敗した場所を記録します。
アサーションならその呼び出し位置、パニックならその型と発生位置、ステートフルテストならどのコマンドのどの検査かです。
縮小器が候補を採用するのは、同じ同一性で失敗するときに限ります。
この規則がないと、大きな入力で見つかったパニックが、小さな入力で引っかかる無関係なアサーションへ縮小してしまい、探索が見つけたバグが報告から消えてしまいます。
Hypothesisが失敗のinteresting originと呼ぶものと同じ考え方です。

`NoShrink` は生成値の選択列を保護します。
再実行後、縮小器は保護された生成値の選択列が、前のケースの対応する選択列を変更せずに順序を保って抜き出したものかを確認します。
これにより、コレクションから保護された要素を削除しつつ、選択列の削除や位置のずれによって残る要素が別の値に再生成されるのを防ぎます。

実行器はクリーンアップの前にプロパティの実行結果を記録し、各クリーンアップの実行結果も個別に記録します。
失敗は破棄より優先され、この規則は `Conjunction` の各プロパティの間でも適用されます。
クリーンアップのパニックが、プロパティですでに記録されたパニックを置き換えることはありません。

## 設計原則としての分布の制御

Hypothesisには `one_of` の各選択肢に重みを与える手段がありません。
2022年7月に筆者がhypothesis-usersでPropErの `frequency/1` に相当する機能の有無を尋ねたとき、メンテナーのZac Hatfield-Doddsは[返信](https://groups.google.com/d/msgid/hypothesis-users/c67575ea-16c7-41f0-9e88-e16c1d589f8cn%40googlegroups.com)で、提供しない理由を3点挙げています。
確率の指定はAPIを大きくして学習を難しくすること、ユーザーの注意を入力の全域ではなく特定の種類のデータへ向けてしまうこと、バグはユーザーが考えなかった場所にあるのでHypothesis（HypoFuzzやCrossHairで補強したもの）のほうが入力の探索で人間を上回ること、の3点です。

pbtはこの方針を採りません。
長時間走らせるステートフルテストでは、読み取りと書き込みの比率や、エラー経路へ入る頻度を決めなければ、到達したい状態に辿り着かないことが多いためです。
PropErとQuickCheckが `frequency` と `sized` を公開し、同時に `collect` や `classify` で分布を観察する手段を用意しているのは、分布の責任をユーザーが持つという前提の表れです。
分布を渡す以上、それを確かめる手段も渡します。
QuickCheckの `cover` と `checkCoverage` に相当する機能も含めます。
「この分類が通過したケース全体の20%以上を占める」と宣言する機能です。
100%未満の要求は統計的に判定します。
観測した割合の99.7%Wilson区間の下限が宣言した割合以上なら充足、上限が宣言した割合未満なら不足、それ以外は未決です。
100%の要求は統計的な区間を使わず、通過したすべてのケースでそのラベルに該当すれば充足、1ケースでも該当しなければ不足です。
`CheckCoverage` を付けない場合、`Check` は不足をログに出すだけです。
付けた場合は、すべての宣言が決まるまで、設定した回数の20倍を上限にテストケースを走らせ続け、不足のときも証拠が足りないままのときも失敗にします。
通過したなら、実行が短かったからではなく、データがその割合を裏付けたことになります。

エンジンが探索で人間を上回るという論点には、標的型PBTで対応します。
ユーザーが効用関数を書き、エンジンがそれを最大化する入力を探す形なので、探索の方向もユーザーが決めます。

選択列方式で重みを扱うときの約束も決めてあります。
重みは生成時の確率だけを決め、縮小は重みに関係なく先頭の選択肢へ向かいます。
つまり `Frequency` の選択肢の並び順が縮小の優先順位です。
プリミティブ型の生成器が境界値へ偏らせるのはデフォルトの挙動として残し、`Frequency` と `Just` の組み合わせで上書きできる形にしています。

## 診断、複数の失敗、規格の形式

v0.4.0では、HegelとHypothesisから4つの考え方を取り込み、ここまでの設計原則に合わせました。
いずれもエンジンを純粋なGoのまま保っています。

ヘルスチェックは、`Cover` と `CheckCoverage` と同じ組み合わせに従います。
通過した実行では警告を出し、オプションを付けると警告を失敗に変えます。
`HealthFilter` は、捨てたケースを、捨てた `Filter`、`Assume`、`Skip` ごとに報告します。
`HealthLarge` は、選択を多く使うケースを、その選択を使った取り出しごとに報告します。
`HealthSlow` は生成の遅さを報告しますが、CIのマシンの性能はまちまちなので、名前を指定しない限り実行を失敗にしません。
失敗したときは、縮小の前に `Check` がそのケースを1回だけ再実行します。
再実行で失敗が再現しなければ `HealthFlaky` の注記を付けますが、縮小と保存はそのまま行います。
並行テストの競合は本質的に再現が不安定だからです。
縮小が予算で止まったときは `HealthShrink` の注記を付けます。

`MaxFailures(n)` は失敗の後も実行を続け、識別子の異なる失敗を集めます。
識別子は、アサーションの位置か、panicの型と位置です。
デフォルトの1は従来の挙動を保ち、報告が1件であることを前提にしたexamplesの `internal/expect` もそのまま動きます。

縮小器は選択肢を切り替えられるようになりました。
`OneOf` の添字や、要素より前に引く長さのような、区間の最初の選択を下げると、その区間が使う選択の数が変わります。
縮小器は候補を再実行し、新しい区間が閉じた時点で、古い区間の後ろにあった選択をつなぎ直します。
区間は、それより前に始まった区間の数で特定します。
その区間より前の選択は変わらないので、この数は変わりません。

規格の形式は、`pbt` の公開APIだけで組んだ `format` パッケージに置きます。
標準ライブラリの型の生成器は、`Make` と `cmd/pbtgen` が使うので `pbt` に置きます。
サブパッケージは、自分が依存するパッケージからimportできないからです。
名前の付け方は、`time.RFC3339` のようなGoのレイアウト定数に倣います。
規格名の関数は、パーサーが受け付けなければならない生成規則の選択肢をすべて出します。
平易な名前の関数は、プログラムが書き出すべき一般的な形か正規形を出します。
どの生成器も値を絞り込まずに文法から組み立てるので、縮小の候補も常に妥当な値です。

Hegelの、`Rule` と `Invariant` のメソッドをリフレクションで集める仕組み、値のプール、ワークロードの実行器、エンジンのフェーズの選択、Antithesis向けの乱数源は取り込みませんでした。
明示的な `Command` の値がルールの役割を表せること、`Var` がプールの役割を担うこと、`TB` によって `go test` の外でも `Check` を実行できること、必要なフェーズは既存のオプションで選べることが理由です。

## ephyraから取り入れた考え方

[ephyra](https://github.com/datadog-labs/ephyra)は、Datadogが2026年10月に公開したRustのライブラリです（v0.0.1、コミット3983b2eで確認）。
コーディングエージェントがテストを書き、検証することを前提に作られています。
縮小についての立場はpbtと逆です。
利用者が生成器ごとに縮小関数と変異関数を書き、ライブラリはその両方が生成器の値域から外れないことを検査します。
pbtは選択列方式を保つので、縮小の候補は生成器が出した値であり、この検査は必要ありません。
以下の機能はこの違いに左右されないので、v0.5.0で取り込みました。

因果分析は、Antithesisとephyraの `loops::causality` に倣います。
失敗したステートフルなプログラムをk個のコマンドの後で切り、残りを新しい乱数で何度も引き直して、同じ識別子で失敗する継続の割合をいくつかのkについて測ります。
割合が最も大きく上がった区間を、1手になるまで二分して絞ります。
ephyraでは、接頭辞の後から値を再開する方法を `Trace` として実装する必要があります。
選択列では接頭辞は選択の接頭辞そのもので、各コマンドの終わりもすでに記録されているので、`Causality(n)` は利用者に何も求めません。
分析は縮小の前に、見つかったままのケースに対して行います。
最小のプログラムには、バグを仕込んだコマンドと検出したコマンドの間の距離が残らないからです。
並行プログラムは、生成の順序が実行の順序と一致しないので対象外にします。

`T.Signature` を使うと、`ephyra::signature` と同じように、プロパティが失敗の識別子に名前を付けられます。
識別子はアサーションの位置かpanicの位置で、縮小器がどの候補を受け入れるかと、`MaxFailures` が失敗をどうまとめるかを決めます。
いくつもの形で失敗する往復の検査では、位置は1つでも原因は複数あります。
名前を付ければ、原因を分けて扱えます。

ephyraのregressionsファイルと `low_distinct_inputs` の警告から、ヘルスチェックを2つ取り込みます。
`HealthStale` は、保存したケースに記録した取り出しと、その再実行での取り出しを比べます。
違いがあれば、生成器が変わり、再実行が別の入力を検査していることになります。
保存したケースは1つの入力を再実行するためにあるので、このチェックはデフォルトで警告します。
`HealthDuplicate` は、生成したケースのうち選択列が異なるものの数を数え、半分を下回ると警告します。
サイズはキーに含めません。
サイズはケースごとに増えるので、キーに含めるとデフォルトの実行のほとんどのケースが別物として数えられてしまいます。
小さな定義域のプロパティは意図して同じ入力を繰り返すので、このチェックは `WarnHealth` か `CheckHealth` で名前を指定するまで動きません。

`Budget`、`ReportJSON`、`Observations` は、長時間の実行や無人の実行のためのものです。
予算は一定の時間が過ぎるとケースの生成を止め、そのことを要約に書きます。
実行ごとに1行のJSONを書くので、プログラムやコーディングエージェントは報告の文章を解析せずに結果を得られます。
JSONのスキーマには `go.ymotongpoo.com/pbt/report/v1` というバージョンを付けます。
観測記録はTycheとHypothesisが使うOpenPBTStats形式で書くので、既存のビューアーで読めます。
pbtが書くのはファイルだけです。
ログの転送エージェントを使えばファイルをバックエンドへ送れるので、ライブラリにネットワークのコードは足しません。

カバレッジ誘導探索は最小限の実装です。
`go test -cover` でビルドしたテストバイナリでは、計装したパッケージの実行中のカバレッジを `testing.Coverage()` で読めます。
値は実行したブロックの割合です。
`CoverageGuided()` は通過したケースのたびにこの値を読み、値を上げたケースを種として残し、新しいケースと種の変異を交互に試します。
ただし、手がかりになるシグナルは粗いものです。
ブロック単位のカバレッジなのでヒット回数もエッジもなく、カウンターはプロセス全体で共有され、1回の読み取りの手間はカウンターの数に比例します。
それでも、ランダムな生成では届かない比較の連鎖を通り抜けます。
シード1では、0から15の値に対する6段の入れ子のバイト比較を、誘導探索は52,114ケース目で通り抜けました。
誘導しない探索は、10万ケースを使っても届きませんでした。
これは `go test -fuzz` の代わりにはなりません。

ephyraのほかの機能は取り込みません。

- 手書きの縮小関数と変異関数、`check_shrinker`、`ephyra check` は、選択列方式が構造上保証していることを検証するものです。
- `Run` を使った利用者自身の探索ループと、プロジェクトのフィクスチャーから作る種のプールには、値から選択列へ戻る手段が必要です。PropErの `?USERNF` を先送りにしているのと同じ理由です。
- フォールトローカライゼーションは、通過した実行と失敗した実行のカバレッジから関数を順位付けします。関数ごとのカウンターが必要ですが、`testing.Coverage()` はそれを返しません。読むには `runtime/coverage` のファイルを解読する必要があり、その形式はGoの内部仕様です。
- 実行を可視化するページと、Datadogへメトリクスを送るシンクは、サーバーと特定のベンダーに依存します。JSONのファイルがあれば、どちらにも依存せずに同じ用途を満たせます。

ガイドには、pbtでプロジェクトをテストするコーディングエージェント向けの章を足します。
ephyraのSTARTUP.mdに当たる章で、正しさの定義、オラクルの選び方、失敗の読み方、何も失敗しないときに変えるものを扱います。

## APIの骨格

rapidと同じく、プロパティ本体の中で生成器から値を取り出す命令的な形を基本にします。
Goには可変長の型パラメーターがないので、QuickCheck式に `ForAll` へ生成器を並べる形は引数の数ごとに関数を用意することになります。
`ForAll`、`ForAll2`、`ForAll3` は糖衣として提供します。
PropErのマクロに相当する合成は、`Property` を値として扱う関数で提供します。
以下は実装済みのAPIの一覧で、詳細はパッケージのドキュメントを参照してください。

```go
package pbt

// 生成器。選択列を消費して値を返す決定的な関数。
type Gen[V any] struct{ /* 非公開 */ }
func (g Gen[V]) Draw(t *T, label string) V
func (g Gen[V]) Example(seed ...uint64) V                 // proper_gen:pick

func Int() Gen[int]
func IntRange(lo, hi int) Gen[int]
func IntRangeToward(lo, hi, center int) Gen[int]          // 数値に対する ?SHRINK の再中心化
func Integer[I integer]() Gen[I]                          // IntegerRange、IntegerRangeToward も
func Float64() Gen[float64]                               // Float64Range、Float32、Float32Range も
func AnyFloat64() Gen[float64]                            // Float64 に NaN と無限大を加えたもの
func Bool() Gen[bool]
func Rune() Gen[rune]
func String() Gen[string]                                 // StringN、StringOf、StringOfN も
func Bytes() Gen[[]byte]                                  // BytesN も
func SliceOf[E any](elem Gen[E]) Gen[[]E]                 // SliceOfN、SliceOfDistinct、SliceOfNDistinct も
func MapOf[K comparable, V any](k Gen[K], v Gen[V]) Gen[map[K]V] // MapOfN も
func Ptr[E any](elem Gen[E], allowNil bool) Gen[*E]
func Permutation[E any](values []E) Gen[[]E]
func SampledFrom[E any](values []E) Gen[E]                // elements
func Any() Gen[any]                                       // term()。値域は限定
func Just[V any](v V) Gen[V]                              // exactly
func Custom[V any](name string, f func(*T) V) Gen[V]
func Map[A, B any](g Gen[A], f func(A) B) Gen[B]
func Bind[A, B any](g Gen[A], f func(A) Gen[B]) Gen[B]   // ?LET
func Filter[V any](g Gen[V], pred func(V) bool) Gen[V]   // ?SUCHTHAT
func OneOf[V any](gs ...Gen[V]) Gen[V]                   // union
type Weighted[V any] struct{ Weight int; Gen Gen[V] }
func Frequency[V any](ws ...Weighted[V]) Gen[V]          // weighted_union。重み0は縮小でのみ到達する
func W[V any](weight int, g Gen[V]) Weighted[V]           // 型を推論させる Weighted
func Sized[V any](f func(size int) Gen[V]) Gen[V]        // ?SIZED
func Resize[V any](size int, g Gen[V]) Gen[V]            // resize。サイズを固定し、明示した長さの制約は保つ
func Lazy[V any](f func() Gen[V]) Gen[V]                 // ?LAZY
func ShrinkTo[V any](g Gen[V], alts ...Gen[V]) Gen[V]    // ?SHRINK
func LetShrink[V any](parts []Gen[V], combine func([]V) V) Gen[V] // ?LETSHRINK
func NoShrink[V any](g Gen[V]) Gen[V]                    // noshrink
func SampleShrink[V any](g Gen[V], seed ...uint64) iter.Seq[V]    // proper_gen:sampleshrink
func Func[A comparable, B any](ret Gen[B]) Gen[*Fn[A, B]] // function/2。Fn.Call で呼ぶ
func Func2[A, B comparable, C any](ret Gen[C]) Gen[*Fn2[A, B, C]]
func AsAny[V any](g Gen[V]) Gen[any]                     // 型の違う生成器を OneOf でまとめるための拡大
func Make[V any](opts ...MakeOption) Gen[V]              // リフレクションによる型からの導出。pbt 構造体タグ、WithType、WithField
func Recursive[V any](leaf Gen[V], extend func(self Gen[V]) Gen[V]) Gen[V] // 予算付きの再帰
func Bounded[V any](leaf, node Gen[V]) Gen[V]            // 値1つにつき1つの再帰予算を型を跨いで共有する
func NoPanic(f any, opts ...MakeOption) Property           // check_spec。シグネチャから分かる範囲に限る
func BoundedGrowth[V any](g Gen[V]) Gen[V]               // Make の成長の予算。1つの値あたり 8*max(size, 1) 単位
func UnboundedGrowth[V any](g Gen[V]) Gen[V]             // 囲む予算から一部を外す
func Time() Gen[time.Time]                               // と TimeRange。暦の境目とゾーンの切り替わりに寄る
func Duration() Gen[time.Duration]                       // と DurationRange
func FixedZone() Gen[*time.Location]                     // と IANA のゾーンを読む Zone(names...)
func IPv4() Gen[netip.Addr]                              // と IPv6、Addr、AddrIn(prefixes...)。RFC 6890 のブロックを含む
func Prefix() Gen[netip.Prefix]                          // と AddrPort、HardwareAddr
func RuneIn(tables ...*unicode.RangeTable) Gen[rune]     // どの範囲も同じ確率で選ぶ
func StringMatching(pattern string) Gen[string]          // 文字列全体が正規表現に一致する
// cmd/pbtgen: go run go.ymotongpoo.com/pbt/cmd/pbtgen -type T1,T2 [-prefix Gen] が、go build が対象にするファイルから
// GenT1、GenT2 をソースとして書き出す。パッケージが既に宣言している名前はエラーになる

// プロパティ。
type Property func(*T)
type TB interface { Helper(); Name() string; Logf(string, ...any); Errorf(string, ...any) } // *testing.T が満たす
func Check(tb TB, prop Property, opts ...Option)
func CheckExists(tb TB, prop Property, opts ...Option)   // ?EXISTS
func ForAll[A any](g Gen[A], body func(*T, A)) Property  // ForAll2、ForAll3 も
func Conjunction(subs ...Named) Property                 // conjunction
func Deadline(d time.Duration, prop Property) Property   // ?TIMEOUT。プロパティが戻った後に判定する。Timeout は非推奨の別名
func Recover(f func()) (panicked any)
func Causality(continuations int) Option                 // ステートフルテストの失敗がほぼ確実になったコマンドを示す。-pbt.causality
func Budget(d time.Duration) Option                      // 探索の時間予算。-pbt.budget
func ReportJSON(path string) Option                      // 実行ごとに1行のJSON。スキーマは go.ymotongpoo.com/pbt/report/v1。-pbt.json
func Observations(dir string) Option                     // Tyche向けのOpenPBTStatsの記録。テストごとに1ファイル。-pbt.observe
func CoverageGuided() Option                             // 実験的。go test -cover で testing.Coverage から種を作る。-pbt.coverage
func Checks(n int) Option                                // numtests
func Seed(seed uint64) Option
func Sizes(start, max int) Option                        // start_size、max_size
func ShrinkTime(d time.Duration) Option
func SearchSteps(n int) Option                           // search_steps
func Setup(f func(t *T) (teardown func())) Option        // ?SETUP
func CheckCoverage() Option                              // checkCoverage
func NoSave() Option
func TestdataDir(dir string) Option
func ReplayFile(path string) Option                      // 保存済みのケースを1つ先に再実行する。フラグ -pbt.replay も同じ
func NoReplay() Option                                   // testdata/pbt の保存済みケースを再実行しない
func Verbose() Option                                    // 通過した実行の要約と、採用した縮小の各ステップを出す。-pbt.v
func ReportLimit(chars int) Option                       // 報告に載せる値1つの文字数。デフォルトは2000
func ReportDepth(levels int) Option                      // 報告に載せる入れ子のラベル付き値の深さ。デフォルトは3
func MaxFailures(n int) Option                           // 集める失敗の数。-pbt.failures
type HealthCheck uint8                                   // HealthFilter、HealthLarge、HealthSlow、HealthFlaky、HealthShrink、HealthStale、HealthDuplicate
func CheckHealth(checks ...HealthCheck) Option           // 警告を失敗にする。-pbt.checkhealth
func WarnHealth(checks ...HealthCheck) Option            // HealthDuplicate を警告として有効にする。-pbt.warnhealth
func IgnoreHealth(checks ...HealthCheck) Option          // 警告を隠す。-pbt.ignorehealth
func (t *T) Assume(cond bool)                            // ?IMPLIES
func (t *T) Classify(cond bool, label string)            // classify
func (t *T) Collect(v any)                               // collect
func (t *T) Aggregate(values ...any)                     // aggregate
func (t *T) Measure(name string, v float64)              // measure
func (t *T) Cover(cond bool, pct float64, label string)  // cover
func (t *T) OnFail(f func())                             // ?WHENFAIL
func (t *T) Signature(name string)                       // ケースの失敗の識別子に名前を付ける
func (t *T) Maximize(name string, utility float64)       // ?MAXIMIZE。名前ごとに1つの目的。Minimize は符号を反転する
func (t *T) Size() int
// ほかに Errorf、Error、Fatalf、Fatal、Fail、FailNow、Failed、Logf、Log、Skip、Skipf、SkipNow、Cleanup

// ステートフルテスト。
type Var struct{ /* 非公開 */ }                          // {var, N}。Env で解決する
type Env struct{ /* 非公開 */ }
func (e Env) Get(v Var) any
func Resolve[T any](env Env, v Var) T
type Command struct {                                    // precondition、呼び出し、next_state、postcondition
	Name string; Args []any                              // 関数はテストのモデルとシステムを参照するクロージャ
	Pre  func() bool
	Run  func(env Env) any
	Next func(result Var)                                // 生成時も実行時もシンボリック
	Post func(env Env, result any) error
}
type CommandGen = Gen[Command]                           // コマンドの生成器の型
func Commands(init func(), gen func() CommandGen, opts ...StatefulOption) Gen[Program]
func NoCommands() CommandGen                             // 列を終える
func Length(min, max int) StatefulOption                 // 列、または並行プログラムの前段の長さを制限する
func Branches(n, maxLen int) StatefulOption              // 並行プログラムの形
func ModelState(f func() any) StatefulOption             // 報告にモデルを表示する
func (p Program) RunSequential(t *T)
func ParallelCommands(init func(), gen func() CommandGen, opts ...StatefulOption) Gen[ParallelProgram]
func (p ParallelProgram) RunParallel(t *T)
```

`format` パッケージは文字列を返します。
生成器の一覧はパッケージのドキュメントにあり、ここでは名前の付け方を示す例だけを挙げます。

```go
package format // go.ymotongpoo.com/pbt/format

func Time(layout string) pbt.Gen[string]                  // Goのレイアウト定数に従って time.Time.Format が書く文字列
func RFC3339() pbt.Gen[string]                            // うるう秒を含むdate-timeの生成規則全体
func Email() pbt.Gen[string]                              // 検証器が受け付ける形
func RFC5321Mailbox() pbt.Gen[string]                     // と RFC5322AddrSpec。メールのパーサーが受け付けるべき生成規則
func URL() pbt.Gen[string]                                // と RFC3986URI、RFC3986Reference
func JSON() pbt.Gen[string]                               // と IJSON、UUID、SemVer、RFC4180CSV、RFC5646LanguageTag など
```

本書11章の `proper_fsm` に相当する層は、サブパッケージ `fsm` にあります。
現在の状態名を追跡し、状態データはモデルと同じくテストの変数に置き、状態ごとに列挙した遷移に重みを付けて（`weight/3`）、`Command` の仕組みの上で動かします。
プログラムと統計では各コマンドに遷移元と遷移先の状態名を付けます。
`zip(state_names, command_names)` と同じ見え方です。

```go
package fsm // go.ymotongpoo.com/pbt/fsm

type Transition struct {
	Weight int                                            // weight/3。0 は縮小でのみ到達する
	To     string
	Cmd    pbt.CommandGen                                 // データに対する precondition/4、next_state_data/5、postcondition/5
}
func Commands(initial string, init func(), transitions func(state string) []Transition, opts ...pbt.StatefulOption) pbt.Gen[pbt.Program]
func ParallelCommands(initial string, init func(), transitions func(state string) []Transition, opts ...pbt.StatefulOption) pbt.Gen[pbt.ParallelProgram]
```

実行時のオプションはフラグ `-pbt.checks`、`-pbt.seed`、`-pbt.startsize`、`-pbt.maxsize`、`-pbt.shrinktime`、`-pbt.searchsteps`、`-pbt.nosave`、`-pbt.replay`、`-pbt.v`、`-pbt.failures`、`-pbt.checkhealth`、`-pbt.warnhealth`、`-pbt.ignorehealth`、`-pbt.causality`、`-pbt.budget`、`-pbt.json`、`-pbt.observe`、`-pbt.coverage` で与え、`Check` に渡したオプションが優先されます。
本書の `rebar3 proper -n`、`-p`、`--start_size` は、順に `-pbt.checks`、`go test -run`、`-pbt.startsize` に対応します。

ステートフルテストで**シンボリック変数**（`Var`）を使うのは、コマンド列の生成時にはまだコマンドが実行されておらず、結果が存在しないためです。
PropErでは生成時に `{var, N}` としてモデル状態から結果を参照させ、実行時に実際の値へ解決します。
Goでも同じく、`Var` と、実行時に値を引く環境 `Env` の組で表します。
コマンドごとに結果の型が違い、1つの生成器が何種類ものコマンドを返すので、`Var` は型パラメーターを持たず、型付きの取り出しはジェネリック関数 `Resolve[T]` が行います。
`Next` は生成時も実行時も `Var` を受け取るので、モデルが実際の結果に依存することはなく、縮小はコマンド列を再生成できます。
`Post` は実際の結果と `Env` を受け取ります。
コマンドの生成器はモデルを読むので、`Frequency` の重みを状態に応じて変えられます。
PropErの `command/1` の中で `frequency` を使う書き方に対応します。

『PropEr Testing』の第10章では、縮小時に不正なコマンドを事前条件で拒否する必要性を説明しています。
pbtでは、コマンドを削除すると、新しいモデル状態を使って後続のコマンドの生成器を呼び直します。
`Pre` は縮小中も生成された候補を絞り込み、試行回数の上限に達すれば列を終了し、最小の長さに達していない場合はケースを破棄します。
`RunSequential` は各呼び出しの前に `Pre` を再検査し、偽ならケースを失敗にします。
`ParallelCommands` も `Pre` を使い、すべての交互配置が有効になるまで枝の末尾からコマンドを取り除きます。
書店の例は、モデルに基づく選択と事前条件による絞り込みを併用しています。
既知のISBNを選ぶだけでは、貸し出せる在庫があるとは限らないためです。

失敗時には、実行したコマンドと結果、失敗したステップの前のモデル状態を実行器が表示します。
本書9章で `?WHENFAIL` の中に手で書いている内容と同じものです。

v0.3.0以降、ステートフルテストのAPIには型パラメータがありません。
モデルとテスト対象のシステムはテスト関数の変数に置き、コマンドはそれを参照するクロージャで、`init` がモデルを初期状態に戻します。
pbtは `init` を呼んでから各コマンドの `Next` を再生することでモデルを巻き戻します。
プログラムは短いのでこの再生は軽く、並行の検査では交互配置の候補ごとに1回行います。
最初の設計ではモデルを値として `NextState` に通し、モデルとシステムの型をすべてのコマンドに引き回していました。
そのためどのテストも `Cmd[Model, *System]`、`Weighted[Command[Model, *System]]`、`fsm.Transition[Name, Data, *System]` と書くことになり、読みやすさを保つには型の別名が要りました。
生成器の型パラメータは残しています。
`Draw` が型付きの値を返すことがプロパティから型アサーションをなくしており、型パラメータはほとんどの呼び出しで推論に隠れるためです。
推論が届かないのは、コマンドの生成器を返す関数の署名です。
`iter.Seq[V]` を返すメソッドの署名に型引数を書くのと同じです。
そこには `Gen[Command]` の別名である `CommandGen` を書きます。
`Map` や `Frequency` が返す生成器と同じ型です。

並行コマンドテストはPropErの `parallel_commands` に倣います。
生成器が「逐次の前置き」と「最大3本の並行枝」からなるプログラムを作り、実行器が枝をゴルーチンで実行します。
その後、記録した履歴の交互配置を列挙して、逐次モデルが受け入れる配置が1つでもあるかを調べます。
この検査は各枝の中での順序だけを守り、枝どうしの実時間の前後関係は見ません。
PropErも同じ方式で、線形化可能性の検査より弱い検査です。
コマンドは合計12個までなので総当たりで十分です。

PropErの標的型PBTは、値に対する近傍関数を使う焼きなまし法です。
選択列では、これまでで最良の選択列を少し変異させることが近傍の生成にあたり、`internal/fuzz` の変異器がその参考になります。
同じ探索が存在性プロパティも実装し、見つかった証拠は縮小してから報告します。

## 優先順位

| フェーズ | 内容 | 完了の判定 |
| --- | --- | --- |
| 0 | 中核エンジン。ラベル付き区間を持つ選択列、`math/rand/v2` の ChaCha8 による乱数、実行器（回数、サイズの増加、シード）、縮小器（区間の削除、ゼロ化、個別の値の二分探索、隣接する値の再配分、子孫区間への置換）、panicの回収、ラベル付きの Draw 列としての最小反例の報告、`testdata/pbt/` への保存と自動再実行 | rapidの公開テストを移植し、同等以上に小さい反例へ縮小できる |
| 1 | 生成器一式（PropErの `proper_types` に対応）、関数の生成、プロパティの合成、統計の出力と分布の検証、縮小の制御（`ShrinkTo`、`ShrinkToward`、`LetShrink`、`NoShrink`、`SampleShrink`） | 本書1章から8章の例題と演習がすべて移植できる |
| 2 | ステートフルテスト（逐次）、シンボリック変数、`Setup`、縮小中のコマンドの再生成と事前条件による絞り込み、履歴の表示、`fsm` パッケージ | 本書9章の逐次キャッシュ、10章の書店、11章のサーキットブレーカーが移植できる |
| 3 | 並行コマンドテスト（交互配置の検査）。レースディテクターと `testing/synctest` との併用 | 本書9章のキャッシュに仕込んだ競合が検出される |
| 4 | 標的型PBT、存在性プロパティ | PropErのドキュメントにある標的型PBTの例を移植できる（本書のオンライン版にこの章はない） |
| 5 | 型からの導出（リフレクションによる `Make[T]`、構造体タグによる制約、`go/types` を使った `go generate` でのコード生成）、関数仕様の検査 | 再帰型を含む構造体の生成器が自動で得られる |
| 6 | 縮小過程の可視化、ドキュメント、rapidとの性能比較 | 対応表付きの移行ガイドがある |
| 7 | 診断と規格の形式。報告に載るラベルなしの取り出しの名前と行、ヘルスチェック、1回の実行での複数の失敗、複数の目的、選択肢を切り替える縮小、標準ライブラリの型の生成器、`StringMatching`、`format` パッケージ、`Make` の仕事量の上限 | ガイド10章で、標準ライブラリのパーサーとRFCとの違いを示せる |
| 8 | ephyraから取り入れた考え方。ステートフルテストの失敗の因果分析、プロパティが名前を付ける失敗の識別子、保存したケースの陳腐化と入力の重複を報告するヘルスチェック、時間予算、JSONの報告とOpenPBTStatsの観測記録、`go test -cover` でのカバレッジ誘導探索、コーディングエージェント向けのガイドの章 | コマンド列の早い位置で仕込み、遅い位置で検出されたバグを、仕込んだコマンドまで辿れる |

第0フェーズの完了判定にrapidを使うのは、同じ方式を採る実装どうしでは差が縮小の質に直接現れるためです。
2026年10月31日時点で、第0フェーズから第4フェーズまでは実装とテストが済んでいます。
並行コマンドテストと標的型探索を含みます。
本書の全章が `examples/` に移植されています。
第5フェーズも揃っています。
リフレクション版の `Make[T]`（構造体タグと上書き付き）、`go/types` で組んだコード生成器 `cmd/pbtgen`（同じ生成器をソースとして書き出し、非公開フィールドも埋められる）、再帰を予算で抑える `Recursive`、シグネチャだけから検査する `check_spec` の Go 版である `NoPanic` です。
rapidが自身の縮小品質に課しているテストは `parity_test.go` で再現し、MIGRATION.md がrapidのAPIをpbtに対応づけ、`bench/` モジュールが両者を比較した結果を BENCHMARK.md に記録しています。
10章の移植は `PBT_BOOKSTORE_DSN` を設定するとPostgreSQLでも走ります。
その実行でSQLiteとの差異が2つ見つかりました。`LIKE` のエスケープ文字がデフォルトでバックスラッシュであること、列幅が強制されることの2つで、章のREADMEに記録してあります。
可視化は詳細ログに縮小の各ステップを出すところまでです。
2026年11月24日時点で、v0.4.0に向けて第7フェーズを実装しました。
Hegelが備える診断と規格の形式の生成器を取り込み、エンジンは純粋なGoのまま保っています。
2026年11月25日時点で、v0.5.0に向けて第8フェーズを実装しました。
内容は「ephyraから取り入れた考え方」の節に書いたとおりです。

## Go固有の利点

Go固有の機能で、PropErやQuickCheckにはない形で使えるものを挙げます。

- ジェネリクスにより生成器を型付きにできます。メソッドは型パラメーターを持てないので、`Map` や `Bind` はトップレベル関数にします。
- Go 1.23の `iter.Seq` は、縮小候補や統計の列挙を遅延列として扱うのに向いています。統合縮小へ方針転換する場合の木の表現にもなります。
- `math/rand/v2` の ChaCha8 はシード付きで決定的に動き、ストリームの分割も容易です。
- レースディテクターは並行コマンドテストと併用できます。
- `testing/synctest` はGo 1.25で安定化し、仮想時間の中でタイムアウトを含む性質を決定的に試験できます。rapidもv1.3.0で `SyncTest` を追加しています。
- `go/types` と `go generate` を使えば、PropErの `proper_typeserver` が実行時に行う「型宣言からの生成器導出」を、コード生成として型安全に行えます。

対応バージョンは最新の安定版から2つ前までです（2026年10月31日に決定）。
2026年10月時点ではGo 1.25、1.26、1.27が対象で、`go.mod` の `go` ディレクティブは1.25にしています。
新しい安定版が出るたびに引き上げます。

## 本書の問題への対応

本書のオンライン版は11章からなり、Elixir向けの記述を除けば、例題と演習は次の表の範囲に収まります。
各章の例題と演習は `examples/` 配下にGoで移植してあり、フェーズの完了判定に使います。
説明を求める演習（4章のQ1とQ2、5章のQ1からQ3、8章のQ1とQ2、10章のQ1、Q2、Q4、Q5）はコードを伴わないので、各章のREADMEにGoでの答えを書いています。

| 章 | 例題と演習 | 必要な機能 | pbtでの対応 |
| --- | --- | --- | --- |
| 1 | レジの釣り銭 `cash/3` の性質（合計の一致、大きい単位から返す）。反例がクラッシュを起こす | `ForAll`、panicの回収、反例の表示 | 第0フェーズ |
| 2 | `rebar3 proper -n`、`-p`、`rebar3 as test shell` での生成器の試用 | 回数の指定、プロパティの選択、生成器の試用 | `-pbt.checks`、`go test -run`、`Gen.Example` |
| 3 | `biggest/1` のモデル、`sort` の不変条件、`last` の一般化、encode と decode の対称性 | 基本の生成器 | 第1フェーズ |
| 4 | デフォルト生成器の表、`prop_biggest`（空リストで失敗して `non_empty` へ）、`[-1,6,3,1]` から `[0,1]` への縮小、`prop_sort`、`prop_last`、演習Q3からQ7（`lists:seq`、`keysort`、`sets:union` のモデル修正、`dict:merge`、`word_count`） | PropErのデフォルト生成器に対応する一式、`NonEmpty`、`Any` | 第1フェーズ。`term()` は基本型と複合型を値域とする `Any()` に対応させ、ソートの例は `[]int` のような型付きの値で書く |
| 5 | `collect` と `aggregate`（バケット分け）、`resize`、`?SIZED`、生成器の関数化、`?LET`、`?SUCHTHAT`、`frequency`（`text_like`）、`?LAZY` による再帰（`wordy_string`）、決定的なサイズ付き再帰、シンボリック呼び出し（`dict_symb`）、演習Q4からQ6（木の終了、タイムスタンプの対、ファイルハンドルの生成） | 統計、サイズ、`Bind`、`Filter`、`Frequency`、`Lazy`、反例の可読性 | 第1フェーズ。シンボリック呼び出しはラベル付きの Draw 列の表示で代替する。反例は最終的な値だけでなく、構築に使った入力も表示する |
| 6 | CSVの往復、`"\r\na"` の反例、1列のときの曖昧さ（列数を `Size+1` にする）、ヘッダーとレコードでサイズを共有する入れ子の `?LET`、`date()`、例示テストとの併用、カバレッジ | 基本機能、再現実行 | 第1フェーズ。例示テストは `testing` の通常のテストとして同じファイルに置き、カバレッジは `go test -cover` で取る |
| 7 | 段階的な `?LET`（価格表から商品列と期待値へ）、`elements(PriceList)`、`?SIZED` の再帰、`collect` のバケット、負のテスト（結果か特定のエラーか）、重複の生成（`vector(length+1, elements)`）、92%の実行が無効な入力だったという発見 | `Bind`、統計、`Cover` | 第1フェーズ。エラーはGoの `error` 値で受け、panicは `Recover` で値にする |
| 8 | `?SHRINK` による再中心化（年を1970年へ、タイムゾーンの分を0、15、30、45へ）、`?LETSHRINK`（木から部分木へ、`A ++ B ++ C`、演習Q3の meal、演習Q4の checkout の独立した縮小）、`sampleshrink` | 縮小の制御と観察 | 第1フェーズ。`ShrinkTo`、`ShrinkToward`、`LetShrink`、`SampleShrink` |
| 9 | statem のコールバック、キャッシュのモデル、`?WHENFAIL` での履歴と状態の表示、`aggregate(command_names)`、バグの注入、`parallel_commands`、`erlang:yield()` による競合の顕在化、`+T9` | `Command`、履歴の表示、コマンド名の統計、並行実行と交互配置の検査 | 第2フェーズと第3フェーズ。`runtime.Gosched()` と `-race` を併用する |
| 10 | PostgreSQLの書店、ISBNの生成器、`friendly_unicode`、shimによる意図のラベル（`new`、`exists`、`avail`）、`partial/1`、事前条件が偽を返して縮小が成立する場面、`--start_size`、起動と停止の配置（演習Q3）、実結果を `next_state` で使う注意（演習Q4） | 縮小中のコマンドの再生成と事前条件による絞り込み、`Setup`、`Var`、`-pbt.startsize` | 第2フェーズ。shimのラベルはコマンド名に含めて統計に出す |
| 11 | `proper_fsm` の状態名、`weight/3`、`precondition/4`、`postcondition/5`、`next_state_data/5`、`state_names` による統計、`?TRAPEXIT`、サーキットブレーカー（ok、tripped、blocked）とモデルの修正 | `fsm` パッケージ | 第2フェーズ |

Erlang固有で、Goでは別の形になるものを挙げます。

- アトムは文字列定数か型付きの定数で書きます。
- `term()` は値域を限った `Any()` にします。Goでは比較と整列が型ごとに定義されるので、本書で `list(term())` を使う例は型付きの値で書きます。
- シンボリック呼び出しは、反例を読みやすくする目的と、縮小を引数に対して行う目的の2つで使われています。前者はラベル付きの Draw 列の表示で、後者は選択列方式そのもので賄います。
- `?TRAPEXIT` はpanicの回収で代替します。ゴルーチンのリークは検知しません。
- `+T9` によるスケジューラーの攪乱は、`runtime.Gosched()` の挿入と `-race` で代替します。
- `rebar3 proper` は `go test` です。`rebar3 new proper base` のような雛形生成は用意しません。

10章の書店は、`go test` だけで完結するように、pure Goで書かれたSQLite実装（[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)）で動かし、`PBT_BOOKSTORE_DSN` でデータベースを指定したときは pgx ドライバー経由でPostgreSQLでも動かします。
本書で見つかる不具合のうち、`LIKE` のワイルドカード（`%` と `_`）と `available` の上限はSQLiteでも再現します。
NULバイトの扱いはドライバー固有なので再現しません。
何が再現するかは章のREADMEに書いてあります。
11章のサーキットブレーカーは、Klarnaの `circuit_breaker` のAPIを模した小さな実装を `examples/` 内に置いています。
ok、tripped、blockedの3状態、エラーとタイムアウトのしきい値、手動のblock、deblock、resetを持ちます。
Goの既存ライブラリは時間ベースの回復を前提にしていて手動のブロックがなく、本書のモデルと合わないためです。

## リポジトリとモジュールの設定

ルートモジュールは外部依存を持たず、本書の移植は別モジュールにしています。

```text
go.ymotongpoo.com/pbt        ルートモジュール。外部依存なし
├── go.mod
├── LICENSE.md               Apache License 2.0
├── fsm/                     proper_fsm に相当（11章）
├── format/                  規格が定める形式の文字列
└── examples/                別モジュール go.ymotongpoo.com/pbt/examples。本書の各章の移植
    └── go.mod
```

ルートの `go.mod` は次のとおりです。
`go mod init` は手元のツールチェーンのバージョン（1.27.1）を書くので、手で1.25に直しています。

```text
module go.ymotongpoo.com/pbt

go 1.25
```

importパス `go.ymotongpoo.com/pbt` を使うには、`https://go.ymotongpoo.com/pbt?go-get=1` が `go-import` メタタグを含むHTMLを返す必要があります。
`go get` とモジュールプロキシ（proxy.golang.org）はこのURLを取得し、メタタグに書かれたリポジトリを clone します。
サブパッケージ `go.ymotongpoo.com/pbt/fsm` を取得するときは `/pbt/fsm?go-get=1` を取得し、接頭辞 `go.ymotongpoo.com/pbt` の確認のために `/pbt?go-get=1` も取得します。
したがって `/pbt` 配下のどのパスでも同じHTMLを返すようにします。

サイトは www.ymotongpoo.com と同じCloudflare Pagesの別プロジェクトで、カスタムドメインに `go.ymotongpoo.com` を割り当てています。
DNSのCNAMEはカスタムドメインの設定が作ります。
配信するファイルはリポジトリ [github.com/ymotongpoo/go.ymotongpoo.com](https://github.com/ymotongpoo/go.ymotongpoo.com) にあり、このドメインで公開するすべてのモジュールをそこで扱います。
モジュールの一覧から、`go-import` タグを持つモジュールのページ、索引、ドメインの `llms.txt`、`_redirects` と `_headers` の規則を生成し、各モジュールが描画したドキュメントは `public/guide/<module>/` にコピーします。
pbtのガイドは本リポジトリの `site/` で描画してからそこへコピーします。
手順はAGENTS.mdにあります。
サイトは当初、本リポジトリの孤立ブランチでしたが、ガイドとエージェント向けのファイルが加わってバニティパスのページより大きくなったため、独立させました。

`_redirects` は、`/pbt` 配下のすべてのパスを同じHTMLで応答させる書き換えです。
Cloudflare Pagesの `_redirects` はステータス200の書き換え（proxying）に対応していて、splat（`*`）は1行に1つだけ書けます。

```text
/pbt    /pbt/index.html 200
/pbt/*  /pbt/index.html 200
```

`pbt/index.html` は次のとおりです。
リポジトリは `github.com/ymotongpoo/pbt` です。
`go-source` はpkg.go.devがソースへリンクするためのもので、取得には影響しません。
人間が開いたときはpkg.go.devへ転送します。

```html
<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="go-import" content="go.ymotongpoo.com/pbt git https://github.com/ymotongpoo/pbt">
<meta name="go-source" content="go.ymotongpoo.com/pbt https://github.com/ymotongpoo/pbt https://github.com/ymotongpoo/pbt/tree/main{/dir} https://github.com/ymotongpoo/pbt/blob/main{/dir}/{file}#L{line}">
<meta http-equiv="refresh" content="0; url=https://pkg.go.dev/go.ymotongpoo.com/pbt">
<title>go.ymotongpoo.com/pbt</title>
</head>
<body><a href="https://pkg.go.dev/go.ymotongpoo.com/pbt">pkg.go.dev/go.ymotongpoo.com/pbt</a></body>
</html>
```

同じメタタグで `examples/` も別モジュール `go.ymotongpoo.com/pbt/examples` として解決できます。
goコマンドはリポジトリ内の `examples/go.mod` を探し、タグは `examples/v0.1.0` の形になります。

公開後の確認は、`v0.1.0` のようなタグを打ってから次の手順で行います。
proxy.golang.orgは最初の要求時に取得してキャッシュするので、タグを打ち直した場合は新しいバージョン番号を使います。

```sh
curl -sL 'https://go.ymotongpoo.com/pbt?go-get=1' | grep go-import
cd "$(mktemp -d)" && go mod init example.com/check && go get go.ymotongpoo.com/pbt@latest
```

pkg.go.devはライセンスを認識できるモジュールだけドキュメントを表示します。
Apache License 2.0は認識の対象です。
メジャーバージョンを2以上に上げるときは、モジュールパスを `go.ymotongpoo.com/pbt/v2` にします。

## 決定事項

最初の8項目は2026年10月31日に、次の5項目は2026年11月24日に、最後の5項目は2026年11月25日に決めました。

| 項目 | 決定 |
| --- | --- |
| Hypothesisへの立場 | 不同意は分布の制御をユーザーに渡さない方針に対するもので、縮小の仕組みではない。縮小は選択列方式で進め、分布の制御と検証を設計原則に置く |
| `testing.F` | 流用も接続もしない |
| 失敗の保存先 | `testdata/pbt/<テスト名>/<ハッシュ>`。`go test` のたびに自動で再実行する |
| 対応バージョン | 最新の安定版から2つ前まで。現時点ではGo 1.25以上 |
| 名前とimportパス | パッケージ名 `pbt`、モジュールパス `go.ymotongpoo.com/pbt` |
| ライセンス | Apache License 2.0 |
| rapid互換レイヤー | 設けない |
| 到達目標 | 本書の例題と演習をすべて移植できること |
| 規格の形式の置き場所 | 標準ライブラリの型の生成器は `Make` が使うので `pbt` に置き、文字列の形式はサブパッケージ `format` に置く |
| 規格の形式の名前 | 規格名の関数は生成規則を網羅し、平易な名前は一般的な形を出す。`format.Time` はGoのレイアウト定数を受け取る |
| ヘルスチェック | デフォルトは警告。`CheckHealth` と `-pbt.checkhealth` で失敗にする。`HealthSlow` は名前を指定したときだけ |
| 複数の失敗 | `MaxFailures`。デフォルトは1 |
| 登録簿のデータ | ISO 3166、ISO 4217、ISO 639、IBANの国別の長さのように登録簿に従う表は、v0.4.0には入れない |
| 利用者が書く縮小関数 | ephyraからは取り込まない。選択列方式では候補がいつも生成器の値域に収まるので、手書きの縮小関数を検査しても見つかるものがない |
| 因果分析 | `Causality(n)`。デフォルトは無効。縮小の前の失敗ケースに対して行い、逐次のプログラムに限る |
| カバレッジ | `CoverageGuided` は `go test -cover` で `testing.Coverage` を読む。フォールトローカライゼーションと、`runtime/coverage` のファイルの解読は行わない |
| プログラム向けの出力 | JSON Linesのファイルと、OpenPBTStatsのファイル。ネットワークへの送信は行わない |
| 新しいヘルスチェックのデフォルト | `HealthStale` はデフォルトで警告する。`HealthDuplicate` は `WarnHealth` か `CheckHealth` で名前を指定するまで動かない |

v0.1.0の前に行ったレビューで、2点を先送りにしました。

- 標的型探索でユーザーが近傍関数を定義する手段（PropErの `?USERNF`）です。探索は選択列を変異させますが、近傍関数は値に対して働きます。これを適用するには、新しい値から選択列へ戻る手段、つまりどの選択がどの値を生んだかの記録が要りますが、エンジンはそれを持っていません。その設計ができるまで、`Maximize` と `Minimize` は効用だけで探索を導きます。
- ステートフルテストの型付きシンボリック変数 `Ref[T]` です。`Var` が型を持たないのは、1つの生成器が多種のコマンドを返し、モデルがそれらの結果を並べて保持するためで、型付きの参照は実行時に `Resolve[T]` が行います。型付き変数にすると検査がコンパイル時へ移りますが、結果を保持するすべてのコマンドに型パラメーターが付くことになり、本書の例題には必要ありません。

次の文書は第0フェーズの仕様で、次の4点を扱います。

- 選択列の表現
- 区間のモデル
- 縮小の各操作の順序と打ち切り条件
- `testdata/pbt/` のファイル形式
