.DEFAULT_GOAL := help

APP_NAME := moleport
BUILD_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOFLAGS := -trimpath
LDFLAGS := -s -w -X github.com/ousiassllc/moleport/internal/cli.Version=$(VERSION)

.PHONY: help build run clean test test-race vet fmt lint linterly install update setup-tools

help: ## ヘルプを表示
	@echo ""
	@echo "  MolePort - SSH Port Forwarding Manager"
	@echo ""
	@echo "  Usage: make <target>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""

build: ## バイナリをビルド (bin/moleport)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/moleport

run: build ## ビルドして実行
	./$(BUILD_DIR)/$(APP_NAME)

test: ## テストを実行
	go test ./...

test-race: ## テストを実行 (race detector 付き)
	go test -race ./...

vet: ## go vet を実行
	go vet ./...

fmt: ## go fmt を実行
	gofmt -w .

lint: ## golangci-lint を実行
	golangci-lint run ./...

linterly: ## linterly でファイル行数チェック
	linterly check .

setup-tools: ## 開発ツールをインストール
	go install github.com/ousiassllc/linterly/cmd/linterly@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

clean: ## ビルド成果物を削除
	rm -rf $(BUILD_DIR)

update: ## 最新版にアップデート (go install)
	go install github.com/ousiassllc/moleport/cmd/moleport@latest

install: ## $GOPATH/bin にインストール
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/moleport
	@GOBIN=$$(go env GOPATH)/bin; \
	case ":$$PATH:" in \
		*":$$GOBIN:"*) ;; \
		*) echo ""; \
		   echo "  \033[33m警告: $$GOBIN が PATH に含まれていません\033[0m"; \
		   echo "  以下をシェル設定に追加してください:"; \
		   echo ""; \
		   echo "    export PATH=\"\$$PATH:$$GOBIN\""; \
		   echo "" ;; \
	esac
