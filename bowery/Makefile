DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@mkdir -p client/bin
	@bash --norc -i ./scripts/build_client.sh

deps:
	@echo "--> Installing build dependencies"
	@go get -d -v ./...
	@echo $(DEPS) | xargs -n1 go get -d

format:
	@echo "--> Running go fmt"
	@gofmt -w .

test: deps
	go test ./...

release:
	@bash --norc -i ./scripts/release_agent.sh

clean:
	rm -rf **/bin
	rm -rf **/pkg

.PHONY: all deps test format
