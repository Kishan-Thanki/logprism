BINARY_NAME=logprism
VERSION=1.3.1
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install release clean test bench fmt vet lint check docker-build gen-perf

build:
	mkdir -p build
	go build $(LDFLAGS) -o build/$(BINARY_NAME) ./cmd/logprism

install:
	go install $(LDFLAGS) ./cmd/logprism

release:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/logprism
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/logprism
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/logprism
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/logprism

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY_NAME):$(VERSION) .

gen-perf:
	go run scripts/gen_perf_data.go

test:
	go test -v ./cmd/... ./tests/...

bench:
	go test -bench=. -benchmem -run=^$$ ./cmd/...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

check: fmt vet test

clean:
	rm -f $(BINARY_NAME) perf_1m.log bench_time.txt coverage.out
	rm -rf bin build
