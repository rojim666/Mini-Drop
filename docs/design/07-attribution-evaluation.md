# 07. Attribution Evaluation

## Scope

The current attribution loop is deterministic and tool-driven. It does not call
a remote LLM yet. Instead, it defines the same evidence boundary the later LLM
step must obey:

- `get_top_hotspots(task_id)` reads analyzer `topn.json`.
- `match_hotspot_rules(topn)` classifies hotspots into single-hotspot,
  runtime/scheduler, IO/storage, allocation, or mixed CPU paths.
- `compare_with_baseline(task_id)` compares the top hotspot with seeded baseline
  rows.

Every result is persisted in SQLite as `attribution_results`, including
conclusion, confidence, evidence, recommendations, source metadata, prompt text,
and tool trace.

## Seed Baselines

The API seeds three small baseline rows during migration:

| ID | Collector | Pattern | Expected |
|---|---|---|---|
| `base_runtime_schedule` | `mock-perf` | `runtime` | 18% |
| `base_storage_write` | `mock-perf` | `storage` | 12% |
| `base_malloc` | `perf` | `malloc` | 10% |

These are intentionally tiny. They prove the comparison contract without
pretending to be production-quality historical data.

## Rule Samples

Automated tests cover:

| Sample | Top Hotspot | Expected Result |
|---|---|---|
| Single hotspot | `main.spinCPU` at 60% | Conclusion contains `单个热点` |
| Runtime scheduling | `runtime.schedule` and `runtime.park_m` | Conclusion contains `runtime / scheduler` |
| IO/storage | `sqlite3_step` and `write` | Conclusion contains `IO / storage` |
| Baseline comparison | `storage.writeArtifacts` at 42% vs 12% baseline | Evidence contains `baseline` |

## Current Limitations

- Baselines are seeded demo rows, not historical production aggregates.
- The prompt is recorded but not sent to a remote LLM.
- The tool set does not yet include resource timeline comparison.
- The evaluator checks deterministic rule behavior, not natural-language
  answer quality.

## Acceptance Evidence

Run:

```bash
go test ./internal/apiserver
```

The tests verify that attribution is returned from task results, persisted on
first read, reused on later reads, includes a tool trace, and can include
baseline evidence.
