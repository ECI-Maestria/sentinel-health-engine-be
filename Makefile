.PHONY: all build test lint docker-build docker-push help

REGISTRY  ?= crsentinelhe.azurecr.io
TAG       ?= latest
SERVICES  := telemetry-service health-rules-service alerts-service

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

tidy: ## go mod tidy on all modules
	@for svc in $(SERVICES); do \
		echo "→ Tidying $$svc..."; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done
	cd shared && go mod tidy && cd ..

build: ## Build all services
	@for svc in $(SERVICES); do \
		echo "→ Building $$svc..."; \
		cd services/$$svc && go build ./... && cd ../..; \
	done

test: ## Run tests on all services
	@for svc in $(SERVICES); do \
		echo "→ Testing $$svc..."; \
		cd services/$$svc && go test ./... -v -count=1 && cd ../..; \
	done

docker-build: ## Build Docker images for all services
	@for svc in $(SERVICES); do \
		echo "→ Building image for $$svc..."; \
		docker build -t $(REGISTRY)/$$svc:$(TAG) -f services/$$svc/Dockerfile .; \
	done

docker-push: ## Push Docker images to ACR
	@for svc in $(SERVICES); do \
		docker push $(REGISTRY)/$$svc:$(TAG); \
	done

run-telemetry: ## Run telemetry-service locally
	cd services/telemetry-service && go run ./cmd/server/...

run-health-rules: ## Run health-rules-service locally
	cd services/health-rules-service && go run ./cmd/server/...

run-alerts: ## Run alerts-service locally
	cd services/alerts-service && go run ./cmd/server/...
