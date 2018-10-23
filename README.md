# ISUCON8 本戦問題

- [MANUAL](docs/MANUAL.md) はこちら

## 本番当日の動作環境

### マシンスペック

チームごとに1物理マシンを割り当てており、それぞれのチームごとのVMは下記のようになっていました。

- 各チーム毎に配布したVM x 4
    - vCPU 2コア : Intel(R) Xeon(R) CPU E5-2640 v4 @ 2.40GHz
    - メモリ 1GB
    - ネットワーク帯域 1Gbps
    - ディスク SSD
- ベンチマーカー x 1
    - vCPU 3コア : Intel(R) Xeon(R) CPU E5-2640 v4 @ 2.40GHz
    - メモリ 2GB
    - ネットワーク帯域 1Gbps
    - ディスク SSD
- 外部API x 1
    - vCPU 3コア : Intel(R) Xeon(R) CPU E5-2640 v4 @ 2.40GHz
    - メモリ 2GB
    - ネットワーク帯域 1Gbps
    - ディスク SSD


### ネットワーク

[マニュアル](docs/MANUAL.md) に記載があるように、グローバルIPとプライベートIPとベンチマーカーIPの3つのNICが存在し、それぞれネットワークは別れておりました。


### 初期状態

2018/10/20(土)の本戦当日は、isuconユーザーのhomeディレクトリは下記のようになっておりました。
※ このリポジトリのwebappのみを配置しかつ `webapp/sql` ディレクトリはrmしておりました。

```
isucon2018-final
  ├── docs
  └── webapp
      ├── mockservice
      ├── mysql
      ├── nginx
      ├── public
      ├── go
      ├── perl
      ├── php
      ├── python
      └── ruby
```

※ サービスは、下記の`/etc/systemd/system/isucoin.service`によってsystemdで起動しておりました
```
[Unit]
Description = isucoin application

[Service]
LimitNOFILE=102400
LimitNPROC=102400

WorkingDirectory=/home/isucon/isucon2018-final/webapp

ExecStartPre = /usr/local/bin/docker-compose -f docker-compose.yml -f docker-compose.go.yml build
ExecStart = /usr/local/bin/docker-compose -f docker-compose.yml -f docker-compose.go.yml up
ExecStop = /usr/local/bin/docker-compose -f docker-compose.yml -f docker-compose.go.yml down

Restart   = always
Type      = simple
User      = isucon
Group     = isucon

[Install]
WantedBy = multi-user.target
```

### 本戦当日のスコア

- 初期スコア
    - go:      500前後
    - ruby:    500前後
    - python:  500前後
    - php:    1000前後
    - perl:   1200前後
- 優勝スコア
    - 35,312 (最大の敵は時差)
- 最大スコア
    - 51,834 (takedashi)

## ローカルでのアプリケーションの起動

### 動作環境

- [Docker](https://www.docker.com/)
- [docker-compose](https://docs.docker.com/compose/)
- [Golang](https://golang.org/)

### webappの起動方法

アプリケーションは `docker-compose` で動かします

```
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.go.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.perl.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.ruby.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.python.yml up [-d]
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.php.yml up [-d]
```

### webappの言語実装を切り替える場合

一度downしてからbuildしてupし直します 

例: go→perl
```
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.go.yml down
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.perl.yml build
docker-compose -f webapp/docker-compose.yml -f webapp/docker-compose.perl.yml up [-d]
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
