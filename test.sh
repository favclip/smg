#!/bin/sh -eux

go tool vet .
golint .
goapp test ./...
