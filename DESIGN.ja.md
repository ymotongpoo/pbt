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
pbtの列は本書面での提案です。

| 機能 | PropEr | QuickCheck | rapid v1.3.0 | pbt（案） |
| --- | --- | --- | --- | --- |
| 型安全な生成器 | －（動的型） | ○ | ○ | ○ |
| ユーザーコード不要の縮小 | △（独自型には縮小関数が必要） | △（`shrink` の実装が必要） | ○ | ○ |
| Bind（`?LET`）を跨ぐ縮小 | △（内側を縮小して外側を再生成） | ×（`>>=` で縮小が失われる） | ○ | ○ |
| 縮小の制御（`?SHRINK`、`?LETSHRINK`） | ○ | ○ | × | △（部分木への縮小は自動。値レベルの指定は後続） |
| サイズパラメーター（`sized`、`resize`） | ○ | ○ | × | ○ |
| 重み付き選択（`frequency`、`weighted_union`） | ○ | ○ | ×（`OneOf` は均等） | ○ |
| 分布の検証（`cover`、`checkCoverage`） | × | ○ | × | ○ |
| 統計（`collect`、`classify`、`aggregate`、`measure`） | ○ | ○ | × | ○ |
| プロパティの合成（`conjunction`、`?IMPLIES`、`?WHENFAIL`、`?TIMEOUT`） | ○ | ○ | × | ○ |
| ステートフルテスト | ○ | ×（別ライブラリ） | ○ | ○ |
| シンボリック変数 | ○ | － | × | ○ |
| 並行コマンドテスト（交互配置の検査） | ○ | × | × | ○ |
| 標的型PBT | ○ | × | × | ○ |
| 存在性プロパティ（`?EXISTS`） | ○ | × | × | ○ |
| 関数の生成 | ○ | ○ | × | ○ |
| 型からの生成器導出 | ○（型宣言から） | △（総称導出） | ○（`Make[T]`） | ○（リフレクションとコード生成） |
| 関数仕様の検査（`check_spec`） | ○ | × | × | △（panicしないこと等に限る） |
| 失敗の保存と自動再実行 | △（プロセス内のみ） | ×（シードの再指定のみ） | ○ | ○ |
| カバレッジ誘導ファザーとの連携 | × | × | ○（`MakeFuzz`） | ×（対象外） |

## testing.F の流用可否

Goのファジングはユーザーからは `testing.F` として見えますが、本体は `internal/fuzz` にあります。
Go 1.27.1のソースで確認した事実と、そこから導いた判断を表にまとめます。

| 観点 | 確認した事実 | 判断 |
| --- | --- | --- |
| パッケージの可視性 | `internal/fuzz` はユーザーのモジュールからimportできない | コピーする以外に流用の手段がない |
| 縮小（minimize）の対象 | `isMinimizable` が真を返すのは `string` と `[]byte` だけで、整数や浮動小数点数は縮小しない。アルゴリズムは末尾の切り詰めと1バイトずつの削除 | 構造を持つ値の縮小には使えない |
| 変異（mutate） | 全プリミティブ型に対応している | 標的型PBTの近傍関数を設計するときの参考になる |
| カバレッジ | `go test -fuzz` で計装ビルドされたときだけ取れる | 通常の `go test` 中にライブラリが参照できる情報ではない |
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

## 設計原則としての分布の制御

Hypothesisには `one_of` の各選択肢に重みを与える手段がありません。
2022年7月に筆者がhypothesis-usersでPropErの `frequency/1` に相当する機能の有無を尋ねたとき、メンテナーのZac Hatfield-Doddsは[返信](https://groups.google.com/d/msgid/hypothesis-users/c67575ea-16c7-41f0-9e88-e16c1d589f8cn%40googlegroups.com)で、提供しない理由を3点挙げています。
確率の指定はAPIを大きくして学習を難しくすること、ユーザーの注意を入力の全域ではなく特定の種類のデータへ向けてしまうこと、バグはユーザーが考えなかった場所にあるのでHypothesis（HypoFuzzやCrossHairで補強したもの）のほうが入力の探索で人間を上回ること、の3点です。

pbtはこの方針を採りません。
長時間走らせるステートフルテストでは、読み取りと書き込みの比率や、エラー経路へ入る頻度を決めなければ、到達したい状態に辿り着かないことが多いためです。
PropErとQuickCheckが `frequency` と `sized` を公開し、同時に `collect` や `classify` で分布を観察する手段を用意しているのは、分布の責任をユーザーが持つという前提の表れです。
分布を渡す以上、それを確かめる手段も渡します。
QuickCheckの `cover` と `checkCoverage` に相当する機能も含めます。
「この分類が全体の20%以上を占める」と宣言する機能です。
判定は統計的に行います。
観測した割合の99.7%Wilson区間が宣言した割合を上回れば充足、下回れば不足、またがっていれば未決です。
`CheckCoverage` を付けない場合、`Check` は不足をログに出すだけです。
付けた場合は、すべての宣言が決まるまで、設定した回数の20倍を上限にテストケースを走らせ続け、不足のときも証拠が足りないままのときも失敗にします。
通過したなら、実行が短かったからではなく、データがその割合を裏付けたことになります。

エンジンが探索で人間を上回るという論点には、標的型PBTで対応します。
ユーザーが効用関数を書き、エンジンがそれを最大化する入力を探す形なので、探索の方向もユーザーが決めます。

選択列方式で重みを扱うときの約束も決めてあります。
重みは生成時の確率だけを決め、縮小は重みに関係なく先頭の選択肢へ向かいます。
つまり `Frequency` の選択肢の並び順が縮小の優先順位です。
プリミティブ型の生成器が境界値へ偏らせるのはデフォルトの挙動として残し、`Frequency` と `Just` の組み合わせで上書きできる形にしています。

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
func Resize[V any](size int, g Gen[V]) Gen[V]            // resize。サイズ0ではコレクションが空になる
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
func (t *T) Assume(cond bool)                            // ?IMPLIES
func (t *T) Classify(cond bool, label string)            // classify
func (t *T) Collect(v any)                               // collect
func (t *T) Aggregate(values ...any)                     // aggregate
func (t *T) Measure(name string, v float64)              // measure
func (t *T) Cover(cond bool, pct float64, label string)  // cover
func (t *T) OnFail(f func())                             // ?WHENFAIL
func (t *T) Maximize(name string, utility float64)       // ?MAXIMIZE。Minimize は符号を反転する
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
func Commands(init func(), gen func() Gen[Command], opts ...StatefulOption) Gen[Program]
func NoCommands() Gen[Command]                           // 列を終える
func Length(min, max int) StatefulOption                 // 列、または並行プログラムの前段の長さを制限する
func Branches(n, maxLen int) StatefulOption              // 並行プログラムの形
func ModelState(f func() any) StatefulOption             // 報告にモデルを表示する
func (p Program) RunSequential(t *T)
func ParallelCommands(init func(), gen func() Gen[Command], opts ...StatefulOption) Gen[ParallelProgram]
func (p ParallelProgram) RunParallel(t *T)
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
	Cmd    pbt.Gen[pbt.Command]                           // データに対する precondition/4、next_state_data/5、postcondition/5
}
func Commands(initial string, init func(), transitions func(state string) []Transition, opts ...pbt.StatefulOption) pbt.Gen[pbt.Program]
func ParallelCommands(initial string, init func(), transitions func(state string) []Transition, opts ...pbt.StatefulOption) pbt.Gen[pbt.ParallelProgram]
```

実行時のオプションはフラグ `-pbt.checks`、`-pbt.seed`、`-pbt.startsize`、`-pbt.maxsize`、`-pbt.shrinktime`、`-pbt.searchsteps`、`-pbt.nosave`、`-pbt.replay`、`-pbt.v` で与え、`Check` に渡したオプションが優先されます。
本書の `rebar3 proper -n`、`-p`、`--start_size` は、順に `-pbt.checks`、`go test -run`、`-pbt.startsize` に対応します。

ステートフルテストで**シンボリック変数**（`Var`）を使うのは、コマンド列の生成時にはまだコマンドが実行されておらず、結果が存在しないためです。
PropErでは生成時に `{var, N}` としてモデル状態から結果を参照させ、実行時に実際の値へ解決します。
Goでも同じく、`Var` と、実行時に値を引く環境 `Env` の組で表します。
コマンドごとに結果の型が違い、1つの生成器が何種類ものコマンドを返すので、`Var` は型パラメーターを持たず、型付きの取り出しはジェネリック関数 `Resolve[T]` が行います。
`Next` は生成時も実行時も `Var` を受け取るので、モデルが実際の結果に依存することはなく、縮小はコマンド列を再生成できます。
`Post` は実際の結果と `Env` を受け取ります。
コマンドの生成器はモデルを読むので、`Frequency` の重みを状態に応じて変えられます。
PropErの `command/1` の中で `frequency` を使う書き方に対応します。
縮小中はコマンドを削除するたびに事前条件を再評価し、満たさない列は反例の候補から外します。
本書10章がこの性質に依存しています。事前条件が偽を返すようにして初めて縮小が成立する場面があるためです。
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
| 2 | ステートフルテスト（逐次）、シンボリック変数、`Setup`、縮小中の事前条件の再評価、履歴の表示、`fsm` パッケージ | 本書9章の逐次キャッシュ、10章の書店、11章のサーキットブレーカーが移植できる |
| 3 | 並行コマンドテスト（交互配置の検査）。レースディテクターと `testing/synctest` との併用 | 本書9章のキャッシュに仕込んだ競合が検出される |
| 4 | 標的型PBT、存在性プロパティ | PropErのドキュメントにある標的型PBTの例を移植できる（本書のオンライン版にこの章はない） |
| 5 | 型からの導出（リフレクションによる `Make[T]`、構造体タグによる制約、`go/types` を使った `go generate` でのコード生成）、関数仕様の検査 | 再帰型を含む構造体の生成器が自動で得られる |
| 6 | 縮小過程の可視化、ドキュメント、rapidとの性能比較 | 対応表付きの移行ガイドがある |

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
| 10 | PostgreSQLの書店、ISBNの生成器、`friendly_unicode`、shimによる意図のラベル（`new`、`exists`、`avail`）、`partial/1`、事前条件が偽を返して縮小が成立する場面、`--start_size`、起動と停止の配置（演習Q3）、実結果を `next_state` で使う注意（演習Q4） | 縮小中の事前条件の再評価、`Setup`、`Var`、`-pbt.startsize` | 第2フェーズ。shimのラベルはコマンド名に含めて統計に出す |
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
配信するファイルは本リポジトリの孤立ブランチ `pages`（`main` と履歴を共有しない）にあります。

```text
_redirects
pbt/index.html
index.html        ルート用。go-import タグを持たない
README.md         ブランチの説明と配信手順
```

Pagesのプロジェクトを本リポジトリに接続して本番ブランチを `pages` にするか、このブランチのチェックアウトから `wrangler pages deploy` で直接アップロードします。
Git連携にする場合は、`main` がサイトとして配信されないように、プレビューの対象ブランチを無しにします。

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

2026年10月31日に次を決めました。

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

v0.1.0の前に行ったレビューで、2点を先送りにしました。

- 標的型探索でユーザーが近傍関数を定義する手段（PropErの `?USERNF`）です。探索は選択列を変異させますが、近傍関数は値に対して働きます。これを適用するには、新しい値から選択列へ戻る手段、つまりどの選択がどの値を生んだかの記録が要りますが、エンジンはそれを持っていません。その設計ができるまで、`Maximize` と `Minimize` は効用だけで探索を導きます。
- ステートフルテストの型付きシンボリック変数 `Ref[T]` です。`Var` が型を持たないのは、1つの生成器が多種のコマンドを返し、モデルがそれらの結果を並べて保持するためで、型付きの参照は実行時に `Resolve[T]` が行います。型付き変数にすると検査がコンパイル時へ移りますが、結果を保持するすべてのコマンドに型パラメーターが付くことになり、本書の例題には必要ありません。

次の文書は第0フェーズの仕様で、次の4点を扱います。

- 選択列の表現
- 区間のモデル
- 縮小の各操作の順序と打ち切り条件
- `testdata/pbt/` のファイル形式
