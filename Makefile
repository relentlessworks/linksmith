.PHONY: build test vet clean run

BINARY=linksmith
CMD_DIR=cmd/linksmith

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./$(CMD_DIR)

test:
	go test ./...

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) linksmith.json

docker-build:
	docker build -t linksmith .
