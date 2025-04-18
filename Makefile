.PHONY: build run clean

SRCS = $(shell find . -name '*.go') Dockerfile Makefile go.mod go.sum
PORT = 8080

run: .build
	docker run --rm -p '${PORT}:8080' -it -v '${PWD}:/app' $(shell cat $<)

build: .build
.build: ${SRCS}
	docker build -q --network=none . > $@

clean:
	rm -f .build

.DELETE_ON_ERROR:
