.PHONY: build run clean

BINARY=cli-relay

build:
	go build -o bin/$(BINARY) .

run: build
	./bin/$(BINARY) -config config.yaml

clean:
	rm -f bin/$(BINARY)

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	gofmt -w .
