BINARY_NAME=logprism
VERSION=1.2.1
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install release clean test bench fmt vet lint check

# 1. Build for your current machine
build:
	mkdir -p build
	go build $(LDFLAGS) -o build/$(BINARY_NAME) ./cmd/logprism

# 2. Install globally on your machine
install:
	go install $(LDFLAGS) ./cmd/logprism

# 3. Create binaries for all major platforms (Distribution)
release:
	mkdir -p bin
	# Linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/logprism
	# Mac (Intel)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/logprism
	# Mac (Silicon)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/logprism
	# Windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/logprism

# 4. Run all tests
test:
	go test -v ./cmd/... ./tests/...

# 4b. Run benchmarks
bench:
	go test -bench=. -benchmem -run=^$$ ./cmd/...

# 5. Format sources
fmt:
	go fmt ./...

# 6. Static analysis
vet:
	go vet ./...

# 7. golangci-lint (requires `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
lint:
	golangci-lint run ./...

# 8. Format + vet + test
check: fmt vet test

# 9. Clean up binaries
clean:
	rm -f $(BINARY_NAME)
	rm -f *.log *.txt
	rm -rf bin build
