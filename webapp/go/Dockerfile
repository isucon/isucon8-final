FROM golang:1.10

RUN apt-get update && apt-get -y install mysql-client
RUN curl -sL https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 > /usr/local/bin/dep && chmod +x /usr/local/bin/dep
