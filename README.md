## webapp

#### 起動方法

    cd webapp
    docker-compose up [-d]

### mockserviceの利用

blackbox APIを利用するためmockserviceを利用する方法です。

1. docker-composeでアプリの`links` に`mockservice`を追加してください
2. `/initialize` を手動で叩いてmockserviceを使うようにします

    curl https://localhost.isucon8.flying-chair.net/initialize \
        -d bank_endpoint=http://mockservice:14809 \
        -d bank_appid=mockbank \
        -d log_endpoint=http://mockservice:14690 \
        -d log_appid=mocklog

## blackbox

benchマーカーと対になるように用意したい

- bank   : 銀行API
- logger : ログAPI

#### 開発用の起動方法

    cd blackbox
    docker-compose -f docker-compose.local.yml up [-d]

## bench

    go run ./bench/cmd/bench/main.go \
        -appep=https://localhost.isucon8.flying-chair.net \
        -bankep=https://compose.isucon8.flying-chair.net:5515 \
        -logep=https://compose.isucon8.flying-chair.net:5516 \
        -internalbank=https://localhost.isucon8.flying-chair.net:5515 \
        -internallog=https://localhost.isucon8.flying-chair.net:5516 \
        -result=/path/to/result.json \
        -log=/path/to/stderr.log

    # 上記はdefaultなので下記で良いです
    go run ./bench/cmd/bench/main.go

    # defaultのresultはstdout, logはstderrなのでjqを使うと結果が見やすいです
    go run ./bench/cmd/bench/main.go | jq .

