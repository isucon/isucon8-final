# webapp

## API

### 初期化

#### `POST /initialize`

- request: application/form-url-encoded
    - bank_endpoint : bankAPIのエンドポイントを指定
    - bank_appid    : bankAPIで利用するappid
    - log_endpoint  : logAPIのエンドポイント
    - log_appid     : logAPIのエンドポイントで利用するappid

### TOP

#### `GET /`

- response: text/html 

### 登録

#### `POST /signup`

- request: application/form-url-encoded
    - name : 登録名(重複可)
    - bank_id : ISUBANKのid
    - password 

- response
    - status: 200
    - status: 400
        - error: parameters failed
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
    - status: 404
        - error: bank_id or password is not match
    - status: 500
        - error: server error
- log
    - tag:signin
        - user_id: $user_id

### 注文

#### `POST /orders`

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
        - error: unautholize
    - status: 500
        - error: server error
- log
    - tag:{$type}.order
        - order_id: $order_id
        - user_id: $user_id
        - amount: $amount
        - price: $price
    - tag:buy.error (与信API失敗時)
        - user_id: $user_id
        - error: $error
        - amount: $amount
        - price: $price
- note 
    - 買い注文の場合、資金があるか処理前にISUBANKの残高チェックAPIによって確認をする必要がある

#### `DELETE /order/{id}`

- response: application/json
    - status: 200
        - id: $order.id
    - status: 401
        - error: unautholize
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
        - error: unautholize
    - status: 500
        - error: server error

### 更新情報API

#### `GET /info`

- request: 
    - cursor: 全リクエストでのcursor

- response: application/json
    - status: 200
        - cursor: 次のリクエストに渡すcursor
        - traded_orders: トレードの成立した注文
            - [$order]
        - chart_by_sec: ロウソクチャート用の単位秒取引結果
        - chart_by_min: ロウソクチャート用の単位分取引結果
        - chart_by_hour: ロウソクチャート用の単位時間取引結果
        - lowest_sell_price: $price
        - highest_buy_price: $price
        - enable_share: シェアボタン有効化フラグ
    - status: 500
        - error: server error

## 取引処理

### 仕様

#### 基本的な優先順位

1. 単価 
    - 売り注文の場合は安いほうが優先
    - 買い注文の場合は高いほうが優先
2. 注文時間

※ 例外
- 注文脚数が多く成立対象の椅子が不足している場合に限り、優先順位の繰り上がりを行うことができる

#### 価格の決定

売り注文と買い注文の価格が異なる場合、(例: 売り注文=550、買い注文=560) この場合、550-560の間で矛盾がなければ価格は幾らでも構わない。
ただし、同一取引における価格はすべて同じでなければならない。(売り注文=550x3, 買い注文1=560x2, 買い注文2=555x1 の場合、550-555 の間の価格で単価は統一しなければならない)

#### 自動キャンセル

いすこん銀行の口座に対する与信に失敗した場合、注文は自動的にキャンセルとし、優先順位が繰り上がる

### Log

下記のログを送らなければならない

- tag:trade
    - trade_id: $trade_id
    - amount:   $amount   (取引全体の脚数)
    - price:    $price    (取引での単価)
- tag:{$order.type}.trade
    - order_id: $order_id
    - trade_id: $trade_id
    - user_id:  $user_id
    - amount:   $amount
    - price:    $price
- tag:{$order.type}.delete
    - order_id: $order_id
    - reason: reserve_failed
