.PHONY: fmt fmt-check mod-check vet test race build smoke test-mysql test-mysql-down ci

BINARY ?= /tmp/dbprobe
MYSQL_COMPOSE := docker compose -f test/integration/mysql/docker-compose.yml
MYSQL80_DSN ?= mysql://dbprobe:dbprobe-pass@127.0.0.1:13306/shop
MYSQL84_DSN ?= mysql://dbprobe:dbprobe-pass@127.0.0.1:13307/shop

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"

mod-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

build:
	go build -o $(BINARY) ./cmd/dbprobe

smoke: build
	$(BINARY) inspect fake://local --format=json --sample-window=1ms > /tmp/dbprobe-report.json
	@grep -q '"schema_version": "dbprobe.inspect/v1alpha1"' /tmp/dbprobe-report.json
	@grep -q '"delta": 40' /tmp/dbprobe-report.json
	@$(BINARY) inspect fake://local --sample-window=1ms | grep -q 'dbprobe · fake · local'
	@if $(BINARY) inspect 'redis://user:secret@local' --sample-window=1ms >/tmp/dbprobe-bad.out 2>/tmp/dbprobe-bad.err; then echo 'unsupported scheme unexpectedly succeeded'; exit 1; fi
	@if grep -q 'user:secret' /tmp/dbprobe-bad.err; then echo 'credential leaked in error output'; exit 1; fi
	@if $(BINARY) inspect fake://local --format=xml --sample-window=1ms >/tmp/dbprobe-bad-format.out 2>/tmp/dbprobe-bad-format.err; then echo 'unsupported format unexpectedly succeeded'; exit 1; fi

test-mysql:
	@set -eu; \
	$(MYSQL_COMPOSE) up -d --wait; \
	trap '$(MYSQL_COMPOSE) down -v' EXIT; \
	DBPROBE_MYSQL_INTEGRATION=1 DBPROBE_MYSQL80_DSN='$(MYSQL80_DSN)' DBPROBE_MYSQL84_DSN='$(MYSQL84_DSN)' go test ./test/integration/mysql -v; \
	DBPROBE_TEST_MYSQL_DSN='$(MYSQL80_DSN)' go test ./test/contract -run TestAdapterContract/mysql -v; \
	DBPROBE_TEST_MYSQL_DSN='$(MYSQL84_DSN)' go test ./test/contract -run TestAdapterContract/mysql -v

test-mysql-down:
	$(MYSQL_COMPOSE) down -v

ci: mod-check fmt-check vet test race smoke
