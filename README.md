## webapp

#### 起動方法

    cd webapp
    docker-compose up [-d] go

## blackbox

benchマーカーと対になるように用意したい

- bank   : 銀行API
- logger : ログAPI

#### 開発用の起動方法

    cd blackbox
    docker-compose up [-d]

## bench

    go run ./bench/cmd/bench/main.go -appep=http://127.0.0.1:12510 -bankep=http://172.17.0.1:5515 -logep=http://172.17.0.1:5516 -internalbank=http://127.0.0.1:5515
