# ISUCON8 本戦問題

- [MANUAL](docs/MANUAL.md) はこちら

## 動作環境

- [Docker](https://www.docker.com/)
- [docker-compose](https://docs.docker.com/compose/)
- [Golang](https://golang.org/)
- [dep](https://golang.github.io/dep/docs/installation.html)

## webapp

### 起動方法

アプリケーションは `docker-compose` で動かします

```
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.go.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.perl.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.ruby.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.python.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.php.yml up [-d]
```


### blackboxの起動

競技中に使う外部APIとして下記の2種類があります。こちらも `docker-compose` で起動します

- bank   : 銀行API
- logger : ログAPI

```
docker-compose -f blackbox/docker-compose.local.yml up [-d]
```


## bench

### 準備

```
cd bench
make init
make deps
make build
```


### 実行

ベンチマークを実行するときは、webapp, blackbox の両方を起動した上で下記コマンドを実行してください

```
./bench/bin/bench

# 細かいオプションを指定する場合(手元では無いと思います)
./bench/bin/bench \
    -appep=https://localhost.isucon8.flying-chair.net \
    -bankep=https://compose.isucon8.flying-chair.net:5515 \
    -logep=https://compose.isucon8.flying-chair.net:5516 \
    -internalbank=https://localhost.isucon8.flying-chair.net:5515 \
    -internallog=https://localhost.isucon8.flying-chair.net:5516 \
    -result=/path/to/result.json \
    -log=/path/to/stderr.log
```

※ *.flying-chair.net 等のドメインの維持は保証しません
