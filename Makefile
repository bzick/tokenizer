lint:
	golangci-lint run ./...

test:
	go test -v ./...

.PHONY: lint test
