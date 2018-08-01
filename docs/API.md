## webapp

### 初期化

#### `POST /initialize`

- request: application/form-url-encoded
    - bank_endpoint : bankAPIのエンドポイントを指定
    - bank_appid    : bankAPIで利用するappid
    - log_endpoint  : logAPIのエンドポイント
    - log_appid     : logAPIのエンドポイントで利用するappid

### 登録

#### `GET /signup`

- response: text/html 
    - 登録フォーム

#### `POST /signup`

- request: application/form-url-encoded
    - name
    - bank_id : ISUBANKのid
    - password 

- response
    - redirect `/signin`
- log
    - tag:signup
        - name: $name
        - bank_id: $bank_id
        - user_id: $user_id

### ログイン

#### `GET /signin`

- response: text/html 
    - ログインフォーム

#### `POST /signin`

- request: application/form-url-encoded
    - bank_id : ISUBANKのid
    - password 

- response
    - redirect `/mypage`
    - with Session
- log
    - tag:signin
        - user_id: $user_id

### マイページ

#### `GET /mypage`

- response: text/html 
    - 売り注文リスト
    - 買い注文リスト
    - 自分の注文
    - 成立した売買
    - 売り注文フォーム
    - 買い注文フォーム

- Memo
    - リストのページャー
    - リアルタイム性(TODO)
    - 売買成立の通知


### 売り注文

#### `POST /sell_orders`

- request: application/form-url-encoded
    - amount: 売りたい脚数
    - price:  指値(1脚あたりの最低額)
- response: application/json
    - {"ok":true} = 成功
    - {"ok":false,"error":"メッセージ"} = 失敗
- log
    - tag:sell.order
        - user_id: $user_id
        - sell_id: $sell_id
        - amount: $amount
        - price: $price
- memo
    - 処理後に買い注文とのマッチングをする

#### `GET /sell_orders`

- response: application/json
    - list
        - id         : $order_id
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

### 買い注文

#### `POST /buy_orders`

- request: application/form-url-encoded
    - amount: 買いたい脚数
    - price:  指値(1脚あたりの最高額)
- response: application/json
    - {"ok":true} = 成功
    - {"ok":false,"error":"メッセージ"} = 失敗
- log
    - tag:buy.order
        - user_id: $user_id
        - buy_id: $buy_id
        - amount: $amount
        - price: $price
    - tag:buy.error (与信API失敗時)
        - user_id: $user_id
        - error: $error
        - amount: $amount
        - price: $price
- memo
    - 処理前に与信APIを叩く(これを叩かないとエラー)
    - 処理後に売り注文とのマッチングをする

#### `GET /buy_orders`

- response: application/json
    - list
        - id         : $order_id
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

### 売買成立

#### `GET /trades`

- response: application/json
    - list
        - id         : $trade_id
        - amount     : $amount (取引脚数)
        - price      : $price (取引価格)
        - created_at : $created_at (成立時間)

#### `runTrade`

売り注文/買い注文の確定後に実行されるサブルーチン

- memo
    - 買い注文の金額内で売り注文をマッチング
    - 買い注文に対して 引き落としAPIを叩く
    - 売り注文に対して 振込APIを叩く
- log
    - tag:close
        - trade_id: $trade_id
        - amount: $amount
        - price: $price
    - tag:sell.close
        - trade_id: $trade_id
        - user_id: $user_id
        - sell_id: $sell_id
        - amount: $amount
        - price: $price
    - tag:buy.close
        - trade_id: $trade_id
        - user_id: $user_id
        - buy_id: $buy_id
        - amount: $amount
        - price: $price


## 銀行API

ユーザーには公開しないAPI

### `POST /register`

登録 (本来は非公開API)

- request: application/json
    - bank_id 
- response: application/json
    - status: ok

### `POST /add_credit`

creditの追加 (本来は非公開API)

- request: application/json
    - bank_id 
    - price
- response: application/json
    - status: ok

### `POST /check`

与信API

指定したpriceを支払い可能の場合 status:ok

- request: application/json
    - app_id
    - bank_id
    - price
- response: application/json
    - status: ok
    - error: ...

### `POST /reserve`

決済予約API

`price>0` : 振込
`price<0` : 引き落とし

reserveの有効期限は1分間
1分以内のcommitは保証されます

- request: application/json
    - app_id
    - bank_id
    - price
- response: application/json
    - status: ok
    - reserve_id: bigint
    - error: ...

### `POST /commit`

決済確定API

reserve APIで予約した決済を確定します

- request: application/json
    - app_id
    - reserve_ids
- response: application/json
    - status: ok
    - error: ...

### `POST /cancel`

決済取り消しAPI

reserve APIで予約した決済を取り消します

- request: application/json
    - app_id
    - reserve_ids
- response: application/json
    - status: ok
    - error: ...

## ロガー

### `POST /send`

- request: application/json
    - app_id
    - tag
    - time
    - data 

### `POST /send_bulk`

- request: application/json

    - app_id
    - logs
        - tag
        - time
        - data
