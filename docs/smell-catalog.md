# Architecture Smell Catalog

Reference guide for all architecture smells detected by archlint.

Each entry includes the violation kind (as reported in JSON output), severity, detection threshold, description, and a fix recipe.

---

## circular-dependency

- **Severity:** error
- **Threshold:** any cycle in import graph
- **Description:** Two or more packages depend on each other, creating a circular import chain. Go compiler forbids direct cycles; archlint detects indirect cycles that signal design problems even when allowed by tooling (e.g., within the same module via interfaces).
- **Fix:** Introduce an interface at the cycle boundary and apply dependency inversion. Move the shared abstraction to a third package that both sides can import without creating a loop.
- **Example:** `pkg/a` imports `pkg/b`, `pkg/b` imports `pkg/a` -> create `pkg/a/iface` with the interface, `pkg/b` depends on `iface` only.

---

## high-efferent-coupling

- **Severity:** warning
- **Threshold:** > 10 import dependencies per package
- **Description:** Package imports too many other packages. High efferent coupling means the package is fragile: any change in a dependency may require changes here. It also suggests the package may be doing too much.
- **Fix:** Extract a facade or mediator that groups related dependencies. Break the package into smaller, focused packages each responsible for one thing.
- **Example:** `internal/service` imports 12 packages -> split into `internal/service/order`, `internal/service/payment`, each with 4-5 imports.

---

## srp-too-many-methods

- **Severity:** warning
- **Threshold:** > 7 methods on a struct
- **Description:** Struct has too many methods, indicating it handles multiple responsibilities. Violates Single Responsibility Principle.
- **Fix:** Identify groups of related methods and extract each group into a dedicated struct with its own interface.
- **Example:** `OrderService` has 10 methods mixing order creation, payment, and notification -> split into `OrderCreator`, `PaymentProcessor`, `Notifier`.

---

## srp-too-many-fields

- **Severity:** warning
- **Threshold:** > 10 fields on a struct
- **Description:** Struct has too many fields, suggesting it holds data for multiple concerns. Violates Single Responsibility Principle.
- **Fix:** Group related fields into embedded structs or extract them into separate value objects.
- **Example:** `UserProfile` has 14 fields mixing auth, contact, and billing data -> extract `AuthInfo`, `ContactInfo`, `BillingInfo`.

---

## dip-concrete-dependency

- **Severity:** warning
- **Threshold:** struct field references a concrete type from another package
- **Description:** Struct depends directly on a concrete implementation rather than an abstraction. Violates Dependency Inversion Principle. Makes testing harder and couples the component to a specific implementation.
- **Fix:** Replace the concrete field type with an interface. Define the interface in the consuming package (consumer owns the interface).
- **Example:** `Handler struct { repo *postgres.Repository }` -> `Handler struct { repo RepositoryReader }` where `RepositoryReader` is a local interface.

---

## isp-fat-interface

- **Severity:** warning
- **Threshold:** > 5 methods on an interface
- **Description:** Interface declares too many methods. Clients are forced to depend on methods they do not use. Violates Interface Segregation Principle.
- **Fix:** Split the interface into smaller, role-specific interfaces. Compose them in the struct that needs all roles.
- **Example:** `Storage interface` with 9 methods -> split into `Reader`, `Writer`, `Deleter`; the repository struct implements all three.

---

## god-class

- **Severity:** warning
- **Threshold:** > 15 methods, or > 20 fields, or fan-out > 10 for a single struct
- **Description:** Struct does too much. It accumulates behavior and data across many concerns, becoming the central point of the codebase. Changes are risky because the class affects everything.
- **Fix:** Apply Single Responsibility Principle aggressively. Extract cohesive subsets of methods and fields into dedicated structs. Use composition and delegation.
- **Example:** `App struct` with 20 methods and 25 fields -> extract `Config`, `Server`, `Database`, `Cache` structs wired together in a `bootstrap` package.

---

## hub-node

- **Severity:** warning
- **Threshold:** fan-in + fan-out > 15 for a single function or method
- **Description:** Function or method is called by many callers and itself calls many other functions. Acts as a bottleneck in the call graph. Changes to a hub node have wide impact.
- **Fix:** Decompose the hub into smaller functions with narrower scope. Consider whether the hub is a symptom of a missing abstraction layer.
- **Example:** `processRequest()` called by 10 handlers and calling 8 services -> extract orchestration logic into a dedicated coordinator struct.

---

## feature-envy

- **Severity:** warning
- **Threshold:** method calls more methods on other types than on its own receiver (and > 2 external calls)
- **Description:** Method is more interested in the data of another type than its own. This is a sign that the method belongs in the other type.
- **Fix:** Move the method to the type whose data it uses most, or extract a service that encapsulates the cross-type operation.
- **Example:** `Order.CalculateDiscount()` calls 4 methods on `Customer` and 1 on `Order` -> move `CalculateDiscount` to `Customer` or extract a `DiscountCalculator`.

---

## shotgun-surgery

- **Severity:** warning
- **Threshold:** changes to a struct would affect > 5 distinct files
- **Description:** A single type is referenced in many places. Any change to its interface or behavior requires editing many files simultaneously. High change amplification risk.
- **Fix:** Introduce an abstraction (interface) in front of the widely-used type. Reduce direct coupling to the concrete type. Consider the Facade or Adapter pattern.
- **Example:** `DBClient` struct used directly in 8 files -> introduce `DataStore` interface; only one adapter file knows about `DBClient`.

---

## Severity Reference

| Kind | Severity | Blocks CI |
|------|----------|-----------|
| circular-dependency | error | yes (recommended) |
| high-efferent-coupling | warning | optional |
| srp-too-many-methods | warning | optional |
| srp-too-many-fields | warning | optional |
| dip-concrete-dependency | warning | optional |
| isp-fat-interface | warning | optional |
| god-class | warning | optional |
| hub-node | warning | optional |
| feature-envy | warning | optional |
| shotgun-surgery | warning | optional |

## Threshold Reference

| Kind | Threshold |
|------|-----------|
| circular-dependency | any cycle |
| high-efferent-coupling | > 10 imports |
| srp-too-many-methods | > 7 methods |
| srp-too-many-fields | > 10 fields |
| dip-concrete-dependency | any concrete cross-package field |
| isp-fat-interface | > 5 methods |
| god-class | > 15 methods OR > 20 fields OR fan-out > 10 |
| hub-node | fan-in + fan-out > 15 |
| feature-envy | external calls > own calls AND external calls > 2 |
| shotgun-surgery | used in > 5 distinct files |
