#!/usr/bin/env sh
go get github.com/mattn/goreman
go install mockservice/isubank
go install mockservice/isulogger
exec goreman start
