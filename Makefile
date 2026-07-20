.PHONY: build test vet clean run

BINARY=linksmith
CMD_DIR=cmd/linksmith

build:
	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./$(CMD_DIR)

test:
	go test ./... -count=1 -race

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) linksmith.json

docker-build:
	docker build -t linksmith .
