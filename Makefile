.PHONY: build fetch build-db build-db-cc run test clean docker-up docker-down docker-logs

build:
	go build -o bin/cjp-etl ./cmd/cjp-etl
	go build -o bin/cjp-api ./cmd/cjp-api

fetch: build
	./bin/cjp-etl fetch

build-db: build
	./bin/cjp-etl build --db data/cjp.db --include-gpl=true

build-db-cc: build
	./bin/cjp-etl build --db data/cjp-cc.db --include-gpl=false

run: build
	./bin/cjp-api --addr :8088 --db data/cjp.db

test:
	go test -v ./...

clean:
	rm -rf bin/

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
