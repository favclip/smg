#!/usr/bin/env bash -eux
go fmt ./...
goapp run cmd/smg/main.go -type Sample -output misc/fixture/a/model_search.go misc/fixture/a
goapp run cmd/smg/main.go -type Sample -output misc/fixture/b/model_search.go misc/fixture/b
goapp run cmd/smg/main.go -type Sample -output misc/fixture/c/model_search.go misc/fixture/c
goapp run cmd/smg/main.go -output misc/fixture/d/model_search.go misc/fixture/d
goapp run cmd/smg/main.go -output misc/fixture/e/model_search.go misc/fixture/e
