lint:
	golangci-lint run ./...

test:
	go test -v ./...

bench:
	go test -run=^$$ -bench=. -benchmem ./...

.PHONY: lint test bench
