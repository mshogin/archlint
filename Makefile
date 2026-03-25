.PHONY: help build install collect clean fmt test lint init build-rs

# Переменные
BIN_DIR := bin
BINARY := $(BIN_DIR)/archlint
GRAPH_DIR := arch
OUTPUT_ARCH := architecture.yaml
GITHUB_REPO := mshogin/archlint

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Собрать проект
	@echo "=== Сборка archlint ==="
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/archlint
	@echo "✓ Бинарник создан: $(BINARY)"

install: build ## Установить archlint в $GOPATH/bin
	@echo "=== Установка archlint ==="
	go install ./cmd/archlint
	@echo "✓ archlint установлен в GOPATH/bin"

collect: build ## Построить структурный граф для archlint
	@echo "=== Построение структурного графа ==="
	@mkdir -p $(GRAPH_DIR)
	$(BINARY) collect . -o $(GRAPH_DIR)/$(OUTPUT_ARCH)
	@echo "✓ Структурный граф сохранён: $(GRAPH_DIR)/$(OUTPUT_ARCH)"

fmt: ## Форматирование кода
	@echo "=== Форматирование кода ==="
	go fmt ./...
	@echo "✓ Готово"

test: ## Запустить тесты
	@echo "=== Запуск тестов ==="
	go test -v ./...

lint: ## Проверить код с помощью golangci-lint
	@echo "=== Проверка кода ==="
	@golangci-lint run ./...

clean: ## Очистить сгенерированные файлы
	@echo "=== Очистка ==="
	rm -rf $(BIN_DIR)
	rm -rf $(GRAPH_DIR)
	@echo "✓ Готово"

implement: ## Реализовать спецификацию с помощью Claude Code
	claude "$(cat .claude/contribution.md)"

init: ## Скачать archlint из GitHub Releases для текущей платформы
	@echo "=== Загрузка archlint ==="
	@mkdir -p $(BIN_DIR)
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m); \
	case "$$ARCH" in \
		x86_64)  ARCH="amd64" ;; \
		aarch64) ARCH="arm64" ;; \
		arm64)   ARCH="arm64" ;; \
		*)       echo "Unsupported architecture: $$ARCH"; exit 1 ;; \
	esac; \
	ASSET="archlint-$${OS}-$${ARCH}"; \
	echo "Platform: $${OS}-$${ARCH}"; \
	echo "Downloading: $${ASSET}"; \
	DOWNLOAD_URL=$$(curl -s "https://api.github.com/repos/$(GITHUB_REPO)/releases/latest" \
		| grep "browser_download_url.*$${ASSET}\"" \
		| cut -d '"' -f 4); \
	if [ -z "$$DOWNLOAD_URL" ]; then \
		echo "ERROR: Binary not found for $${OS}-$${ARCH}"; \
		echo "Available at: https://github.com/$(GITHUB_REPO)/releases"; \
		exit 1; \
	fi; \
	curl -sL "$$DOWNLOAD_URL" -o $(BINARY); \
	chmod +x $(BINARY); \
	echo "Verifying checksum..."; \
	CHECKSUMS_URL=$$(curl -s "https://api.github.com/repos/$(GITHUB_REPO)/releases/latest" \
		| grep "browser_download_url.*checksums-sha256.txt\"" \
		| cut -d '"' -f 4); \
	if [ -n "$$CHECKSUMS_URL" ]; then \
		EXPECTED=$$(curl -sL "$$CHECKSUMS_URL" | grep "$$ASSET" | awk '{print $$1}'); \
		ACTUAL=$$(sha256sum $(BINARY) 2>/dev/null || shasum -a 256 $(BINARY) 2>/dev/null); \
		ACTUAL=$$(echo "$$ACTUAL" | awk '{print $$1}'); \
		if [ "$$EXPECTED" = "$$ACTUAL" ]; then \
			echo "Checksum OK"; \
		else \
			echo "WARNING: Checksum mismatch!"; \
			echo "  Expected: $$EXPECTED"; \
			echo "  Actual:   $$ACTUAL"; \
		fi; \
	fi; \
	echo "Installed: $(BINARY) ($$(./$(BINARY) --version 2>/dev/null || echo 'unknown version'))"

build-rs: ## Собрать archlint-rs локально
	@echo "=== Сборка archlint-rs ==="
	@mkdir -p $(BIN_DIR)
	cd archlint-rs && cargo build --release
	cp archlint-rs/target/release/archlint $(BINARY)
	@echo "Binary: $(BINARY)"

all: fmt build collect ## Выполнить всё (форматирование, сборка, построение графа)

.DEFAULT_GOAL := help
