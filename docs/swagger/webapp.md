ISUCOIN API
===========
世界最大の仮想椅子取引所「ISUCOIN」のAPIドキュメント


**Version:** 1.0.0

### /initialize
---
##### ***POST***
**Summary:** 初期化

**Description:** ベンチマークの初期化時に一度だけ外部APIに必要な情報をPOSTします
このAPIは10秒以内にレスポンスを返す必要があります


**Parameters**

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| bank_endpoint | formData | ISUBANK API エンドポイント | Yes | string |
| bank_appid | formData | ISUBANK アプリケーションID | Yes | string |
| log_endpoint | formData | ISU Logger API エンドポイント | Yes | string |
| log_appid | formData | ISU Logger アプリケーションID | Yes | string |

**Responses**

| Code | Description |
| ---- | ----------- |
| 200 | ok |
| 500 | Internal Server Error |

### /signup
---
##### ***POST***
**Summary:** ユーザー登録

**Description:** isubankidを用いて登録をします
ISUBANK API の Check を用いてisubankidの存在チェックを行うことができます


**Parameters**

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| name | formData | ISUCOIN上で利用するユーザー名 | Yes | string |
| bank_id | formData | ISUBANK ID ユニークである必要があります  | Yes | string |
| password | formData | Password | Yes | string |

**Responses**

| Code | Description |
| ---- | ----------- |
| 200 | ok |
| 400 | Invalid parameters |
| 404 | bank id not found |
| 409 | bank_id conflict |

### /signin
---
##### ***POST***
**Summary:** ログイン

**Description:** isubankidを用いてログインします


**Parameters**

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| bank_id | formData | ISUBANK ID ユニークである必要があります  | Yes | string |
| password | formData | Password | Yes | string |

**Responses**

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | ok | [User](#user) |
| 400 | Invalid parameters |  |
| 404 | bank id or password not match |  |

### /info
---
##### ***GET***
**Summary:** ページ情報取得

**Description:** クライアントはこのAPIをポーリングして情報を取得します


**Parameters**

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| cursor | query | 前回リクエスト時に返却されたcursorを指定 | No | integer |

**Responses**

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | ok | object |
| 401 | Unauthorized |  |

### /orders
---
##### ***GET***
**Summary:** 自分の注文履歴

**Description:** 自分が行ったすべての注文履歴を取得します
注文履歴には成立した注文は含まれますが、キャンセルした注文は含まれません


**Responses**

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | 自分の注文リスト | [ [Order](#order) ] |
| 401 | Unauthorized |  |

##### ***POST***
**Summary:** 注文

**Description:** 新規に注文を行います
買い注文を行う場合、ISUBANKの口座に注文金額に対して残高が不足している場合はエラーとなります


**Parameters**

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| type | formData | 注文種別 売り注文の場合=sell 買い注文=buy  | Yes | string |
| amount | formData | 注文脚数 | Yes | integer |
| price | formData | 1脚あたりの単価 ISUCOINでは指値注文のみに対応しています。 したがって、売り注文の場合は最低単価、買い注文の場合は最高単価となります  | Yes | integer |

**Responses**

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | 自分の注文リスト | object |
| 400 | Invalid parameters or Credit Insufficient |  |
| 401 | Unauthorized |  |

### /order/{id}
---
##### ***DELETE***
**Summary:** 注文のキャンセル

**Description:** 注文をキャンセルします


**Responses**

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | キャンセル成功した注文ID | object |
| 400 | Unauthorized |  |
| 404 | No Order or already closed |  |

### Models
---

### User  

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| id | integer | ISUCOINアプリケーションで払い出したユーザーの一意なID | No |
| name | string |  | No |

### Trade  

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| id | integer | 成立した取引ごとにユニークなID | No |
| amount | integer | 取引した総脚数 | No |
| price | integer | 1脚あたりの成立した単価 | No |
| created_at | dateTime | 取引成立時間 | No |

### Order  

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| id | integer | 一意な注文番号 | No |
| type | string | sell=売却
buy=購入
 | No |
| user_id | integer |  | No |
| amount | integer | 注文脚数 | No |
| price | integer | 1脚あたりの注文単価 | No |
| created_at | dateTime | 注文時間 | No |
| user | [User](#user) | 注文者情報 | No |
| trade | [Trade](#trade) | 取引が成立した場合設定される | No |

### CandlestickData  

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| time | dateTime |  | No |
| open | integer | 初値 | No |
| close | integer | 終値 | No |
| high | integer | 高値 | No |
| low | integer | 安値 | No |

### OrderLog  

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| order_id | integer |  | No |
| user_id | integer |  | No |
| amount | integer |  | No |
| price | integer |  | No |