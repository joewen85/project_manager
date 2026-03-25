# API Benchmark Compare

- before: `docs/benchmark/before.template.md`
- after: `docs/benchmark/after.md`
- generated_at: `2026-03-25 17:05:59 +0800`

| Endpoint | Before avg (s) | Before min/max (s) | After avg (s) | After min/max (s) | Diff avg |
|---|---:|---:|---:|---:|---:|
| `/api/v1/stats/dashboard` |  |  /  | 0.003134 | 0.002695 / 0.003723 | N/A |
| `/api/v1/tasks?page=1&pageSize=20&sortBy=createdAt&sortOrder=desc` |  |  /  | 0.002787 | 0.002515 / 0.003346 | N/A |
| `/api/v1/projects?page=1&pageSize=20&sortBy=createdAt&sortOrder=desc` |  |  /  | 0.002641 | 0.002214 / 0.003033 | N/A |
