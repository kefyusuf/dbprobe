.PHONY: fmt fmt-check vet test race ci

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

ci: fmt-check vet test
