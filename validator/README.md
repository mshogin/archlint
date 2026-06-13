# archlint validator (Python) — DEPRECATED museum

Status: DEPRECATED. Tier-3 museum per ADR-0002. Research / manual use only.
NOT part of the production (boevoy) gate and NOT invoked from the agent loop or CI.

## What replaced it

The production path is the native Go detectors, run via `archlint scan`:

- SCC / circular dependencies (Tarjan, closed-world ERROR)
- dead-code (reach-from-entrypoints, open-world ERROR, delta-gated)
- ISP usage-subset (closed-world-on-subdomain ERROR, 2 syntactic guards)
- layering (back-edge against declared layer order)

These are soundness-gated and wired into the delta gate. The Go detectors are the
single source of truth for blocking decisions.

## Why this is kept (not deleted)

This package is the museum of ~229 Python metrics (structure + behavior families,
including the research math metrics: topology, spectral, information theory). It is
preserved as a reference corpus and as the source for the Python -> Go port:

- Structural, provable metrics -> being ported to Go (NetworkX -> gonum: DiGraph,
  Tarjan SCC, PageRank, centrality), validated golden-against-Python.
- Magnitude / experimental / behavior metrics -> stay here as museum (not gated).

The port scope is decided by soundness criteria, not bulk migration: only metrics
that survive the soundness gauntlet become Go detectors; the rest remain museum.

## How to run (manual / research only)

It is opt-in and never automatic:

```
# via the Go CLI (opt-in flag)
archlint validate architecture.yaml --python --group research

# or directly
python -m validator validate architecture.yaml -f yaml
```

Requires `pip install -e .` of the validator package or `ARCHLINT_VALIDATOR_PATH`.

## Do not

- Do not wire this into `archlint scan`, the MCP hot path, the agent loop, or CI.
- Do not treat its output as a gate signal — it is research/museum only.
