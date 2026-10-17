GO_TEST_FLAGS ?= -race

.PHONY: dev build test vet fmt ui-install ui-check ui-test check clean

dev:
	wails dev

build:
	wails build

test:
	go test $(GO_TEST_FLAGS) ./internal/...

fmt:
	@out="$$(gofmt -l . | grep -v '^\.worktrees/')"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

ui-install:
	pnpm -C frontend install

ui-check:
	pnpm -C frontend check

ui-test:
	pnpm -C frontend test

check: fmt vet test ui-check ui-test

clean:
	rm -rf build/bin
	find frontend/dist -mindepth 1 -not -name gitkeep -delete
