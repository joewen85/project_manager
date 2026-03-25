# Changelog

All notable changes to this project are documented in this file.

## [v1.0.0] - 2026-03-25

### Added
- Complete project management baseline with `Go (Gin + Gorm + JWT)`, `MySQL`, and `Vite + React + TypeScript`.
- RBAC, user management, department management, project management, task management, personal workspace, dashboard statistics, audit log, gantt chart, and task tree pages.
- Environment template support via `.env.template`.
- UI support modules and reusable components (`Pagination`, `DataState`, status constants, UI docs/checklists).
- Deployment artifacts for Docker Compose and Helm/K8s.

### Changed
- Improved permission scope behavior for project/task visibility (user-scoped results for non-admin users).
- Enhanced dashboard behavior to hide user count metrics for users without user-management privileges.
- Updated project/task edit dialogs to use searchable selector areas.
- Improved gantt chart visual contrast and progress readability.

### Fixed
- Fixed CORS issues in local development.
- Fixed permission refresh timing issues after RBAC updates.
- Fixed selector interaction issues where multi-select required `command + click` to unselect (replaced with checkbox-based selection UX).
- Fixed task/project data-loading edge cases causing empty or unstable option lists in edit dialogs.
- Fixed chart rendering null-safety issues in dashboard/gantt components.
