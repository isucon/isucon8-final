## webapp

### 初期化

#### `POST /initialize`

- request: application/form-url-encoded
    - bank_endpoint : bankAPIのエンドポイントを指定
    - bank_appid    : bankAPIで利用するappid
    - log_endpoint  : logAPIのエンドポイント

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
        - time: $current_time

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
        - time: $current_time

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

#### `POST /sell_requests`

- request: application/form-url-encoded
    - amount: 売りたい脚数
    - price:  指値
- response: application/json
    - {"ok":true} = 成功
    - {"ok":false,"error":"メッセージ"} = 失敗
- log
    - tag:sell.request
        - user_id: $user_id
        - sell_id: $sell_id
        - amount: $amount
        - price: $price
        - time: $current_time
- memo
    - 処理後に買い注文とのマッチングをする


### 買い注文

#### `POST /buy_requests`

- request: application/form-url-encoded
    - amount: 買いたい脚数
    - price:  指値
- response: application/json
    - {"ok":true} = 成功
    - {"ok":false,"error":"メッセージ"} = 失敗
- log
    - tag:buy.request
        - user_id: $user_id
        - buy_id: $buy_id
        - amount: $amount
        - price: $price
        - time: $current_time
    - tag:credit.error (与信API失敗時)
        - user_id: $user_id
        - error_code: $error_code
        - amount: $amount
        - price: $price
        - time: $current_time
- memo
    - 処理前に与信APIを叩く(これを叩かないとエラー)
    - 処理後に売り注文とのマッチングをする

### 売買成立

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
        - time: $current_time
    - tag:sell.close
        - trade_id: $trade_id
        - user_id: $user_id
        - sell_id: $sell_id
        - amount: $amount
        - price: $price
        - time: $current_time
    - tag:buy.close
        - trade_id: $trade_id
        - user_id: $user_id
        - buy_id: $buy_id
        - amount: $amount
        - price: $price
        - time: $current_time


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
    - reserve_id
- response: application/json
    - status: ok
    - error: ...

### `POST /cancel`

決済取り消しAPI

reserve APIで予約した決済を取り消します

- request: application/json
    - app_id
    - reserve_id
- response: application/json
    - status: ok
    - error: ...

## ロガー

### `POST /send`

- request: application/json
    - app_id
    - tag
    - praams

### `POST /send_bulk`

- request: application/json

    - app_id
    - logs
        - tag
        - params
