.PHONY: build dev tests


GOPATH = /repo/go/
export GOPATH

# This prevents Go to use the ~/.cache folder which is in the overlayfs and is destroyed between runs
GOCACHE = /repo/.cache
export GOCACHE

SRC_FILES = $(shell find go/src/dico -type f -name '*.go')

build: ${SRC_FILES}
	cd go/src/dico && go build -ldflags "-X main.VERSION=${PROJECT_VERSION}" -o ../../../build/dico main.go

shell:
	bash

.make.go-install: go/src/dico/go.mod go/src/dico/go.sum
	cd go/src/dico && go install
	touch $@
