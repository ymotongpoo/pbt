# サイズ、分布、統計
<!-- summary: サイズパラメーター、Frequencyによる重み、SizedとResize、統計のCollect、Classify、Aggregate、Measure、Cover、被覆を検査にするCheckCoverage、そして無駄な入力や大きすぎる入力を警告するヘルスチェック。 -->

pbtでは、データがどんな姿になるかを利用者が決め、意図したとおりになっているかを確かめる手段も利用者が持ちます。
この2つは対になっています。
システムの興味深い状態に到達しないプロパティは、その状態について何も証明しません。

## サイズパラメーター

すべてのテストケースはサイズを持って実行されます。
`Check` はサイズ1から始め、ケースが通過するごとに1ずつ、42まで増やします。
実行は小さな入力で始まり、大きな入力で終わります。
文字列、スライス、マップの長さはサイズを平均とし、整数の大きさもサイズとともに増えます。
`t.Size()` は現在のケースのサイズを返します。
`Sizes(start, max)` オプション、またはフラグ `-pbt.startsize` と `-pbt.maxsize` で範囲を変えられます。

`Sized` はサイズから生成器を組みます。
PropErの `?SIZED` と同じ働きです。
`Resize` は生成器を固定したサイズで動かします。
2つを組み合わせると、再帰的な構造に上限を付けられます。

<!-- snippet: distribution_test.go#sized -->

段ごとにサイズを半分にすると、分岐の数がいくつでも、木の深さはサイズの対数以下に収まります。
`SliceOf` のようにサイズを使うコレクション生成器では、正の最小長が指定されていなければ、`Resize(0, g)` は空のコレクションを生成します。
`Resize` が固定するのはサイズパラメーターだけなので、`SliceOfN` で指定した長さの制約は保たれます。
`Resize(0, SliceOfN(Int(), 1, 3))` は要素を1つ生成します。

## 重み

`Frequency` は、重みに比例した確率で選択肢の1つから値を取り出します。

<!-- snippet: distribution_test.go#frequency -->

重みが影響するのは生成だけです。
縮小は重みに関係なく常に先頭の選択肢へ向かうので、選択肢の並び順が縮小の優先順位になります。
重み0の選択肢は生成されず、縮小によってだけ到達できます。
3章の `ShrinkTo` はこの性質の上に組んであります。
`OneOf` は重みが等しい `Frequency` です。

## 統計

`*pbt.T` の4つのメソッドが、実行したケースの様子を記述します。
プロパティが通過すると `Check` が結果を表示します。

<!-- snippet: distribution_test.go#statistics -->

- `Classify(cond, label)` は条件が成り立ったときにラベルを記録し、`Collect(v)` は値の文字列表現をラベルとして記録します。
  1ケースで各ラベルを1回ずつ記録する限り、どちらも通過したケース全体に対する割合を報告します。
  重複は除かないので、同じケースで同じラベルを2回記録すると2回分を数え、割合が100%を超えることがあります。
  1ケースで何度も起きる事象には `Aggregate` を使ってください。
  `Cover` は同じケースの同じラベルを1回と数えます。
- `Aggregate(values...)` は1ケースに複数あるサンプルを数え、サンプル全体に対する割合として報告します。ステートフルテストが実行したコマンドの名前に向いています。
- `Measure(name, value)` は数値を記録し、最小、平均、最大を報告します。

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  22.0% read-only batch
[pbt]  81.1% get
[pbt]  18.9% put
[pbt] batch length: min 1, avg 12.43, max 20 (100 samples)
```

割合は4対1の重みどおりで、長さの行は上限いっぱいのバッチも出ていることを示しています。
`Verbose()` か `-pbt.v` を付けると、プロパティが何も記録していなくても要約が出ます。

## カバレッジを検査する

`Cover(cond, pct, label)` は観察を要求に変えます。
`cond` が成り立つケースが、通過した全ケースの `pct` パーセント以上を占めなければならない、という要求です。
上の例では、バッチの60%に書き込みが含まれることを要求しています。

100ケースの実行が偶然60%を下回ることはあるので、100%未満の要求は統計的に判定します。
pbtは観測した割合の99.7%信頼区間であるWilsonスコア区間を計算します。
区間の下限が `pct` 以上なら充足、上限が `pct` 未満なら不足、それ以外は未決です。
100%の要求は統計で判定せず、通過したすべてのケースでそのラベルに該当すれば充足、1ケースでも該当しなければ不足です。
`CheckCoverage` オプションがなければ、`Check` は不足をログに出して通過します。
オプションがあると、`Check` はすべての要求が決まるまで、設定した回数の20倍を上限にケースを走らせ続けます。
不足なら失敗し、要求が未決のまま終わったときも失敗します。
通過したなら、データがその割合を裏付けています。

<!-- snippet: distribution_test.go#cover-fails -->

```text
[pbt] insufficient coverage:
[pbt]   round hundred: 17.6% of 125 cases, required 30.0%
```

区間が30%を除外した時点、つまり125ケースで実行は止まりました。
観測された割合は `IntRange` についても教えてくれます。
0は境界値なので、1000分の1よりずっと頻繫に出ているのです。
統計はそれを知る手段で、一様な分布がプロパティに必要なときに分布を変える手段が `Frequency` や `SampledFrom` です。

## ヘルスチェック

統計は、利用者が立てた問いに答えます。
ヘルスチェックは、利用者が思い付かないかもしれない問い、つまり実行がケースを無駄なく使っているかに答えます。
`Check` はすべての実行を監視し、無駄になった入力、手間のかかる生成、保存した入力の変化を警告します。
オプションの検査を有効にすると、選択の重複も検出します。

| 検査 | 警告する条件 |
| --- | --- |
| `HealthFilter` | ケースの半数以上（かつ20件以上）が、`Assume`、`Skip`、または引き直しを諦めた `Filter` で破棄された。または、`Filter` が100個以上の値のうち9割を却下した |
| `HealthLarge` | ケースの5分の1が、それぞれ10,000個を超える選択を取り出した |
| `HealthSlow` | プロパティ自身の処理を除いた値の取り出しに、1ケースあたり10ミリ秒、合計で1秒かかった |
| `HealthStale` | 保存したケースの取り出しのコメントが、再実行で得た取り出しと異なる。デフォルトで警告する |
| `HealthDuplicate` | 破棄されなかった20件以上のランダムなケースのうち、選択列が異なる入力が半分に満たない。デフォルトでは無効 |

長い文字列だけを残すプロパティでは、表の最初の検査 `HealthFilter` が警告を出します。

<!-- snippet: health_test.go#health -->

```text
[pbt] health: HealthFilter: 629 of 729 cases were discarded; most by Assume at health_test.go:18 (629) (CheckHealth(HealthFilter) makes this fail; IgnoreHealth(HealthFilter) hides it)
```

実行は通過しましたが、100ケースを得るために729ケースを走らせており、警告は残りを捨てた行を示しています。
長い文字列を組む生成器 `StringN(8, 40)` を使えば、この無駄はなくなります。
警告は、破棄や却下の多かった箇所を示します。
`Filter` ならその説明、`Assume` や `Skip` なら呼び出した行です。
`HealthLarge` は、最も多くの選択を使った取り出しを示します。

`HealthDuplicate` 以外のヘルスチェックは、デフォルトでは `Cover` と同じく警告だけを出し、実行は通過します。
`CheckHealth()` を付けると、`HealthFilter` か `HealthLarge` の警告が出た実行は、プロパティが通過していても失敗になります。
`CheckHealth(HealthSlow)` は時間の検査を加えます。
負荷の高いCIマシンでは引っかかることがあるので、この検査はデフォルトでは含めていません。
`CheckHealth(HealthStale)` を付けると、保存したケースの陳腐化が通過した実行を失敗にします。
生成器は意図して変えることもあるので、`CheckHealth()` はこの検査を警告のまま残します。
`IgnoreHealth(checks...)` は検査を隠し、`CheckHealth` と `WarnHealth` より優先されます。

フラグ `-pbt.checkhealth` と `-pbt.ignorehealth` は同じ検査を名前で受け取り、`go test` の実行中のすべての `Check` に効きます。
例えば `-pbt.checkhealth=default,slow` です。
失敗した実行には、さらに `HealthFlaky` と `HealthShrink` の2つの検査が働きます。
この2つは3章で説明します。

`HealthStale` の警告は、保存した同じ選択が、今は別の取り出しの文字列になることを意味します。
直したはずのバグがまだ失敗するという意味ではありません。
警告は、ファイルと、最初に変わった取り出しを示します。
再実行の結果を信じる前に何を確かめればよいかは、[4章](04-properties.md#シード保存したケース再実行)で説明します。

`WarnHealth(HealthDuplicate)` か `-pbt.warnhealth=duplicate` を付けると、重複の警告が有効になります。
`CheckHealth(HealthDuplicate)` を付けると、エラーとして有効になります。
引数のない `WarnHealth()` は何も変えません。
ヘルスチェックのフラグとオプションは足し合わされ、`CheckHealth` がエラーにした検査を `WarnHealth` で警告に戻すことはできません。

<!-- snippet: diagnostics_test.go#duplicate -->

真偽値の定義域には値が2つしかないので、この実行は意図して同じ値を繰り返します。

```text
[pbt] health: HealthDuplicate: only 2 of 20 generated inputs were distinct by their choices; possible causes include a small domain such as Bool or SampledFrom, a low maximum size, or more Checks than the input space has values (CheckHealth(HealthDuplicate) makes this fail; IgnoreHealth(HealthDuplicate) hides it)
```

`HealthDuplicate` は、通常のランダムな生成で得た、通過したケースと失敗したケースを数えます。
破棄されたケース、保存したケースの再実行、縮小、標的型探索は数えません。
カバレッジ誘導探索の変異も数えません。
キーには使った選択のハッシュを使い、サイズは含めません。
サイズは実行の間に増えるので、含めると序盤のほとんどのケースで選択の重複が見えなくなります。

この数は、異なる値の数の近似です。
異なる選択が同じ値になることもあり、サイズだけから導いた値は重複として数えられます。
異なるキーが4096を超えると数えるのをやめます。
小さな定義域は意図したものであることも多いので、この検査は明示的なオプションを必要とします。
