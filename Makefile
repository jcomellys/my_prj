.PHONY: help build run test fmt vet tidy clean

BINARY := agent
PKG    := ./...

help:
	@echo "Targets:"
	@echo "  make build   - compile the agent binary to ./bin/$(BINARY)"
	@echo "  make run     - run the agent with config.yaml (creates from example if missing)"
	@echo "  make test    - run unit tests"
	@echo "  make fmt     - format all Go code"
	@echo "  make vet     - static analysis"
	@echo "  make tidy    - tidy go.mod"
	@echo "  make clean   - remove build artifacts"

build:
	@mkdir -p bin
	go build -o bin/$(BINARY) ./cmd/agent

run: config.yaml
	go run ./cmd/agent --config config.yaml

config.yaml:
	@if [ ! -f config.yaml ]; then \
		cp config.example.yaml config.yaml; \
		echo "Created config.yaml from config.example.yaml — edit it to taste."; \
	fi

test:
	go test -race -count=1 $(PKG)

fmt:
	gofmt -s -w .

vet:
	go vet $(PKG)

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/ $(BINARY)
