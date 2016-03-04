#!/bin/sh -eux

goimports -w .
go tool vet .
golint .
goapp test ./...
