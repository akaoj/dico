.PHONY: build dev providers tests


GOPATH = /repo/go/
export GOPATH

# This prevents Go to use the ~/.cache folder which is in the overlayfs and is destroyed between runs
GOCACHE = /repo/.cache
export GOCACHE

SRC_FILES = $(shell find go/src/dico -type f -name '*.go')
PROVIDERS_SRC_FILES = $(shell find go/src/dico/fetch/providers -type f -name '*.go')
PROVIDERS_BUILT_FILES = $(addsuffix .so,$(addprefix build/providers/,$(basename $(notdir ${PROVIDERS_SRC_FILES}))))

build/providers/%.so: go/src/dico/fetch/providers/%.go
	cd go/src/dico && go build -buildmode=plugin -o ../../../$@ fetch/providers/$(notdir $<)

build: ${SRC_FILES} .make.go-install
	cd go/src/dico && go build -ldflags "-X main.VERSION=${PROJECT_VERSION}" -o ../../../build/dico main.go

providers: ${PROVIDERS_BUILT_FILES}

shell:
	bash

.make.go-install: go/src/dico/go.mod go/src/dico/go.sum
	cd go/src/dico && go install
	touch $@
