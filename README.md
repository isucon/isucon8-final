## webapp

### 起動方法

アプリケーションは `docker-compose` で動かします

	docker-compose -f webapp/docker-compose.yml up [-d]


### blackboxの起動

競技中に使う外部APIとして下記の2種類があります。こちらも `docker-compose` で起動します

- bank   : 銀行API
- logger : ログAPI

	cd blackbox
	docker-compose -f blackbox/docker-compose.local.yml up [-d]


### mockserviceの利用

blackbox APIを利用せずにblackbox APIを使う場合です。

1. docker-composeでアプリの`links` に`mockservice`を追加してください
2. `/initialize` を手動で叩いてmockserviceを使うようにします

	curl https://localhost.isucon8.flying-chair.net/initialize \
	    -d bank_endpoint=http://mockservice:14809 \
	    -d bank_appid=mockbank \
	    -d log_endpoint=http://mockservice:14690 \
	    -d log_appid=mocklog

※ ただし、[docs/MANUAL.md](docs/MANUAL.md) にあるように `isucon-{001..100}` のbankidを利用できるためblackbox を起動している場合は原則必要ありません。(blackboxの存在を知らない競技中に使うためのものです)


## bench

ベンチマークを実行するときは、webapp, blackbox の両方を起動した上で下記コマンドを実行してください

	go run ./bench/cmd/bench/main.go
	
	# defaultのresultはstdout, logはstderrなのでjqを使うと結果が見やすいです
	go run ./bench/cmd/bench/main.go | jq .
	
	# 細かいオプションを指定する場合(手元では無いと思います)
	go run ./bench/cmd/bench/main.go \
	    -appep=https://localhost.isucon8.flying-chair.net \
	    -bankep=https://compose.isucon8.flying-chair.net:5515 \
	    -logep=https://compose.isucon8.flying-chair.net:5516 \
	    -internalbank=https://localhost.isucon8.flying-chair.net:5515 \
	    -internallog=https://localhost.isucon8.flying-chair.net:5516 \
	    -result=/path/to/result.json \
	    -log=/path/to/stderr.log


### 負荷試験前のテストのみを行う

負荷実行は60秒間継続するため、負荷走行前のテストのみを行うツールも用意しています。
言語移植などに取り組む場合は、主にこちらで互換性を確認すると待ち時間を減少できます


	go run ./bench/cmd/isucointest/main.go
	
	# 細かいオプションを指定する場合(手元では無いと思います)
	go run ./bench/cmd/isucointest/main.go \
	    -appep=https://localhost.isucon8.flying-chair.net \
	    -bankep=https://compose.isucon8.flying-chair.net:5515 \
	    -logep=https://compose.isucon8.flying-chair.net:5516 \
	    -internalbank=https://localhost.isucon8.flying-chair.net:5515 \
	    -internallog=https://localhost.isucon8.flying-chair.net:5516
