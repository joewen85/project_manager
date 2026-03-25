# API Benchmark (after-frontend-loader)

- base_url: `http://localhost:8080`
- runs: `5`
- generated_at: `2026-03-25 17:05:59 +0800`

| Endpoint | avg (s) | min (s) | max (s) |
|---|---:|---:|---:|
| `/api/v1/stats/dashboard` | 0.003134 | 0.002695 | 0.003723 |
| `/api/v1/tasks?page=1&pageSize=20&sortBy=createdAt&sortOrder=desc` | 0.002787 | 0.002515 | 0.003346 |
| `/api/v1/projects?page=1&pageSize=20&sortBy=createdAt&sortOrder=desc` | 0.002641 | 0.002214 | 0.003033 |
