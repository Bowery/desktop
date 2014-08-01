DEPS=$(shell go list -f '{{join .Deps "\n"}}' ./... | xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}' | sed 's&github.com/Bowery.*&&')

all: format deps
	@mkdir -p bowery/client/bin
	@bash --norc -i ./scripts/build_client.sh
	@echo "--> Building OS X Wrapper..."
	@cd BoweryMenubarApp && xcodebuild -target Bowery &> ../debug.log
	@echo "--> Opening Toolbar App..."
	@open BoweryMenubarApp/build/Release/Bowery.app
	@echo "--> Booting up client..."
	@BoweryMenubarApp/Bowery/Bowery/client &> debug.log &
	@echo "Done."

release-agent: format deps
	@bash --norc -i ./scripts/release_agent.sh

test: format deps
	@go test ./...

deps:
	@echo "--> Installing build dependencies"
	@echo ${DEPS} | xargs -n1 go get -d

format:
	@echo "--> Running go fmt"
	@gofmt -w bowery/

clean:
	rm -f debug.log
	rm -rf **/bin
	rm -rf **/pkg
	rm -rf **/build
	rm -rf BoweryMenubarApp/Bowery/Bowery/
	pkill -f Bowery.app || true
	pkill -f Bowery/client || true

.PHONY: all release-agent test deps format clean
