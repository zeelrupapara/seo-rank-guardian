.PHONY: run dev build test tidy swagger docker-up docker-down worker-py-setup worker-py

run:
	go run main.go start

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

worker-py-setup:
	cd py-worker && python3.13 -m venv .venv && .venv/bin/pip install -r requirements.txt

worker-py:
	cd py-worker && .venv/bin/python main.py
