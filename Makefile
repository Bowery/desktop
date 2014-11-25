DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format ui
	# @bash --norc ./scripts/build_client.sh
	@bash --norc ./scripts/build_server.sh
	# @bash --norc ./scripts/build_updater.sh
	@echo "--> Starting server..."
	-pkill -f bin/server
	osascript -e "do shell script \"sudo ./bin/server > debug.log 2>&1 &\" with administrator privileges"
	# @echo "--> Starting shell..."
	# @bash --norc ./scripts/run_shell.sh > debug.log 2>&1 &
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

ui:
	@myth -v ui/bowery/bowery.css ui/bowery/out.css > debug.log 2>&1
	@cd ui && bower install > debug.log 2>&1
	@mkdir -p bin
	@vulcanize --verbose --inline ui/bowery/bowery.html -o bin/app.html > debug.log 2>&1

release: ui
	@bash --norc ./scripts/release_app.sh

clean:
	-rm -rf pkg
	-rm -rf bowery/client/pkg
	-rm -rf bowery/agent/pkg
	-rm -rf dist
	-rm -rf bin
	-rm -f debug.log
	-rm -f goxc.log
	-pkill -f bin/client
	-pkill -f bin/server
	-pkill -f node_modules/.bin

extra-clean: clean
	-rm -rf build/node_modules
	-rm -rf build/atom-shell
	-rm -rf /tmp/atom

.PHONY: all deps test format clean release server client ui
