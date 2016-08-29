#!/usr/bin/env bash -eux
go fmt ./...
go run cmd/smg/main.go -type Sample -output misc/fixture/a/model_search.go misc/fixture/a
go run cmd/smg/main.go -type Sample -output misc/fixture/b/model_search.go misc/fixture/b
go run cmd/smg/main.go -type Sample -output misc/fixture/c/model_search.go misc/fixture/c
go run cmd/smg/main.go -output misc/fixture/d/model_search.go misc/fixture/d
go run cmd/smg/main.go -output misc/fixture/e/model_search.go misc/fixture/e
go run cmd/smg/main.go -type Sample -output misc/fixture/f/model_search.go misc/fixture/f
