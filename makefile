GO := go
DOCKER := docker

REGISTRY := mylists

IMAGE = $(file < TAG)
VERSION = $(file < VERSION)
TAG = $(IMAGE):$(VERSION)

.PHONY: run test build clean create-category upload docker

all: upload

run:
	$(GO) run .

test:
	$(GO) test -v ./...

build:
	$(GO) build -o bin/mtvl .

create-category:
	@if [ -z "$(NAME)" ]; then echo "Usage: make create-category NAME=category_name [DISPLAY='Display Name'] [DESC='Description']"; exit 1; fi
	$(GO) run cmd/create-category/main.go -name $(NAME) -display "$(DISPLAY)" -description "$(DESC)"

clean:
	rm -rf bin/ mtvl.db

upload:
	$(DOCKER) buildx build \
		--push \
		--platform linux/amd64,linux/arm64 \
		--tag $(REGISTRY)/$(IMAGE):$(VERSION) \
		--tag $(REGISTRY)/$(IMAGE):latest \
		--target deploy .

docker:
	$(DOCKER) buildx build \
		--tag $(REGISTRY)/$(IMAGE):$(VERSION) \
		--tag $(REGISTRY)/$(IMAGE):latest \
		--target deploy .
