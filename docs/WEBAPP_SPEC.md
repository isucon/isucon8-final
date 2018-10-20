# ISUCOIN 仕様書

ISUCOINは仮想椅子の取引所


## 決済について

[いすこん銀行の決済API](ISUBANK_SPEC.md)を利用

- ユーザーの存在確認  
  登録時に残高APIをcredit=0として確認する

- 残高の確認  
  買い注文時に残高APIによって資金を保有していることを確認し無駄な注文を防ぐ

- 決済  
  取引成立時にいすこん銀行APIの仕様に従って、買い注文から出金し売り注文へ入金を行う
 

## データ分析について

[リアルタイム分析基盤ISULOGGER](ISULOGGER_SPEC.md)を利用

各アクションでのログ設計は後述のAPI仕様及び決済仕様に記載している。  
仕様に従ってtagとdataを設定したログを **10秒以内** に送信すること


## API詳細仕様

### ベンチマーカー初期化

※ このAPIは本来アプリケーションとして提供するものではない。  
  いすばたSNSへのシェア機能リリースに向け、ベンチマークを初期化するためだけに利用する

#### `POST /initialize`

- request: application/form-url-encoded
    - bank_endpoint : bankAPIのエンドポイントを指定
    - bank_appid    : bankAPIで利用するappid
    - log_endpoint  : logAPIのエンドポイント
    - log_appid     : logAPIのエンドポイントで利用するappid

### TOP

SPA(シングルページアプリケーション)のHTMLを返す

#### `GET /`

- response: text/html 

### 登録

いすこん銀行へユーザーの存在確認を実施し、存在していれば登録する

※ パスワードはセキュリティ要件により bcrypt でハッシュ化したものをデータベースに保存しなければならない

#### `POST /signup`

- request: application/form-url-encoded
    - name : 登録名(重複可)
    - bank_id : ISUBANKのid
    - password 

- response
    - status: 200
    - status: 400
        - error: parameters failed
    - status: 403
        - error: too many failures # for brute force. 同じbank_idに対して5回連続失敗したときに返して良い
    - status: 404
        - error: bank user not found
    - status: 409
        - error: bank_id conflict
    - status: 500
        - error: server error
- log
    - tag:signup
        - name: $name
        - bank_id: $bank_id
        - user_id: $user_id

### ログイン

#### `POST /signin`

- request: application/form-url-encoded
    - bank_id : ISUBANKのid
    - password 

- response
    - status: 200
        - id:   $user.id
        - name: $user.name
    - status: 400
        - error: invalid parameters
    - status: 403
        - error: too many failures # for brute force. 同じbank_idに対して5回連続失敗したときに返して良い
    - status: 404
        - error: bank_id or password is not match
    - status: 500
        - error: server error
- log
    - tag:signin
        - user_id: $user_id

### 注文


#### `POST /orders`

買い注文、または売り注文を行う。  
※ 買い注文の場合は、上述のように残高の確認を行う必要がある

- request: application/form-url-encoded
    - type:
        - sell: 売り注文
        - buy:  買い注文
    - amount: 注文脚数 (Uint)
    - price:  指値(1脚あたりの最低額) (Uint)

- response: application/json
    - status: 200
        - id: $order_id
    - status: 400
        - error: invalid params
        - error: 残高不足
    - status: 401
        - error: unauthorized
    - status: 500
        - error: server error
- log
    - tag:{$type}.order
        - order_id: $order_id
        - user_id: $user_id
        - amount: $amount
        - price: $price
    - tag:buy.error # 残高確認API失敗時
        - user_id: $user_id
        - error: $error
        - amount: $amount
        - price: $price

#### `DELETE /order/{id}`

注文をキャンセルする

- response: application/json
    - status: 200
        - id: $order.id
    - status: 401
        - error: unauthorized
    - status: 404
        - error: not found
        - error: already closed
    - status: 500
        - error: server error
- log
    - tag:{$type}.delete
        - order_id: $order_id
        - reason:   canceled

#### `GET /orders`

取引成立した注文と有効な注文を返却する。  
※ キャンセルした注文は含まない。  

`POST|DELETE` による注文(または取り消し)は即座に反映されていなければならない    
ただし、注文と同時に決済予約の失敗によって自動的にキャンセルとなった場合は、注文直後であってもPOSTで返却された注文が含まれない場合はある。  
(※ 残高確認APIに予約分が含まれていないため、注文は通るが決済はできないことがあるため)

- response: application/json
    - status: 200
        - list
            - id         : $order_id
            - type       : $type
            - user_id    : $user_id
            - amount     : $amount
            - price      : $price (注文価格)
            - closed_at  : $closed_at (注文成立または取り消しの時間、その他はnull)
            - trade_id   : $trade_id  (注文成立時に注文番号、未成立の場合はキーなし)
            - created_at : $created_at (注文時間)
            - user: 
                - id   : $user_id
                - name : $user.name
            - trade: 
                - id         : $trade_id
                - amount     : $amount (取引脚数)
                - price      : $price (取引価格)
                - created_at : $created_at (成立時間)
    - status: 401
        - error: unauthorized
    - status: 500
        - error: server error

### 更新情報API

ゲストユーザー/ログイン済みユーザー共に1秒おきにリクエストを行う。  
infoに返すデータは原則1秒以内に反映する必要がある。  
ただし、traded_ordersには1秒以内であっても、リクエストとレスポンスのcursor間の取引は含まれなければならない。

#### `GET /info`

- request: 
    - cursor: 全リクエストでのcursor

- response: application/json
    - status: 200
        - cursor: 次のリクエストに渡すcursor
        - traded_orders: トレードの成立した注文(ログインユーザーのみ)
            - [$order]
        - chart_by_sec: ロウソクチャート用の単位秒取引結果
        - chart_by_min: ロウソクチャート用の単位分取引結果
        - chart_by_hour: ロウソクチャート用の単位時間取引結果
        - lowest_sell_price: $price
        - highest_buy_price: $price
        - enable_share: シェアボタン有効化フラグ
    - status: 500
        - error: server error

## 取引処理仕様

後述の優先順位と価格の決定のに従って、可能な限り早く取引を成立させること  

### 基本的な優先順位

1. 単価 
    - 売り注文の場合は安いほうが優先
    - 買い注文の場合は高いほうが優先
2. 注文時間
    - 数ミリ秒でも早く注文した注文が優先

*※ 例外*
- 注文脚数が多く成立対象の椅子が不足している場合に限り、優先順位の繰り上がりを行うことができる

※ 上記の優先順位が守られている限り処理の都合上で取引成立時間が前後することは許容される

### 価格の決定

売り注文と買い注文の価格が異なる場合、(例: 売り注文=550、買い注文=560) この場合、550-560の間で矛盾がない価格であればいくらでもよい。

ただし、同一取引における価格はすべて同じでなければならない。  
(例: 売り注文=550x3, 買い注文1=560x2, 買い注文2=555x1 の場合、550-555 の間の価格で単価は統一しなければならない)

### 自動キャンセル

いすこん銀行の決済予約に失敗した場合、注文は自動的にキャンセルとする。

### ログ

取引処理時に下記のログを送らなければならない

- tag:trade  # 取引に成功したとき、取引の単価と脚数
    - trade_id: $trade_id
    - amount:   $amount   (取引全体の脚数)
    - price:    $price    (取引での単価)

- tag:{$order.type}.trade  # 取引に成功したときに注文ごとの成立単価と脚数
    - order_id: $order_id
    - trade_id: $trade_id
    - user_id:  $user_id
    - amount:   $amount
    - price:    $price

- tag:{$order.type}.delete # 自動キャンセルをしたとき
    - order_id: $order_id
    - reason: reserve_failed
