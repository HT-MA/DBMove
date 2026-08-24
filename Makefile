.PHONY: build up down demo-up demo-down logs verify verify-multi test clean helm-lint

build:
	docker compose build

up:
	docker compose up -d --build

down:
	docker compose down

demo-up:
	docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d --build

demo-down:
	docker compose -f docker-compose.yml -f docker-compose.demo.yml down

logs:
	docker compose logs -f api web

verify:
	powershell -ExecutionPolicy Bypass -File scripts/verify.ps1

verify-multi:
	powershell -ExecutionPolicy Bypass -File scripts/verify-multi.ps1

test:
	cd backend && go test ./...
	cd worker && go test ./...
	cd frontend && npm test

helm-lint:
	helm lint deploy/helm/dbmove

clean:
	docker compose down -v
