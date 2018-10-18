# いすこん銀行 API

超すごい最新鋭のネット銀行 いすこん銀行 の口座から入出金ができるAPI

## End Points

#### baseurl

baseurlは各アプリケーションごとにユニークなURLを払い出す

#### Authorization

Authorization ヘッダのBearerトークンに、予め払い出されているユニークなappidを使用して認証を行う

```
Authorization: Bearer <APP_ID>
```

### `POST /check`

指定した金額の残高を指定したユーザーが保持しているかを確認します  
※ ただし予約分を含みません

また、このAPIのpriceに0を指定することでユーザーの存在チェックに利用することもできます

- request: application/json
    - bank_id
    - price
- response: application/json
    - status: 200
    - status: 400
        - error: paramater invalid
        - error: credit is insufficien
    - status: 401
        - error: app_id not found
    - status: 404
        - error: bank_id not found

### `POST /reserve`

口座から資金を確保し決済予約を行います

予約した決済の有効期限は5分間で期限内で未使用の予約は必ず確定( `POST /commit` )できます

- request: application/json
    - bank_id
    - price
        - `>0` の場合は振込
        - `<0` の場合は引き落とし
- response: application/json
    - status: 200
        - reserve_id: bigint
    - status: 400
        - error: paramater invalid
        - error: credit is insufficien
    - status: 401
        - error: app_id not found
    - status: 404
        - error: bank_id not found

### `POST /commit`

決済予約をしたreserve_idを確定します

※ 上述の通り、期限内のreserve_idの成功は保証されているため、50xなどのエラーが発生したときは適宜リトライすることを推奨します

- request: application/json
    - reserve_ids
- response: application/json
    - status: 200
    - status: 400
        - error: paramater invalid
        - error: reserve is already expired
        - error: reserve is already committed
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
    - status: 400
        - error: paramater invalid
        - error: reserve is already expired
        - error: reserve is already committed
    - status: 401
        - error: app_id not found
    - status: 404
        - error: reserve_id not found
