VERSION ?= v$(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO ?= go
GORELEASER ?= goreleaser
LDFLAGS := -s -w -X github.com/impishMD/jeh/util.Ver=$(VERSION) -X github.com/impishMD/jeh/util.Commit=$(COMMIT) -X github.com/impishMD/jeh/util.Date=$(BUILD_DATE)

.PHONY: deps build frontend backend test lint docs docs-check check container release-check release-snapshot
deps:
	$(GO) mod download
	cd web && npm ci --no-audit --no-fund
frontend:
	cd web && npm run build
backend:
	CGO_ENABLED=0 $(GO) build -trimpath -tags netgo -ldflags '$(LDFLAGS)' -o bin/jeh ./cli
build: frontend backend
test:
	$(GO) test ./...
	cd web && npm run test:unit
lint:
	$(GO) vet ./...
	cd web && npm run lint -- --no-fix
docs:
	$(GO) run ./tools/docsref -out docs/en/reference/configuration.md
	$(GO) run ./tools/clidocs -out docs/en/reference/cli.md
docs-check: docs
	git diff --exit-code -- docs/en/reference
	python3 tools/check_docs.py
check: lint docs-check
container:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t impishmd/jeh:$(VERSION) .
release-check:
	$(GORELEASER) check
release-snapshot:
	JEH_SNAPSHOT_VERSION=$(shell cat VERSION)-dev $(GORELEASER) release --snapshot --clean --skip=publish
