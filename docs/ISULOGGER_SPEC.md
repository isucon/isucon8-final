# ISULOGGER

海外の超イケてるリアルタイム分析基盤

## 制限事項

- bodyサイズは 1MB まで
- 同時に発行できるリクエストは10並列まで
- 1秒あたりのリクエスト数は20リクエストまで

## End Points

#### baseurl

baseurlは各アプリケーションごとにユニークなURLを払い出す

#### Authorization

Authorization ヘッダのBearerトークンに、予め払い出されているユニークなappidを使用して認証を行う

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
    - status: 400
        - error: invalid data
    - status: 401
        - error: app_id not found
    - status: 429
        - Too Many Requests

### `POST /send_bulk`

- request: application/json
    - array
        - tag
        - time
        - data
- response:
    - status: 200
    - status: 400
        - error: invalid data
    - status: 401
        - error: app_id not found
    - status: 413
        - error: request body too large
    - status: 429
        - Too Many Requests
