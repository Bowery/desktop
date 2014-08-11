DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@bash --norc ./scripts/build_client.sh
	@bash --norc ./scripts/build_agent.sh
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

release: agent client
	@echo "Done."

clean:
	-rm -rf pkg
	-rm -rf bowery/client/pkg
	-rm -rf bowery/agent/pkg
	-rm -rf dist
	-rm -rf bin
	-rm -f debug.log
	-rm -f goxc.log
	-pkill -f bin/client

extra-clean: clean
	-rm -rf build/node_modules
	-rm -rf build/atom-shell

.PHONY: all deps test format clean release agent client
