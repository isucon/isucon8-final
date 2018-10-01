## webapp

#### 起動方法

    cd webapp
    docker-compose up [-d] isucoin-go

### mockserviceの利用

blackbox APIを利用するためmockserverを利用する方法です。

※ 尚、本番では初期状態でローカルに立てているmockserverを使うようにしたい

上記のdocker-composeでappを起動している場合mockserviceは一緒に起動しますが `/initialize` でmockserviceを使うように指定する必要があります。

    curl https://localhost.isucon8.flying-chair.net/initialize \
        -d bank_endpoint=http://mockservice:14809 \
        -d bank_appid=mockbank \
        -d log_endpoint=http://mockservice:14690 \
        -log_appid=mocklog

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
