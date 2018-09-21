# webapp

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
    - $orders: 自分の注文リスト (ログインしている場合)
    - $trades: 全体の成立した売買
    - 売り注文フォーム (未ログインの場合は非活性)
    - 買い注文フォーム (未ログインの場合は非活性)

- Memo
    - SPA

### 登録

#### `POST /signup`

- request: application/form-url-encoded
    - name
    - bank_id : ISUBANKのid
    - password 

- response
    - status: 200
    - status: 40x
        - error: bank_id conflict
    - status: 400
        - error: bank user not found
- log
    - tag:signup
        - name: $name
        - bank_id: $bank_id
        - user_id: $user_id

### ログイン

#### `POST /signin`

- request: application/form-url-encoded
    - bank_id : ISUBANKのid
        - id:   $user.id
        - name: $user.name
    - password 

- response
    - status: 200
    - status: 404
        - error: bank_id or password is not match
- log
    - tag:signin
        - user_id: $user_id

### 売り注文

#### `POST /orders`

- request: application/form-url-encoded
    - type:   sell=売り注文, buy=買い注文
    - amount: 注文脚数 (Uint)
    - price:  指値(1脚あたりの最低額) (Uint)

- response: application/json
    - status: 200
        - id: $order_id
    - status: 400
        - error: invalid params
    - status: 40x
        - error: 残高不足
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
- memo
    - 買い注文の場合、資金があるか処理前に与信APIを叩く(これを叩かないとfail)
    - 処理後にマッチングをする
    - 直後にGET /ordersを叩く

#### `DELETE /order/{id}`

- response: application/json
    - status: 200
        - id: $order.id
    - status: 401
        - error: unautholize
    - status: 404
        - error: no order
    - status: 40x
        - error: alreay trade
- log
    - tag:{$type}.delete
        - order_id: $order_id
        - reason:   canceled

- memo
    - 買い注文の場合、資金があるか処理前に与信APIを叩く(これを叩かないとfail)
    - 処理後にマッチングをする

#### `GET /orders`

- response: application/json
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

### 更新情報API

#### `GET /info`

- request: 
    - cursor: 全リクエストでのcursor

- response: application/json
    - cursor: 次のリクエストに渡すcursor
    - traded_orders: トレードの成立した注文
        - [$order]
    - chart_by_sec: ロウソクチャート用の単位秒取引結果
    - chart_by_min: ロウソクチャート用の単位分取引結果
    - chart_by_hour: ロウソクチャート用の単位時間取引結果
    - lowest_sell_price: $price
    - highest_buy_price: $price

### 更新処理

#### `runTrade`

売り注文/買い注文の確定後に実行されるサブルーチン

- memo
    - 買い注文の金額内で売り注文をマッチング
    - 買い注文に対して 引き落としAPIを叩く
    - 売り注文に対して 振込APIを叩く
- log
    - tag:trade
        - trade_id: $trade_id
        - amount: $amount
        - price: $price
    - tag:{$order.type}.trade
        - order_id: $order_id
        - trade_id: $trade_id
        - user_id: $user_id
        - amount: $amount
        - price: $price
    - tag:{$order.type}.delete
        - order_id: $order_id
        - reason: reserve_failed
