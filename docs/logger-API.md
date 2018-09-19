# ロガー

海外の超イケてる分析ができるという進行データ分析基盤

## 噂

- 西海岸に存在するためRTT 100msかかるらしい
- 管理画面は使いやすいらしいが、APIは貧弱で並列度が低いらしい

## 制限事項

- bodyサイズは 1MB まで
- データの都合上遅延は10秒まで

## End Points

#### baseurl

baseurlは各アプリケーションごとにユニークなURLを払い出す

#### Authorization

アプリケーションへの認証は Authorization にユニークなappidをしてする
```
Authorization: app_id APP_ID
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

### `GET /logs` (secret API)

- request: query
    - app_id
    - user_id
    - trade_id

- response:
    - status: 200
        - array
            - tag
            - time
            - data
    - status: 401
        - error: app_id not found
    - status: 400
        - error: invalid data
