.PHONY: run test build clean create-category

run:
	go run main.go

test:
	go test -v ./...

build:
	go build -o bin/mtvl main.go

create-category:
	@if [ -z "$(NAME)" ]; then echo "Usage: make create-category NAME=category_name [DISPLAY='Display Name'] [DESC='Description']"; exit 1; fi
	go run cmd/create-category/main.go -name $(NAME) -display "$(DISPLAY)" -description "$(DESC)"

clean:
	rm -rf bin/ mtvl.db
