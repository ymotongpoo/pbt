# 規格が定める形式
<!-- summary: RFCやISOなどの規格が定める形式の文字列を生成するパッケージformat、文法全体と一般的な形を名前で区別する規則、Goのレイアウト定数、パーサーが規格から外れる箇所の見つけ方。 -->

プログラムの入力の多くは、規格が定める形式の文字列です。
タイムスタンプ、メールアドレス、URI、JSON、UUID、言語タグなどです。
パッケージ `go.ymotongpoo.com/pbt/format` はそれらを生成します。
どの生成器も `pbt.Gen[string]` を返し、規格の文法から値を組みます。
縮小器が試す値を含め、どの値も有効です。

```go
import "go.ymotongpoo.com/pbt/format"
```

1章の生成器は、`time.Time` や `netip.Addr` のようなGoの型を返します。
このパッケージの生成器はテキストを返します。
`pbt.IPv6()` はアドレスを返し、`format.RFC4291IPv6()` はアドレスのさまざまな書き方を返します。

## 2つの問いと2つの名前

形式には2つの問いがあります。
1つは、プログラムが何を書き出すべきかです。
もう1つは、パーサーが何を受理しなければならないかです。
答えは異なり、パッケージはそれぞれの答えに別の名前を付けています。

- `Email`、`URL`、`HTTPDate` のように規格の名前を含まない関数は、一般的な形か正規形を生成します。
  その形式を書き出すコードや保存するコードへの入力と、どのバリデーターも通るべき値に使ってください。
- `RFC5321Mailbox`、`RFC3986URI`、`RFC9110HTTPDate` のように規格の名前を冠した関数は、規格の生成規則にある選択肢を生成します。
  パーサーのテストに使ってください。
  省くのは、RFC 5322の廃止された構文のように、規格が生成を禁じている部分だけです。
  どの部分を省いたかと、上限のない長さに設けた上限は、各関数のドキュメントに書いてあります。

`Time(layout)` はパッケージ `time` のレイアウト定数を受け取ります。
`format.Time(time.RFC1123)` は、そのレイアウトで `time.Time.Format` が書き出す文字列を生成します。
つまり `Time` はGoが書き出す形の名前で、`RFC3339` はRFC 3339が許す形の名前です。

| 分野 | プログラムが書き出すもの | パーサーが受理しなければならないもの |
| --- | --- | --- |
| タイムスタンプ | `Time(time.RFC3339)`、`HTTPDate` | `RFC3339`、`RFC3339FullDate`、`RFC3339FullTime`、`RFC9110HTTPDate`、`RFC5322DateTime` |
| 期間 | `ISO8601Duration` | `RFC3339Duration` |
| アドレス | `IPv4`、`RFC5952IPv6`、`CIDR`、`MAC` | `RFC4291IPv6` |
| ホスト名 | `Hostname`、`IDNAHostname`、`UnicodeHostname` | `RFC1035Domain` |
| メール | `Email`、`HTMLEmail`、`SMTPUTF8Mailbox` | `RFC5321Mailbox`、`RFC5322AddrSpec` |
| URI | `URL` | `RFC3986URI`、`RFC3986Reference` |
| HTTP | `HTTPToken`、`HTTPFieldName`、`HTTPFieldValue`、`MediaType` | `RFC9110FieldValue`、`RFC9110MediaType`、`RFC6265Cookie`、`RFC6265SetCookie` |
| データ | `JSON`、`IJSON`、`JSONPointer`、`JSONPointerFragment`、`UUID`、`SemVer`、`LanguageTag` | `RFC4180CSV`、`RFC5646LanguageTag` |
| エンコーディング | `Base64`、`Base64URL`、`Base32`、`Base32Hex`、`Base16`、`Hex` と、パディングなしの `Raw` 版 | |
| 番号 | `PhoneNumber`、`PaymentCard`、`ISBN13`、`ISBN10` | |

## パーサーをテストする

規格のあらゆる形をパーサーに与えるプロパティは、パーサーが規格から外れる箇所を見つけます。
RFC 3339のタイムスタンプは、どれも同じ名前のレイアウトで解析できるはずです。

<!-- snippet: formats_test.go#parser -->

```text
[pbt] property failed after 1 passed checks and 19 shrink steps (size 2)
[pbt] s (RFC3339()) = "1970-01-01T00:00:00z"
[pbt] error: time.Parse rejects 1970-01-01T00:00:00z: parsing time "1970-01-01T00:00:00z" as "2006-01-02T15:04:05Z07:00": cannot parse "z" as "Z07:00"
[pbt] reproduce: go test -run 'TestRFC3339' -pbt.seed=0x1
```

RFC 3339は小文字の `z` と `t` を許しますが、`time.Parse` はそれらを受理しません。
閏秒の `23:59:60` も同じく受理しません。
それが問題になるかどうかは、タイムスタンプの出どころによります。
このプロパティで判断すべき点が明らかになり、縮小したケースはその違いを1文字で示します。
各生成器のドキュメントには、Go 1.26で確かめた、規格とGoの標準ライブラリのパーサーとの違いが載っています。
パッケージのテストが、それらの違いを検査しています。

## プログラムが書き出すものをテストする

プログラムへの入力には、規格の名前を含まない関数を使います。
その値は、一般的な受け手ならどれでも受理します。
登録フォームは `format.Email()` の値を保存し、`net/mail` が同じアドレスを読み戻すことを期待できます。

<!-- snippet: formats_test.go#producer -->

これらの生成器も1章の生成器と同じく、コードが誤動作しやすいケースに寄ります。
一部のケースでは、次のような値を作ります。

- `Email` はちょうど64文字のローカル部を、`Hostname` はちょうど253文字の名前を作ります。
- `RFC3339` は閏秒とオフセット `-00:00` を書き出します。
- `JSON` は64ビットをはるかに超える数と、対になっていないサロゲートのエスケープを書き出します。

その頻度は各生成器のドキュメントにあります。

<!-- snippet: formats_test.go#roundtrip -->

## 値の縮み方

値は短い有効な文字列へ縮みます。
`Email` なら `a@a.aa`、`URL` なら `http://a`、`RFC3339` なら `1970-01-01T00:00:00Z`、`JSON` なら `null` です。
縮小器は選択の少ないほうを選ぶので、値の一部が、最初に選ばれていた選択肢の中で縮むこともあります。
例えば、引用符で囲んだローカル部は、空の引用文字列 `""@a` へ縮みます。
各生成器のドキュメントには、選択肢ごとの縮小先が載っています。

## パッケージが扱わないもの

このパッケージが生成するのは構文で、登録された値ではありません。

- 言語タグは整形式ですが、サブタグはIANAのレジストリと照合していません。
- カード番号はチェックディジットが正しいものの、どの発行者が割り当てた番号でもありません。
- ISBNにはハイフンを入れません。
  ハイフンの位置は、ISBN機関が公開する範囲で決まるからです。

ISO 3166の国コードやISO 4217の通貨のように、レジストリに従う表はレジストリとともに変わるので、このパッケージには含めていません。
プログラムが対応する値を並べ、`SampledFrom` で書いてください。
