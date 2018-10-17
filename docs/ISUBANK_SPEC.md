# いすこん銀行 API

超すごい最新鋭のネット銀行 いすこん銀行 のAPI

## End Points

#### baseurl

baseurlは各アプリケーションごとにユニークなURLを払い出す

#### Authorization

アプリケーションへの認証は Authorization にユニークなappidを指定する
```
Authorization: Bearer <APP_ID>
```

### `POST /check`

銀行残高を確認します。
※ ただし予約分を含みません

- request: application/json
    - bank_id
    - price
- response: application/json
    - status: 200
    - status: 401
        - error: app_id not found
    - status: 404
        - error: bank_id not found
    - status: 40x
        - error: credit is insufficient

### `POST /reserve`

口座から資金を確保し決済予約を行います

reserveの有効期限は3分間

3分以内のcommitは保証されます

- request: application/json
    - bank_id
    - price
        - `>0` の場合は振込
        - `<0` の場合は引き落とし
- response: application/json
    - status: 200
        - reserve_id: bigint
    - status: 401
        - error: app_id not found
    - status: 404
        - error: bank_id not found
    - status: 40x
        - error: credit is insufficien

### `POST /commit`

reserve APIで予約した決済を確定します

- request: application/json
    - reserve_ids
- response: application/json
    - status: 200
    - status: 401
        - error: app_id not found
    - status: 404
        - error: reserve_id not found

### `POST /cancel`

reserve APIで予約した決済を取り消します

- request: application/json
    - reserve_ids
- response: application/json
    - status: 200
    - status: 401
        - error: app_id not found
    - status: 404
        - error: reserve_id not found
