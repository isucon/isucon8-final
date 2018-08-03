# ロガー

海外の超イケてる分析ができるという進行データ分析基盤

## 噂

- 西海岸に存在するためRTT 100msかかるらしい
- 管理画面は使いやすいらしいが、APIは貧弱で並列度が低いらしい

## 制限事項

- bodyサイズは 1MB まで
- データの都合上遅延は10秒まで

## End Points

### `POST /send`

- request: application/json
    - app_id
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
    - app_id
    - logs
        - tag
        - time
        - data
- response:
    - status: 200
    - status: 401
        - error: app_id not found
    - status: 400
        - error: invalid data

