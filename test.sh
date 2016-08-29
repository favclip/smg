#!/bin/sh -eux

goimports -w ./*.go ./cmd/smg/*.go ./smgutils/*.go
go tool vet .
golint .
goapp test ./... $@
