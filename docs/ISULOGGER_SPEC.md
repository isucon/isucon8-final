# ISUロガー

海外の超イケてる分析ができるという新興データ分析基盤

## 制限事項

- bodyサイズは 1MB まで
- データの都合上遅延は10秒まで

## End Points

#### baseurl

baseurlは各アプリケーションごとにユニークなURLを払い出す

#### Authorization

アプリケーションへの認証は Authorization にユニークなappidをしてする
```
Authorization: Bearer <APP_ID>
```

### `POST /send`

- request: application/json
    - tag
    - time
    - data

- response:
    - status: 200
    - status: 401
        - error: app_id not found
    - status: 400
        - error: invalid data
    - status: 503
        - rate limit exceeded

### `POST /send_bulk`

- request: application/json
    - array
        - tag
        - time
        - data
- response:
    - status: 200
    - status: 401
        - error: app_id not found
    - status: 400
        - error: invalid data
    - status: 413
        - error: request body too large
    - status: 503
        - rate limit exceeded
