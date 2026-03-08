.PHONY: run worker dev build test tidy swagger docker-up docker-down

run:
	go run main.go start

worker:
	go run main.go worker

dev:
	air

build:
	go build -o bin/srg main.go

test:
	go test ./... -v

tidy:
	go mod tidy

swagger:
	swag init --parseDependency --parseInternal

docker-up:
	docker compose up -d

docker-down:
	docker compose down
