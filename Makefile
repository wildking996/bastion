# Bastion V3 Makefile

APP_NAME=bastion

# 版本信息 - 自动从 git tag 获取，如果没有则使用 dev
VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# ldflags 用于注入版本信息
LDFLAGS=-s -w
LDFLAGS+= -X bastion/version.Version=$(VERSION)
LDFLAGS+= -X bastion/version.CommitHash=$(COMMIT_HASH)
LDFLAGS+= -X bastion/version.BuildTime=$(BUILD_TIME)

# 默认目标
.PHONY: all
all: build

# 编译
.PHONY: build
build:
	@echo "Building $(APP_NAME) $(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME)

# 运行
.PHONY: run
run:
	@echo "Running $(APP_NAME)..."
	go run main.go

# 清理
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(APP_NAME) $(APP_NAME).exe *.db

# 测试
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# 跨平台编译
.PHONY: build-all
build-all: build-windows build-linux build-darwin build-linux-arm build-darwin-arm

.PHONY: build-windows
build-windows:
	@mkdir -p dist
	@echo "Generating Windows resources..."
	@command -v rsrc >/dev/null 2>&1 && rsrc -ico icon.ico -o rsrc.syso || echo "rsrc not found, building without icon"
	@echo "Building for Windows amd64 (GUI mode - no console)..."
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS) -H windowsgui" -o dist/$(APP_NAME)-windows-amd64.exe
	@echo "Building for Windows amd64 (Console mode)..."
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-windows-amd64-console.exe

.PHONY: build-linux
build-linux:
	@mkdir -p dist
	@echo "Building for Linux amd64..."
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-linux-amd64

.PHONY: build-darwin
build-darwin:
	@mkdir -p dist
	@echo "Building for macOS amd64..."
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-amd64

.PHONY: build-linux-arm
build-linux-arm:
	@mkdir -p dist
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-linux-arm64

.PHONY: build-darwin-arm
build-darwin-arm:
	@mkdir -p dist
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-arm64

# 安装依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# 格式化代码
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run

# 显示版本信息
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT_HASH)"
	@echo "Build Time: $(BUILD_TIME)"

# 帮助
.PHONY: help
help:
	@echo "Bastion V3 Makefile"
	@echo "Current version: $(VERSION)"
	@echo ""
	@echo "Usage:"
	@echo "  make build       - Build the application"
	@echo "  make run         - Run the application"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make test        - Run tests"
	@echo "  make build-all   - Build for all platforms"
	@echo "  make deps        - Download dependencies"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make version     - Show version info"