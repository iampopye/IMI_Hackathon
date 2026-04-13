.PHONY: run build test migrate lint docker-up docker-down docker-logs docker-build docker-rebuild clean vuln security install-tools \
        kind-create kind-load kind-deploy kind-up kind-status kind-delete \
        test-chaos test-race

KIND_CLUSTER ?= search-system

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

# ── Database ──────────────────────────────────────────────────────────────────
migrate:
	go run ./cmd/server   # server auto-runs migrations on startup

# ── Tests ─────────────────────────────────────────────────────────────────────
test:
	go test ./... -v -count=1

test-unit:
	go test ./... -v -count=1 -short

test-integration:
	go test ./... -v -count=1 -run Integration

test-chaos:
	go test ./... -v -count=1 -run Chaos

test-race:
	go test ./... -count=1 -race

bench:
	go test ./... -bench=. -benchmem -run='^$$'

# ── Code quality ──────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# ── Security (Phase 11) ────────────────────────────────────────────────────────
vuln:
	govulncheck ./...

security: vet vuln

install-tools:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ── Docker (Phase 12) ─────────────────────────────────────────────────────────
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-build:
	docker build -t search-system:latest .

# Rebuild the API image and restart only the api service (keeps infra running)
docker-rebuild:
	docker-compose build api
	docker-compose up -d --no-deps api

# Full teardown including named volumes (clears all persisted data)
docker-clean:
	docker-compose down -v --remove-orphans

# ── Kubernetes / Kind (Phase 13) ──────────────────────────────────────────────

# Create a local Kind cluster with host port 8080 mapped to NodePort 30080
kind-create:
	kind create cluster --name $(KIND_CLUSTER) --config k8s/kind-config.yaml

# Load the locally built image into the Kind cluster (no registry needed)
kind-load:
	kind load docker-image search-system:latest --name $(KIND_CLUSTER)

# Apply manifests in dependency order
kind-deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/secret.yaml
	kubectl apply -f k8s/network-policy.yaml
	kubectl apply -f k8s/postgres.yaml
	kubectl apply -f k8s/redis.yaml
	kubectl apply -f k8s/elasticsearch.yaml
	kubectl apply -f k8s/kafka.yaml
	kubectl apply -f k8s/api.yaml

# Full workflow: build image → load into Kind → deploy all manifests
kind-up: docker-build kind-load kind-deploy
	@echo "Cluster ready. API available at http://localhost:8080"

# Show pod / service / PVC status
kind-status:
	kubectl get pods,svc,pvc -n search-system

# Tear down the Kind cluster (deletes all data)
kind-delete:
	kind delete cluster --name $(KIND_CLUSTER)

# ── Cleanup ───────────────────────────────────────────────────────────────────
clean:
	rm -rf bin/
	go clean ./...
