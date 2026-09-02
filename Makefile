.PHONY: fmt fmt-check mod-check vet test race build cross-build test-sqlite-drivers compare-sqlite-drivers smoke test-mysql test-mysql-down ci

BINARY ?= /tmp/dbprobe
MYSQL_COMPOSE := docker compose -f test/integration/mysql/docker-compose.yml
MYSQL80_DSN ?= mysql://dbprobe:dbprobe-pass@127.0.0.1:13306/shop
MYSQL84_DSN ?= mysql://dbprobe:dbprobe-pass@127.0.0.1:13307/shop
SQLITE_COMPARE_DIR := test/acceptance/sqlite-drivers

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
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/dbprobe

cross-build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/dbprobe-linux-amd64 ./cmd/dbprobe
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/dbprobe-windows-amd64.exe ./cmd/dbprobe
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /tmp/dbprobe-darwin-amd64 ./cmd/dbprobe
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/dbprobe-darwin-arm64 ./cmd/dbprobe

test-sqlite-drivers:
	cd $(SQLITE_COMPARE_DIR) && go mod tidy
	cd $(SQLITE_COMPARE_DIR) && git diff --exit-code -- go.mod go.sum
	cd $(SQLITE_COMPARE_DIR) && go test ./...

compare-sqlite-drivers: test-sqlite-drivers
	cd $(SQLITE_COMPARE_DIR) && ./compare.sh

smoke: build
	@set -eu; \
	data_root=$$(mktemp -d); \
	trap 'rm -rf "$$data_root"' EXIT; \
	export XDG_DATA_HOME="$$data_root"; \
	export LOCALAPPDATA="$$data_root"; \
	export HOME="$$data_root"; \
	$(BINARY) inspect fake://local --format=json --sample-window=1ms > /tmp/dbprobe-report.json; \
	grep -q '"schema_version": "dbprobe.inspect/v1alpha1"' /tmp/dbprobe-report.json; \
	grep -q '"delta": 40' /tmp/dbprobe-report.json; \
	$(BINARY) inspect fake://local --format=json --sample-window=1ms > /tmp/dbprobe-report-2.json; \
	$(BINARY) diff fake://local --format=json > /tmp/dbprobe-diff.json; \
	grep -q '"schema_version": "dbprobe.diff/v1alpha1"' /tmp/dbprobe-diff.json; \
	$(BINARY) inspect fake://local --sample-window=1ms | grep -q 'dbprobe · fake · local'; \
	if $(BINARY) inspect 'redis://user:secret@local' --sample-window=1ms >/tmp/dbprobe-bad.out 2>/tmp/dbprobe-bad.err; then echo 'unsupported scheme unexpectedly succeeded'; exit 1; fi; \
	if grep -q 'user:secret' /tmp/dbprobe-bad.err; then echo 'credential leaked in error output'; exit 1; fi; \
	if $(BINARY) inspect fake://local --format=xml --sample-window=1ms >/tmp/dbprobe-bad-format.out 2>/tmp/dbprobe-bad-format.err; then echo 'unsupported format unexpectedly succeeded'; exit 1; fi

test-mysql:
	@set -eu; \
	$(MYSQL_COMPOSE) up -d --wait; \
	trap '$(MYSQL_COMPOSE) down -v' EXIT; \
	DBPROBE_MYSQL_INTEGRATION=1 DBPROBE_MYSQL80_DSN='$(MYSQL80_DSN)' DBPROBE_MYSQL84_DSN='$(MYSQL84_DSN)' go test ./test/integration/mysql -v; \
	DBPROBE_TEST_MYSQL_DSN='$(MYSQL80_DSN)' go test ./test/contract -run TestAdapterContract/mysql -v; \
	DBPROBE_TEST_MYSQL_DSN='$(MYSQL84_DSN)' go test ./test/contract -run TestAdapterContract/mysql -v; \
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/dbprobe; \
	$(BINARY) inspect '$(MYSQL84_DSN)' --format=json --sample-window=10ms > /tmp/dbprobe-mysql-report.json; \
	grep -q '"schema_version": "dbprobe.inspect/v1alpha1"' /tmp/dbprobe-mysql-report.json; \
	grep -q '"engine": "mysql"' /tmp/dbprobe-mysql-report.json; \
	$(BINARY) explain '$(MYSQL84_DSN)' --statement "SELECT * FROM shop.customers WHERE email = 'alice@example.test'" --format=json > /tmp/dbprobe-mysql-explain.json; \
	grep -q '"schema_version": "dbprobe.explain/v1alpha1"' /tmp/dbprobe-mysql-explain.json; \
	grep -q '"sanitized": true' /tmp/dbprobe-mysql-explain.json; \
	grep -q 'mysql-json-sanitized' /tmp/dbprobe-mysql-explain.json; \
	if grep -q 'alice@example.test' /tmp/dbprobe-mysql-explain.json; then echo 'MySQL explain leaked query literal'; exit 1; fi; \
	if grep -q 'attached_condition' /tmp/dbprobe-mysql-explain.json; then echo 'MySQL explain leaked attached condition'; exit 1; fi; \
	if $(BINARY) inspect 'mysql://dbprobe:wrong-secret@127.0.0.1:13307/shop' --format=json --sample-window=10ms >/tmp/dbprobe-mysql-bad.out 2>/tmp/dbprobe-mysql-bad.err; then echo 'bad MySQL credentials unexpectedly succeeded'; exit 1; fi; \
	if grep -q 'wrong-secret' /tmp/dbprobe-mysql-bad.err; then echo 'MySQL credential leaked in error output'; exit 1; fi

test-mysql-down:
	$(MYSQL_COMPOSE) down -v

ci: mod-check fmt-check vet test race build cross-build smoke
