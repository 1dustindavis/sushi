LAST_TAG := $(shell git describe --tags --abbrev=0 --match '[0-9]*.[0-9]*' 2>/dev/null || echo 0.0)
SHORT_SHA := $(shell git rev-parse --short HEAD)
VERSION ?= $(LAST_TAG)+$(SHORT_SHA)

PLATFORMS := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: build
build:
	@mkdir -p dist
	@for target in $(PLATFORMS); do \
		goos=$${target%/*}; \
		goarch=$${target#*/}; \
		output="sushi-$(VERSION)-$${goos}-$${goarch}"; \
		if [ "$$goos" = "windows" ]; then output="$$output.exe"; fi; \
		echo "building dist/$$output"; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=0 \
			go build -ldflags "-X main.version=$(VERSION)" -o "dist/$$output" ./cmd/sushi; \
	done
