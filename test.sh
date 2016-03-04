#!/bin/sh -eux

goimports -w ./*.go ./cmd/smg/*.go
go tool vet .
golint .
goapp test ./...
