.PHONY: frontend build app test

ifeq ($(shell uname -s),Darwin)
# Match the Wails deployment target and LSMinimumSystemVersion in Info.plist.
MACOS_MIN_VERSION := 12.0
MACOS_MIN_FLAG := -mmacosx-version-min=$(MACOS_MIN_VERSION)
CGO_CFLAGS ?= -O2 -g
CGO_LDFLAGS ?= -O2 -g
BUILD_ENV := MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN_VERSION) \
	CGO_CFLAGS="$(CGO_CFLAGS) $(MACOS_MIN_FLAG)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS) $(MACOS_MIN_FLAG)"
endif

frontend:
	npm --prefix frontend ci
	npm --prefix frontend run build

build: frontend
	$(BUILD_ENV) go build -tags production -o branchpier .

app: build
	mkdir -p build/BranchPier.app/Contents/MacOS
	cp branchpier build/BranchPier.app/Contents/MacOS/branchpier
	cp packaging/Info.plist build/BranchPier.app/Contents/Info.plist

test:
	go test ./internal/gitrepo
