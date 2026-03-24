# Onboarding: How archlint can help your project

archlint is a Go architecture analysis tool. We can help your project with automated code quality analysis.

## What we offer

- **Architecture graphs** - visualize your codebase structure (components, dependencies, layers)
- **SOLID violations detection** - DIP, SRP, ISP checks on your Go code
- **Cycle detection** - find circular dependencies between packages
- **Metrics** - coupling, cohesion, fan-in/fan-out, health scores per package
- **Degradation tracking** - monitor how architecture quality changes over commits
- **Code review** - AI-powered architectural review of PRs

## How to start collaboration

1. Create an issue in your repo (or ours) proposing collaboration
2. We run archlint on your codebase and share results
3. You review our findings, we discuss improvements
4. Ongoing: we review each other's PRs, share architectural insights

## What we ask in return

We're open to collaboration exchange:
- Code review from your side
- Patterns and ideas we can learn from
- Integration feedback (how archlint works with your workflow)

## Communication

- Main channel: [Issue #3](https://github.com/mshogin/archlint/issues/3)
- Contributors: see [CONTRIBUTORS.md](CONTRIBUTORS.md)
- Rules: see [CLAUDE.md](CLAUDE.md)

## Example: self-scan results

We run archlint on itself. Results for 10 packages:
- 217 components, 310+ links
- 63 violations detected (DIP, SRP, feature envy, god classes)
- Health scores from 50/100 to 99/100
- Full results: [docs/self-scan/](docs/self-scan/)
