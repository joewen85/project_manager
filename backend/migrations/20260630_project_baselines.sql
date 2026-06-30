CREATE TABLE IF NOT EXISTS project_baselines (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(150) NOT NULL,
  description TEXT NULL,
  task_count INT NOT NULL DEFAULT 0,
  completed_task_count INT NOT NULL DEFAULT 0,
  planned_start_at DATETIME(3) NULL,
  planned_end_at DATETIME(3) NULL,
  snapshot JSON NULL,
  created_by_id BIGINT UNSIGNED NOT NULL,
  INDEX idx_project_baselines_project_id (project_id),
  INDEX idx_project_baselines_name (name),
  INDEX idx_project_baselines_created_by_id (created_by_id),
  CONSTRAINT fk_project_baselines_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
  CONSTRAINT fk_project_baselines_created_by FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE RESTRICT
);
