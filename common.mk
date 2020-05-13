# Author: Valentin Bremond (https://github.com/akaoj/common.mk)
# Licensed under MIT License, see the LICENSE file

# Use sensible defaults
SHELL := /usr/bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --no-builtin-rules  # these are mainly rules for C and older languages
MAKEFLAGS += --warn-undefined-variables  # complain when variables are undefined

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))/common-custom.mk

# Public targets
.PHONY: build clean dev help image push tests
# Internal targets
.PHONY: _dev_image _requirements
.DEFAULT_GOAL = help

ifeq (${GIT_BRANCH},)
    GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
endif

ifeq (${SERVICE_NAME},)
    $(error "The SERVICE_NAME variable has to be defined in your Makefile")
endif

# USER_ID will be used in the dev container to keep the same ID as the host user (useful for files
# permissions)
USER_ID = $(shell id --user)
export USER_ID

PWD = $(shell pwd)
PROJECTS_DIR = $(shell git rev-parse --show-toplevel)

# Get a "safe" representation of the branch (replace all non [a-z0-9.-] characters with a dash)
GIT_BRANCH_CLEANED = $(shell echo "${GIT_BRANCH}" | sed -E 's/[^a-zA-Z0-9.-]+/-/g')
GIT_CURRENT_COMMIT = $(shell git log --format="%h" -n 1 --abbrev-commit --abbrev=8)
GIT_CURRENT_TAG = $(shell git describe --exact-match --tags 2>/dev/null)

# PROJECT_VERSION is injected in dev container and can be used at build time to set the version in
# the build
PROJECT_VERSION = ${GIT_CURRENT_TAG}
ifeq (${PROJECT_VERSION},)
    PROJECT_VERSION = ${GIT_CURRENT_COMMIT}
endif


DEV_IMAGE_NAME = ${REGISTRY_IMAGE_PREFIX}/${SERVICE_NAME}:dev
PROD_IMAGE_NAME_COMMIT = ${REGISTRY_IMAGE_PREFIX}/${SERVICE_NAME}:${GIT_CURRENT_COMMIT}
PROD_IMAGE_NAME_BRANCH = ${REGISTRY_IMAGE_PREFIX}/${SERVICE_NAME}:${GIT_BRANCH_CLEANED}

_DOCKER_ARGS = -it
ifeq (${CI},true)
    # Disable TTY mode
    _DOCKER_ARGS =
endif

SUDO = sudo --preserve-env


_requirements:
	@command -v docker &>/dev/null || { echo 'You need docker, please install it first'; exit 1; }
	@command -v docker-compose &>/dev/null || { echo 'You need docker-compose, please install it first'; exit 1; }


_dev_image: _requirements
	cd ${PROJECTS_DIR} && ${SUDO} docker-compose --file docker-compose-dev.yml build ${SERVICE_NAME}


define container_make
$(SUDO) docker run ${_DOCKER_ARGS} --user=${USER_ID}:${USER_ID} --rm --network=host -e "PROJECT_VERSION=${PROJECT_VERSION}" -v ${PROJECTS_DIR}:/repo:rw,Z ${DEV_IMAGE_NAME} make --file=dev.mk $1 $2
endef

define HELP_CONTENT
The following commands are available with `make`:

  build:    Build the code (compile, transpile, bundle, ...).
  clean:    Clean the builds.
  dev:      Run locally a development version of the service/project.
  help:     Get this help.
  image:    Build a production-ready Docker image.
  tests:    Run the tests.
  push:     Push the image to the registry.
endef
export HELP_CONTENT


build/: build


# Public targets

help:
	@echo "$$HELP_CONTENT"


build: _dev_image
	mkdir -p build/
	$(call container_make,build,)


clean:
	find build/ -mindepth 1 -delete


dev: _dev_image
	cd ${PROJECTS_DIR} && ${SUDO} docker-compose --file docker-compose-dev.yml up --force-recreate -d ${SERVICE_NAME}
	${SUDO} docker logs -f ${SERVICE_NAME}


tests: _dev_image
	$(call container_make,tests,)


image: build/
	cd build/ && $(SUDO) docker build -t ${PROD_IMAGE_NAME_COMMIT} .


push:
	# Tag and push the image with commit ID (immutable tags) and branch name (mutable tags)
	# Note that the branch will be a git tag in the case the CI runs the build on a git tag.
	$(SUDO) docker tag ${PROD_IMAGE_NAME_COMMIT} ${REGISTRY_URL}/${PROD_IMAGE_NAME_COMMIT}
	$(SUDO) docker push ${REGISTRY_URL}/${PROD_IMAGE_NAME_COMMIT}
	$(SUDO) docker tag ${PROD_IMAGE_NAME_COMMIT} ${REGISTRY_URL}/${PROD_IMAGE_NAME_BRANCH}
	$(SUDO) docker push ${REGISTRY_URL}/${PROD_IMAGE_NAME_BRANCH}
