DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@bash --norc ./scripts/make_ui.sh
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
	@gofmt -w client/
	@gofmt -w updater/

test: deps
	@go test ./...

release: clean
	@echo "--> WARNING: YOU ARE PUTTING BOWERY IN THE WILD!!!"
	-rm -rf build/
	@bash --norc ./scripts/make_ui.sh
	@bash --norc ./scripts/release_client.sh

clean:
	@echo "--> Cleaning up a bit."
	-rm -rf pkg
	-rm -rf client/pkg
	-rm -rf dist
	-rm -rf bin
	-rm -f debug.log
	-rm -f goxc.log
	-pkill -f bin/client

extra-clean: clean
	@echo "--> Cleaning up a lot."
	-rm -rf /tmp/atom
	-rm -rf /tmp/shell
	-rm -rf build/

.PHONY: all deps format test release clean extra-clean
