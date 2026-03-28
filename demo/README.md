# Demo: archlint violations for Stachka 2026

Intentionally broken Go project used to demonstrate archlint during the Stachka 2026 talk.

---

## Quick start

```bash
# Run archlint against the demo project
cd demo/
archlint analyze ./...
```

---

## Violation map

| File | Violation | Rule |
|------|-----------|------|
| `internal/handler/api.go` | 14 imports (fan-out > 5) | `max-fan-out: 5` |
| `internal/handler/api.go` | imports `repo` directly, skipping service layer | `forbidden: handler -> repo` |
| `internal/repo/cache.go` | imports `tracking` (handler-layer), wrong direction | `forbidden: repo -> handler` |
| `internal/model/model.go` | `User` struct has 17 methods (god class > 15) | `max-methods: 15` |
| `internal/config/config.go` | `Configurator` interface has 8 methods (ISP violation > 5) | `max-interface-methods: 5` |

---

## Presenter script

### 1. Introduce the project (30 sec)

> "This is a standard Go REST API. Four layers: handler, service, repo, model.
> Looks clean from the outside. Let's see what archlint finds."

### 2. Run archlint (live demo)

```bash
archlint analyze ./...
```

Expected output (violations highlighted):

```
VIOLATION  handler/api.go:14  fan-out=14 (allowed: 5)
VIOLATION  handler/api.go:29  forbidden import: handler -> repo (layer skip)
VIOLATION  repo/cache.go:12   forbidden import: repo -> tracking (upward dependency)
VIOLATION  model/model.go:13  god class: User has 17 methods (allowed: 15)
VIOLATION  config/config.go:20 fat interface: Configurator has 8 methods (allowed: 5)

5 violations found in 5 files.
```

### 3. Walk through each violation

#### Fan-out (handler/api.go)

> "Handler imports 14 packages. Our budget is 5.
> Each import is a dependency. More dependencies = harder to test, harder to change.
> The fix: move business logic to service, move formatting to model presenters."

Point at the import block at the top of `handler/api.go`.
All `// VIOLATION: fan-out #N` comments are visible to the audience.

#### Layer skip (handler/api.go -> repo)

> "Handler talks directly to the repository. No service in between.
> That means business rules live in the HTTP layer. When you add a gRPC endpoint,
> you copy-paste the rules. When rules change, you update two places."

Show `h.userRepo.Save(&u)` on line ~65.

#### Upward dependency (repo/cache.go -> tracking)

> "Repository imports tracking - a package that belongs to the handler layer.
> Dependencies should only flow downward: handler -> service -> repo.
> Here repo reaches back up. That breaks the layered architecture."

Show the import block in `repo/cache.go`.

#### God class (model/model.go)

> "User has 17 methods. It validates itself, formats itself, activates itself,
> scores itself, and prints itself.
> It knows too much. When anything about users changes, this file changes.
> The fix: split into User (data), UserValidator, UserFormatter, UserStateMachine."

Show the method list in `model/model.go`.

#### Fat interface (config/config.go)

> "Configurator has 8 methods. Any client that depends on it is forced to
> import database config even if it only needs the port number.
> Interface Segregation: split into ServerConfig, DatabaseConfig, CacheConfig."

Show the `Configurator` interface in `config/config.go`.

### 4. Show the clean layer (service/service.go)

> "For contrast - service.go. Two imports, one interface with two methods.
> This is what the other files should look like."

### 5. Close (30 sec)

> "archlint found 5 violations in under a second.
> No manual review, no tribal knowledge required.
> Run it in CI and violations never reach main."

---

## Architecture diagram

```
cmd/main.go
    |
    v
handler/api.go  ---[VIOLATION: direct]---> repo/repo.go
    |                                          ^
    |                                          |
    v                                    [VIOLATION: upward]
service/service.go ---> repo/repo.go <-- repo/cache.go
                                              |
                                        tracking/ (handler-layer)
```

Correct flow (no violations):

```
cmd -> handler -> service -> repo -> model
                    ^
                    |-- depends on repo interface only
```
