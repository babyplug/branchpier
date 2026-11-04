.PHONY: frontend build app test

frontend:
	npm --prefix frontend ci
	npm --prefix frontend run build

build: frontend
	go build -tags production -o branchpier .

app: build
	mkdir -p build/BranchPier.app/Contents/MacOS
	cp branchpier build/BranchPier.app/Contents/MacOS/branchpier
	cp packaging/Info.plist build/BranchPier.app/Contents/Info.plist

test:
	go test ./internal/gitrepo
