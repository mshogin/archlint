.PHONY: help build install collect trace clean fmt test lint

# Переменные
BIN_DIR := bin
BINARY := $(BIN_DIR)/archlint
TRACELINT := $(BIN_DIR)/tracelint
GRAPH_DIR := arch
OUTPUT_ARCH := architecture.yaml

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

lint: ## Проверить код с помощью golangci-lint и tracelint
	@echo "=== Проверка кода ==="
	@echo ""
	@echo "--- golangci-lint ---"
	@golangci-lint run ./... || true
	@echo ""
	@echo "--- tracelint ---"
	@mkdir -p $(BIN_DIR)
	go build -o $(TRACELINT) ./cmd/tracelint
	@echo "✓ tracelint собран: $(TRACELINT)"
	@echo "Проверка ./internal ./pkg ./cmd ..."
	@$(TRACELINT) ./internal/... ./pkg/... ./cmd/... || true

clean: ## Очистить сгенерированные файлы
	@echo "=== Очистка ==="
	rm -rf $(BIN_DIR)
	rm -rf $(GRAPH_DIR)
	@echo "✓ Готово"

implement: ## Реализовать спецификацию с помощью Claude Code
	claude "$(cat .claude/contribution.md)"

all: fmt build collect ## Выполнить всё (форматирование, сборка, построение графа)

.DEFAULT_GOAL := help
