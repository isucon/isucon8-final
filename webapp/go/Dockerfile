FROM golang:1.11

RUN apt-get update && apt-get -y install mysql-client
RUN curl -sL https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 > /usr/bin/dep && chmod +x /usr/bin/dep
