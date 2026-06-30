CREATE TABLE IF NOT EXISTS project_templates (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(150) NOT NULL,
  description TEXT NULL,
  task_tree JSON NULL,
  UNIQUE INDEX idx_project_templates_name (name)
);
