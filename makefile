GO := go
DOCKER := docker

REGISTRY := mylists

IMAGE = $(file < TAG)
VERSION = $(file < VERSION)
TAG = $(IMAGE):$(VERSION)

.PHONY: run test build clean create-cate$(GO)ry

all: upload

run:
	$(GO) run main.$(GO)

test:
	$(GO) test -v ./...

build:
	$(GO) build -o bin/mtvl main.$(GO)

create-cate$(GO)ry:
	@if [ -z "$(NAME)" ]; then echo "Usage: make create-cate$(GO)ry NAME=cate$(GO)ry_name [DISPLAY='Display Name'] [DESC='Description']"; exit 1; fi
	$(GO) run cmd/create-cate$(GO)ry/main.$(GO) -name $(NAME) -display "$(DISPLAY)" -description "$(DESC)"

clean:
	rm -rf bin/ mtvl.db

upload:
	$(DOCKER) buildx build \
		--push \
		--platform linux/amd64,linux/arm64 \
		--tag $(REGISTRY)/$(IMAGE):$(VERSION) \
		--tag $(REGISTRY)/$(IMAGE):latest \
		--target deploy .
