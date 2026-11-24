# 生成器
<!-- summary: 時刻、ネットワークアドレス、Unicodeの文字クラス、正規表現を含む組み込みの生成器、Customによる自作、Map、Bind、Filter、OneOf、Frequencyによる合成、報告に載る引き値の読み方、プロパティの外での標本抽出。 -->

生成器は `pbt.Gen[V]` 型の値です。
pbtが現在のケースについて記録している選択から `V` を作る方法を知っていて、その記録が縮小と再現を支えています。
記録を自分で組むことはありません。
生成器を組み合わせて、そこから値を取り出すだけです。

## 組み込みの生成器

| 値 | 生成器 |
| --- | --- |
| 整数 | `Int`、`IntRange`、`IntMin`、`IntMax`、`IntRangeToward`。`Int32`、`Int64`、`Uint`、`Uint32`、`Uint64`、`Byte` とそれぞれの `Range` 版。任意の整数型には `Integer[I]` と `IntegerRange[I]` |
| 浮動小数点数 | 有限の値には `Float64`、`Float64Range`、`Float32`、`Float32Range`。`AnyFloat64` はNaNと無限大を加える |
| 文字と文字列 | `Bool`、`Rune`、`RuneIn`、`String`、`StringN`、`StringOf`、`StringOfN`、`StringMatching`、`Bytes`、`BytesN` |
| 時刻 | `Time`、`TimeRange`、`Duration`、`DurationRange`、`FixedZone`、`Zone` |
| ネットワーク | `net/netip` の型を返す `IPv4`、`IPv6`、`Addr`、`AddrIn`、`Prefix`、`AddrPort` と、`HardwareAddr` |
| コレクション | `SliceOf`、`SliceOfN`、`SliceOfDistinct`、`SliceOfNDistinct`、`MapOf`、`MapOfN`、`Permutation` |
| 選択 | `Just`、`SampledFrom`、`OneOf`、`Frequency`、`Ptr` |
| 関数 | `Func`、`Func2`。コールバックを受け取るコードのプロパティ向けに、純粋な関数を生成する |
| 任意の値 | 一般的な型を限られた範囲で混ぜる `Any` と、型から生成器を導く `Make[T]`（9章） |

整数は、バグが潜みやすい範囲の境界と小さな値に寄ります。
文字列とコレクションの長さは、ケースのサイズパラメーターに従います。
最大長の `-1` は上限なしを意味します。
RFC 3339のタイムスタンプ、メールアドレス、URIなど、規格が定める形式の文字列は、パッケージ `go.ymotongpoo.com/pbt/format` が生成します。
このパッケージは10章で説明します。

## 時刻とアドレス

時刻とアドレスの生成器も整数と同じ方針で、コードが誤動作しやすい値に寄ります。
`Time` は、`time.Time` のゼロ値から9999年の終わりまでの時刻を生成します。
そのうち半分近くは暦の境目へ移します。
境目とは、日、月、年の最初または最後のナノ秒、2月29日、範囲の端です。
保存されたタイムスタンプの多くはマイクロ秒、ミリ秒、秒の単位までしか持たないので、一部のケースでは秒未満の端数をそれらの単位に丸めます。
1か月後の日付を求める計算は、こうした値で誤動作する典型例です。

<!-- snippet: standard_test.go#billing -->

```text
[pbt] property failed after 39 passed checks and 13 shrink steps (size 40)
[pbt] start (Time()) = time.Date(1970, time.January, 31, 23, 59, 59, 999999999, time.UTC)
[pbt] error: next billing date 1970-03-03 is not in February
[pbt] reproduce: go test -run 'TestBilling' -pbt.seed=0x1
```

`AddDate` は1月31日の1か月後を正規化して3月3日にします。
実行は月の最後のナノ秒を見つけました。
縮小器はそれを、`Time` の縮小先であるUnixエポックの月、1970年1月へ移しました。
`TimeRange(lo, hi)` は、範囲内の値を `lo` のロケーションで生成します。
夏時間のあるロケーションでは、オフセットが切り替わる時刻の近くにも値を寄せます。
切り替わりの瞬間、その1ナノ秒前、前後の30分です。
`Zone` はIANAのタイムゾーンを読み込み、デフォルトでは規則の変わったゾーンの一覧から選びます。
`FixedZone` は固定のオフセットを生成します。

アドレスの生成器は `net/netip` の型を返します。
`IPv4` と `IPv6` は、アドレスの一部をRFC 6890の特殊用途ブロックから生成します。
ループバック、プライベートネットワーク、リンクローカル、ドキュメント用、マルチキャスト、IPv6ではさらにIPv4射影アドレスがこれに含まれます。
`AddrIn` は、渡したプレフィックスの中のアドレスだけを生成します。

<!-- snippet: standard_test.go#network -->

## パターンに一致する文字列

`StringMatching` は、パッケージ `regexp` の構文で書いた正規表現から文字列を組みます。
どの値もパターン全体に一致し、縮小しても一致したままです。
`|` で並べた分岐は前の分岐へ、繰り返しは最小の回数へ、文字クラスはその中で最初の印字可能なASCII文字へ縮みます。
`RuneIn` は、スクリプトやカテゴリなど、パッケージ `unicode` の範囲表からルーンを生成します。
表の各範囲が同じ確率で選ばれるので、小さなスクリプトがCJK統合漢字に埋もれることはありません。

<!-- snippet: standard_test.go#matching -->

## 自分の生成器を作る

`pbt.Custom` は `*pbt.T` を受け取る関数を生成器にします。
その中では、他の生成器から任意の順序と制御構造で値を取り出せます。

<!-- snippet: generators_test.go#user -->

`Custom` に渡した名前が、報告でのこの生成器の表記になります。
中で行う取り出しのラベルにも役割があります。
生成器の中でラベルを付けて取り出した値は、組み上げた値の下に字下げして報告に並びます。
`""` でラベルを付けなかった値は報告に出ません。
それ自体には関心のない値には、この扱いが向いています。

## 生成器を組み合わせる

組み合わせの大半は3つの関数で書けます。

<!-- snippet: generators_test.go#compose -->

- `Map` は各値に関数を適用します。結果は入力と同じように縮小します。
- `Filter` は述語を満たさない値を捨てて引き直します。大半の値を捨てる述語は実行を遅くし、pbtは何度も捨てたケースを諦めます。`Filter(Int(), positive)` より `IntRange(1, 100)` のように、欲しい値を直接作るほうを選んでください。
- `Bind` は値から次の生成器を選びます。値の一部を別の部分に依存させる手段で、PropErでは `?LET` と呼びます。

`OneOf` は複数の生成器から等確率で1つを選び、`Frequency` は重みで選びます。どちらも2章で扱います。
`Ptr` は値をポインタで包み、`nil` を含めるかどうかを選べます。
`Lazy` は生成器の構築を最初に値を取り出すときまで遅らせます。
再帰的な生成器が自分自身を参照するために必要で、上限を付けた形は9章で示します。

## 報告は値の組み上がり方を示す

`User` で失敗するプロパティの報告には、そのユーザーと、その下にラベル付きの部品が並びます。

<!-- snippet: generators_test.go#user-report -->

```text
[pbt] property failed after 3 passed checks and 189 shrink steps (size 4)
[pbt] user (user) = snippets.User{Name:"aaaaaaaaa", Age:0, Email:"aaaaaaaaa@example.com"}
[pbt]   name (StringOfN(SampledFrom(26 values), 1, 12)) = "aaaaaaaaa"
[pbt]   age (IntRange(0, 120)) = 0
[pbt] error: display name "Aaaaaaaaa" is too long for the badge
[pbt] reproduce: go test -run 'TestDisplayName' -pbt.seed=0x1
```

名前は、長さの検査にまだ引っかかる最短の文字列である `a` 9文字まで縮みました。
年齢は失敗に関係しないので0まで縮んでいます。
報告はラベル付きの値を3段の深さまで載せ、1つの値を2000文字で切ります。
`ReportDepth` と `ReportLimit` のオプションで両方を変えられます。

プロパティの最上位で取り出した値は、ラベルがなくても報告に載ります。
上の `start := pbt.Time().Draw(t, "")` のように取り出した値を1つの変数に代入していれば、報告はその変数の名前で値を示します。
そうでなければ、`Int() at prop_test.go:12 = 5` のように、取り出したファイルと行を示します。
報告を作るとき、pbtはソースファイルを読んで変数の名前を探します。
`-trimpath` を付けてビルドしたバイナリのようにファイルを読めないときは、行を示します。
`Verbose()` か `-pbt.v` を付けると、すべての取り出しに行が付きます。

## プロパティの外で値を見る

`Example` は生成器の値を1つ返します。
生成器の出力を手早く確かめるときや、従来のテストでそれらしい入力が必要なときに使います。
生成器のコードとpbtのバージョンが同じであれば、シードを渡すと値が固定されます。

<!-- snippet: generators_test.go#example -->

## 生成器が共有する値

`Just(v)` はすべてのケースに同じ `v` を返します。
`v` がスライス、マップ、ポインタなら、すべてのケースが同じオブジェクトを見ます。
プロパティがそれを変更すると、後のケースと報告が見るものも変わってしまいます。
`SampledFrom(values)` はスライスをコピーしますが、要素はそのまま渡します。
プロパティが値を変更する必要があるなら、`Custom` で毎回新しい値を作ってください。
