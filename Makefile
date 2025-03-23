# Makefile for FrameHound

# Variables
APP_NAME := framehound
DIST_DIR := ./dist
BUILD_DIR := ./build
# Check if the git repo is dirty
IS_DIRTY := $(shell git diff --quiet || echo "dirty")
# Use Development Version if the repo is dirty
ifeq ($(IS_DIRTY),dirty)
VERSION := Development Version
COMMIT := unknown
else
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "Development Version")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
endif
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X 'main.Version=${VERSION}' -X 'main.BuildDate=${BUILD_DATE}' -X 'main.Commit=${COMMIT}'"

# Default target
.PHONY: all
all: build

# Download required dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies downloaded"

# Build the application with ldflags
.PHONY: build
build: deps
	@echo "Building ${APP_NAME}..."
	@mkdir -p ${DIST_DIR}
	@go build ${LDFLAGS} -o ${DIST_DIR}/${APP_NAME}
	@echo "Build complete: ${DIST_DIR}/${APP_NAME}"

# Build the application with a specific version
.PHONY: release
release: deps
	@echo "Building release ${APP_NAME} v${VERSION}..."
	@mkdir -p ${DIST_DIR}
	@go build ${LDFLAGS} -o ${DIST_DIR}/${APP_NAME}
	@echo "Release build complete: ${DIST_DIR}/${APP_NAME} v${VERSION}"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -f ${APP_NAME}
	@rm -rf ${DIST_DIR}
	@echo "Clean complete"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./...
	@echo "Tests complete"

# Install the application
.PHONY: install
install: build
	@echo "Installing ${APP_NAME}..."
	@go install ${LDFLAGS}
	@echo "Installation complete"

# Show help
.PHONY: help
help:
	@echo "FrameHound Makefile targets:"
	@echo "  all      - Default target, builds the application"
	@echo "  build    - Build the application with current git version into ${DIST_DIR} directory"
	@echo "  release  - Build the application for release with version information"
	@echo "  clean    - Remove build artifacts and ${DIST_DIR} directory"
	@echo "  test     - Run tests"
	@echo "  install  - Install the application to GOPATH/bin"
	@echo "  help     - Show this help message"
	@echo ""
	@echo "Current version: ${VERSION}"
	@echo "Build date: ${BUILD_DATE}"
	@echo "Git commit: ${COMMIT}" 