# Spec 0020: Реестр сервисов

**Metadata:**
- Priority: 0020 (High)
- Status: Todo
- Created: 2026-04-20
- Effort: M
- Parent Spec: -
- GitLab Issue: (internal tracker)

---

## Overview

### Problem Statement

В arch-repo отсутствует централизованный реестр сервисов. Сервисы описаны в различных README.md проектов без единой структуры и метаданных. Это затрудняет:

- Поиск и навигацию по существующим сервисам
- Понимание связей между сервисами и системами
- Аудит и ревью архитектурных изменений
- Онбординг новых участников

### Solution Summary

Создать реестр сервисов в формате markdown-файлов с YAML frontmatter. Каждый сервис - отдельный файл с единой структурой атрибутов, связанный с системой.

### Success Metrics

- Единая структура для всех сервисов
- Быстрый поиск через имена файлов и заголовки
- Визуализация связей сервисов с системами
- Возможность автоматической генерации индексов

---

## Architecture

### Data Model

| Type | Fields |
|------|--------|
| Service | name, description, system, owner, status, created, updated, links |
| System | name, description, team, subsystems, status |

---

## Requirements

### R1: Frontmatter Schema

**Description:** Каждый файл сервиса должен иметь YAML frontmatter с фиксированным набором полей.

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Название сервиса |
| `description` | string | yes | Описание назначения |
| `system` | string | yes | Система, к которой относится сервис |
| `owner` | string | no | Owner команды |
| `status` | string | yes | `active` / `planned` / `deprecated` |
| `created` | date | yes | Дата создания записи |
| `updated` | date | yes | Дата последнего обновления |
| `links` | []string | no | Ссылки на связанные сервисы |

**Example Frontmatter:**
```yaml
---
name: "viola-api"
description: "API для фиксации нарушений"
system: "shtrafy-i-narusheniya"
owner: "violations"
status: active
created: 2024-01-15
updated: 2026-04-20
links:
  - "calculation-penalty"
---
```

---

### R2: Directory Structure

**Description:** Файлы сервисов хранятся в отдельной директории с логической организацией.

**Structure:**
```
teams-registry/
├── services/          # Реестр сервисов
│   ├── 01-xxxx.md    # Сервисы с номерами для порядка
│   └── ...
├── services-index.md  # Индекс сервисов по системам
└── systems.md         # Список систем с их сервисами
```

**Naming Convention:**
```
NNNN-short-description.md
```
- `NNNN` = 4-значный ID (0001-9999)
- Меньше число = выше приоритет/стратегичность

---

### R3: System-Service Relationship

**Description:** Каждый сервис должен быть связан с системой через поле `system`.

**Requirements:**
- Поле `system` ссылается на название системы
- Индекс сервисов группирует по системам
- Возможность просмотра всех сервисов системы

---

## Acceptance Criteria

- [ ] AC1: Создана директория `teams-registry/services/`
- [ ] AC2: Создан файл `teams-registry/services-index.md` с индексом сервисов
- [ ] AC3: Создано 10-20 примеров файлов сервисов (реальные данные)
- [ ] AC4: Frontmatter содержит все обязательные поля из R1
- [ ] AC5: Для каждого сервиса указан `system` и `status`
- [ ] AC6: Индекс содержит таблицы с фильтрацией по системе
- [ ] AC7: Добавлен README.md с инструкцией по использованию
- [ ] AC8: Создан файл `teams-registry/systems.md` со списком систем и их сервисами

---

## Implementation Steps

### Phase 1: Foundation

**Step 1.1:** Создать структуру директорий
- Files: `teams-registry/services/`
- Action: Create
- Details: Создать `services/`, `templates/`

**Step 1.2:** Создать шаблон файла сервиса
- Files: `teams-registry/templates/service.md`
- Action: Create
- Details: Структура markdown с frontmatter

---

### Phase 2: Core Content

**Step 2.1:** Создать индекс сервисов
- Files: `teams-registry/services-index.md`
- Action: Create
- Details: Таблица с сервисами по системам

**Step 2.2:** Создать список систем
- Files: `teams-registry/systems.md`
- Action: Create
- Details: Список всех систем с их сервисами

**Step 2.3:** Заполнить реестр реальными сервисами
- Files: `teams-registry/services/0001-*.md` (10-20 файлов)
- Action: Create
- Details: Берутся данные из существующих README.md проектов

---

### Phase 3: Documentation

**Step 3.1:** Создать README.md
- Files: `teams-registry/README.md`
- Action: Create
- Details: Инструкция по добавлению/обновлению

**Step 3.2:** Обновить навигацию
- Files: `Teams/README.md`
- Action: Modify
- Details: Добавить ссылку на новый реестр

---

## Testing Strategy

### Verification Steps
- [ ] Проверить структуру директорий
- [ ] Проверить валидность YAML в frontmatter
- [ ] Проверить связи сервисов с системами
- [ ] Проверить индексацию таблиц

---

## Notes

### Design Decisions

**Decision:** Отдельные файлы для каждого сервиса

**Rationale:**
- Меньшие MR (лучше для review)
- История изменений по каждому сервису
- Минимизация конфликтов при параллельной работе

**Decision:** Связь через поле `system`

**Rationale:**
- Простота добавления/удаления сервисов
- Гибкость в изменении связей
- Удобство фильтрации

### Code Examples

**Validating frontmatter:**
```bash
# Проверка всех файлов на валидность YAML
find teams-registry/services -name "*.md" -exec sh -c '
  head -n 20 "$1" | grep -A 20 "^---" | yq . > /dev/null 2>&1 || echo "Invalid: $1"
' _ {} \;
```

### References

- Related: Issue #1 (Registry of Systems)
- KAFKA-TOPICS-REGISTRY.md - пример реестра
- Spec workflow: `~/.claude/rules/spec-workflow.md`
