VERSION ?= 0.3.0
GOFLAGS ?= -mod=vendor

.PHONY: build run app dev-db test clean

build:
	go build $(GOFLAGS) -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/navmini ./cmd/navmini

run:
	go run $(GOFLAGS) ./cmd/navmini

app:
	VERSION=$(VERSION) ./scripts/build-app.sh

dev-db:
	./scripts/dev-db.sh start

test:
	go test $(GOFLAGS) ./...

clean:
	rm -rf bin dist .build
