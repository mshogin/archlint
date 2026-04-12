---
title: "Scan Results"
description: "Nightly architecture analysis of open-source projects"
---

Last scan: 2026-04-12
Repositories monitored: 15

## Health Dashboard

| Repository | Language | Health | Passed | Failed | Warnings |
|-----------|----------|--------|--------|--------|----------|
| [joho/godotenv](joho-godotenv/) | Go | 88% (good) | 32 | 2 | 1 |
| [fatih/color](fatih-color/) | Go | 88% (good) | 32 | 2 | 1 |
| [caarlos0/env](caarlos0-env/) | Go | 86% (good) | 31 | 2 | 2 |
| [charmbracelet/log](charmbracelet-log/) | Go | 86% (good) | 33 | 2 | 2 |
| [mikeshogin/seclint](mikeshogin-seclint/) | Go | 74% (moderate) | 31 | 4 | 3 |
| [mikeshogin/promptlint](mikeshogin-promptlint/) | Go | 69% (moderate) | 30 | 5 | 3 |
| [mikeshogin/costlint](mikeshogin-costlint/) | Go | 69% (moderate) | 29 | 5 | 3 |
| [orhun/git-cliff](orhun-git-cliff/) | Rust | 59% (needs work) | 28 | 7 | 3 |
| [sharkdp/vivid](sharkdp-vivid/) | Rust | 51% (needs work) | 25 | 9 | 2 |
| [svenstaro/miniserve](svenstaro-miniserve/) | Rust | 39% (poor) | 22 | 11 | 3 |
| [mshogin/archlint](mshogin-archlint/) | Go | 34% (poor) | 22 | 12 | 3 |
| [kgatilin/deskd](kgatilin-deskd/) | Rust | 29% (poor) | 23 | 13 | 3 |
| [ducaale/xh](ducaale-xh/) | Rust | 26% (poor) | 20 | 14 | 2 |
| [sharkdp/hyperfine](sharkdp-hyperfine/) | Rust | 24% (poor) | 20 | 14 | 3 |
| [twitchtv/twirp](twitchtv-twirp/) | Go | 14% (poor) | 17 | 16 | 3 |

## How it works

Every night at 3:00 UTC, archlint clones each monitored repository, builds the architecture dependency graph, and runs 229+ validators across core metrics, SOLID principles, and advanced graph analysis.

Health score: `100 - (failed * 5) - (warnings * 2)`, minimum 0.

Want your repo scanned? [Open an issue](https://github.com/mshogin/archlint/issues/new?title=Add+repo:+owner/name).
