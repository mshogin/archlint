# Архитектурная валидация в archlint

> [🇬🇧 English version](VALIDATION.en.md)

Документ описывает систему валидации архитектуры, перенесенную из проекта aiarch.

## Обзор

archlint теперь включает 172 валидатора архитектуры, организованных в две семьи:
- **Structure** - статический анализ графа зависимостей (167 валидаторов)
- **Behavior** - динамический анализ контекстов/трейсов (8 валидаторов)

## Структура директорий

```
archlint/
├── validator/                    # Python валидаторы
│   ├── __init__.py
│   ├── __main__.py              # CLI точка входа
│   ├── common.py                # Общие утилиты
│   ├── config.py                # Конфигурация
│   ├── graph_loader.py          # Загрузка графа
│   ├── context_loader.py        # Загрузка контекстов
│   ├── requirements.txt         # Python зависимости
│   │
│   ├── structure/               # Семейство Structure
│   │   ├── core/                # Максимальный результат за минимум времени
│   │   │   └── metrics.py       # dag, fan_out, coupling, layers, forbidden_deps
│   │   ├── solid/               # SOLID принципы
│   │   │   └── solid_metrics.py # SRP, OCP, LSP, DIP
│   │   ├── patterns/            # Code smells
│   │   │   └── patterns_metrics.py
│   │   ├── architecture/        # Clean/Hexagonal
│   │   │   └── architecture_metrics.py
│   │   ├── quality/             # Качество и безопасность
│   │   │   └── quality_metrics.py
│   │   ├── advanced/            # Продвинутая аналитика
│   │   │   ├── advanced_metrics.py
│   │   │   └── change_metrics.py
│   │   └── research/            # Математические/экспериментальные
│   │       ├── topology_metrics.py
│   │       ├── information_theory_metrics.py
│   │       ├── linear_algebra_metrics.py
│   │       ├── category_theory_metrics.py
│   │       └── ... (15 модулей)
│   │
│   └── behavior/                # Семейство Behavior
│       ├── core/                # Базовые проверки контекстов
│       │   └── context_metrics.py
│       └── advanced/            # Продвинутые проверки
│           └── context_metrics.py
│
└── internal/cli/
    └── validate.go              # Go команда для вызова Python
```

## Группы валидаторов

### Structure - Core (10 валидаторов)

Максимальный результат за минимальное время:

| Валидатор | Описание |
|-----------|----------|
| dag_check | Обнаружение циклов зависимостей |
| max_fan_out | Слишком много исходящих зависимостей |
| coupling | Ca/Ce метрики связности |
| instability | Направление зависимостей (I = Ce/(Ca+Ce)) |
| layer_violations | Нарушение архитектурных слоев |
| forbidden_dependencies | Запрещенные связи |
| hub_nodes | God Objects (много связей) |
| orphan_nodes | Изолированные компоненты |
| strongly_connected_components | Взаимные зависимости |
| graph_depth | Глубина цепочек зависимостей |

### Structure - SOLID (5 валидаторов)

| Валидатор | Описание |
|-----------|----------|
| single_responsibility | SRP - одна причина для изменения |
| open_closed | OCP - открыт для расширения, закрыт для модификации |
| liskov_substitution | LSP - заменяемость подтипов |
| dependency_inversion | DIP - зависимость от абстракций |
| interface_segregation | ISP - маленькие интерфейсы |

### Structure - Patterns (8 валидаторов)

| Валидатор | Описание |
|-----------|----------|
| god_class | God Classes |
| shotgun_surgery | Shotgun Surgery |
| divergent_change | Divergent Change |
| lazy_class | Lazy Classes |
| middle_man | Классы-посредники |
| speculative_generality | Неиспользуемые абстракции |
| data_clumps | Data Clumps |
| feature_envy | Feature Envy |

### Structure - Architecture (7 валидаторов)

| Валидатор | Описание |
|-----------|----------|
| domain_isolation | Изоляция domain слоя |
| ports_adapters | Ports & Adapters |
| use_case_purity | Чистота use cases |
| dto_boundaries | DTO на границах |
| inward_dependencies | Зависимости к центру |
| bounded_context_leakage | Изоляция Bounded Contexts |
| service_autonomy | Автономность сервисов |

### Structure - Quality (10 валидаторов)

| Валидатор | Описание |
|-----------|----------|
| auth_boundary | Аутентификация на границе |
| sensitive_data_flow | Поток чувствительных данных |
| input_validation_layer | Валидация на входе |
| mockability | Возможность мокирования |
| test_isolation | Изоляция тестов |
| test_coverage_structure | Структурное покрытие |
| logging_coverage | Покрытие логированием |
| metrics_exposure | Экспозиция метрик |
| health_check_presence | Наличие health check |
| api_consistency | Консистентность API |

### Structure - Advanced (~25 валидаторов)

Продвинутая аналитика: centrality, modularity, change propagation, blast radius и др.

### Structure - Research (~90 валидаторов)

Математические подходы:
- Topology (Betti numbers, Euler characteristic)
- Information Theory (entropy, mutual information)
- Linear Algebra (SVD, spectral analysis)
- Category Theory (functors, monads)
- Game Theory (Shapley value, Nash equilibrium)
- Combinatorics и другие

### Behavior - Core (4 валидатора)

| Валидатор | Описание |
|-----------|----------|
| context_coverage | Покрытие критических компонентов тестами |
| untested_components | Непокрытые компоненты |
| ghost_components | Компоненты в тестах, но не в архитектуре |
| context_complexity | Сложность контекста |

### Behavior - Advanced (4 валидатора)

| Валидатор | Описание |
|-----------|----------|
| single_point_of_failure | Компоненты во всех контекстах |
| context_coupling | Связность между контекстами |
| layer_traversal | Нарушение слоев в execution flow |
| context_depth | Глубина стека вызовов |

## Использование

### Go CLI

```bash
# Базовая валидация (core группа)
archlint validate architecture.yaml

# С контекстами (behavior валидация)
archlint validate architecture.yaml -c contexts.yaml

# Конкретная группа
archlint validate architecture.yaml -g solid

# Параллельный запуск всех групп
archlint validate architecture.yaml -p

# JSON вывод
archlint validate architecture.yaml -f json -o report.json
```

### Python CLI (напрямую)

```bash
cd archlint
python -m validator validate architecture.yaml
python -m validator validate architecture.yaml -g core
python -m validator list
```

## Архитектура

```
Go CLI (archlint)
    |
    v
Python Validator (subprocess)
    |
    +-- Structure validators
    |       +-- core (NetworkX)
    |       +-- solid
    |       +-- patterns
    |       +-- architecture
    |       +-- quality
    |       +-- advanced
    |       +-- research (NumPy, SciPy)
    |
    +-- Behavior validators
            +-- core
            +-- advanced
```

Go CLI вызывает Python валидаторы через subprocess. При флаге `-p` (parallel) группы запускаются параллельно через goroutines.

## Конфигурация

Валидаторы настраиваются через `.archlint.yaml`:

```yaml
validators:
  dag_check:
    enabled: true
    error_on_violation: true
    exclude:
      - "external/*"
      - "generated/*"

  max_fan_out:
    threshold: 5
    error_on_violation: false

  layer_violations:
    params:
      layers:
        cmd: 0
        api: 1
        service: 2
        domain: 3
        repository: 4
```

## Зависимости

Python:
- networkx >= 3.0 (граф алгоритмы)
- pyyaml >= 6.0 (парсинг YAML)
- numpy >= 1.24 (для advanced/research)
- scipy >= 1.10 (для advanced/research)

## Миграция из aiarch

Все 172 валидатора перенесены из aiarch без изменения логики. Изменения:
1. Реорганизация по семействам (Structure/Behavior) и группам
2. Добавлен Python CLI (__main__.py)
3. Интеграция с Go CLI через subprocess
4. Поддержка параллельного запуска групп
