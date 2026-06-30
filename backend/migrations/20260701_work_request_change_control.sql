ALTER TABLE work_requests
  ADD COLUMN target_task_id BIGINT UNSIGNED NULL,
  ADD COLUMN change_payload JSON NULL,
  ADD COLUMN applied_at DATETIME(3) NULL,
  ADD COLUMN applied_by_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_work_requests_target_task_id (target_task_id),
  ADD INDEX idx_work_requests_applied_at (applied_at),
  ADD INDEX idx_work_requests_applied_by_id (applied_by_id),
  ADD CONSTRAINT fk_work_requests_target_task FOREIGN KEY (target_task_id) REFERENCES tasks(id) ON DELETE SET NULL,
  ADD CONSTRAINT fk_work_requests_applied_by FOREIGN KEY (applied_by_id) REFERENCES users(id) ON DELETE SET NULL;
