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
- `get_resource_timeline(task_id)` reads analyzer `resource_timeline.json`
  when it exists. For `perf`, the analyzer derives this timeline from
  `perf script` sample timestamps. For mock, eBPF syscall, and py-spy artifacts,
  the analyzer emits the same structured contract from profile samples so the
  Web and attribution store do not need collector-specific branches.

Every result is persisted in SQLite as `attribution_results`, including
conclusion, confidence, evidence, recommendations, source metadata, prompt text,
resource timeline JSON, and tool trace.

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
| Single CPU hotspot | `main.spinCPU` at 64% | Conclusion contains `单个热点` |
| Runtime scheduling | `runtime.schedule` and `runtime.park_m` | Conclusion contains `runtime / scheduler` |
| IO/storage baseline regression | `storage.writeArtifacts` at 42% vs 12% baseline | Evidence contains `baseline` |
| Perf allocation with timeline | `malloc` with `perf_script_samples` timeline | Conclusion contains allocation guidance and the timeline remains attached |
| Mixed CPU hotspots | `business.route`, `crypto.hash`, `template.render` | Conclusion avoids overconfident single-cause attribution |
| eBPF syscall distribution | `read`, `write`, `epoll_wait` | Conclusion contains `IO / storage` with syscall timeline alignment |

## Scored Report

Run:

```bash
make attribution-evaluation
```

Or, without `make`:

```powershell
python scripts\demo\write_attribution_evaluation.py
```

The command writes `artifacts/attribution-evaluation-report.md`. It executes
`go test -run TestAttributionEvaluationSamples -v ./internal/apiserver`,
parses the emitted `ATTRIBUTION_EVAL` JSON lines, and creates a reviewable
Markdown report.

Each of the six samples is scored on a 10-point rubric:

| Criterion | Weight |
|---|---:|
| conclusion | 2 |
| evidence | 2 |
| timeline | 2 |
| tool_trace | 2 |
| recommendation | 1 |
| confidence | 1 |

The full report is a 60-point evidence artifact for the deterministic
attribution loop. `make final-preflight` also generates this report and fails
the recording gate if any required sample fails.

## Current Limitations

- Baselines are seeded demo rows, not historical production aggregates.
- The prompt is recorded but not sent to a remote LLM.
- Resource timeline data is now a structured analyzer artifact. It is derived
  from profile samples and `perf script` timestamps, not from independent node
  CPU / IO / memory / wait metric collectors yet.
- The evaluator checks deterministic rule behavior, not natural-language
  answer quality.

## Acceptance Evidence

Run:

```bash
go test ./internal/apiserver
make attribution-evaluation
```

The tests verify that attribution is returned from task results, persisted on
first read, reused on later reads, includes a tool trace, includes resource
timeline evidence, can include baseline evidence, and produces a scored
Markdown evaluation report for review.
