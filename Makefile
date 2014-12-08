DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@bash --norc ./scripts/build_client.sh
	@bash --norc ./scripts/build_updater.sh
	@echo "--> Starting shell..."
	@bash --norc ./scripts/run_shell.sh > debug.log 2>&1 &
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

agent:
	@echo "--> Releasing agent..."
	@bash --norc ./scripts/release_agent.sh

client:
	@bash --norc ./scripts/release_client.sh

clean:
	-rm -rf pkg
	-rm -rf client/pkg
	-rm -rf agent/pkg
	-rm -rf dist
	-rm -rf bin
	-rm -f debug.log
	-rm -f goxc.log
	-pkill -f bin/client

extra-clean: clean
	-rm -rf /tmp/atom
	-rm -rf /tmp/shell
	-rm -rf build/

.PHONY: all deps test format clean agent client
