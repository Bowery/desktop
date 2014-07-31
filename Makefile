DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@mkdir -p bowery/client/bin
	@bash --norc -i ./bowery/scripts/build_client.sh
	@cd BoweryMenubarApp; xcodebuild -target Bowery
	@open BoweryMenubarApp/build/Release/Bowery.app
	BoweryMenubarApp/Bowery/Bowery/client > debug.log &

deps:
	@echo "--> Installing build dependencies"
	@go get -d -v ./...
	@echo $(DEPS) | xargs -n1 go get -d

format:
	@echo "--> Running go fmt"
	@gofmt -w bowery/

test: deps
	go test ./...

release:
	@bash --norc -i ./bowery/scripts/release_agent.sh

clean:
	rm -rf **/bin
	rm -rf **/pkg
	rm -rf **/build
	pkill -f Bowery.app
	pkill -f Bowery/client

.PHONY: all deps test format
