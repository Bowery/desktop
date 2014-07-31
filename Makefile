DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@mkdir -p bowery/client/bin
	@bash --norc -i ./bowery/scripts/build_client.sh
	@echo "--> Building OS X Wrapper..."
	@cd BoweryMenubarApp && xcodebuild -target Bowery > ../debug.log
	@echo "--> Opening Toolbar App..."
	@open BoweryMenubarApp/build/Release/Bowery.app
	@echo "--> Booting up client..."
	@BoweryMenubarApp/Bowery/Bowery/client > debug.log 2>&1 &
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
	@bash --norc -i ./bowery/scripts/release_agent.sh

clean:
	rm -rf **/bin
	rm -rf **/pkg
	rm -rf **/build
	rm -rf BoweryMenubarApp/Bowery/Bowery/
	pkill -f Bowery.app
	pkill -f Bowery/client

.PHONY: all deps test format
