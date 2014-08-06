DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@echo "--> Building client..."
	@bash --norc -i ./scripts/build_client.sh
	@echo "--> Starting shell..."
	@bash --norc -i ./scripts/run_shell.sh > debug.log 2>&1 &
	@echo "Done."

deps:
	@echo "--> Installing build dependencies"
	@go get -d -v ./...
	@echo $(DEPS) | xargs -n1 go get -d

format:
	@echo "--> Running go fmt"
	@gofmt -w bowery/

test: deps
	@go test ./...

release:
	@bash --norc -i ./scripts/release_agent.sh

clean:
	rm debug.log
	rm -rf bin
	rm -rf pkg
	pkill -f client

.PHONY: all deps test format clean
