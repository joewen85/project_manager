CREATE TABLE IF NOT EXISTS sprints (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(150) NOT NULL,
  goal TEXT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'planned',
  start_at DATETIME(3) NULL,
  end_at DATETIME(3) NULL,
  capacity_hours DECIMAL(10,2) NOT NULL DEFAULT 0,
  created_by_id BIGINT UNSIGNED NOT NULL,
  INDEX idx_sprints_name (name),
  INDEX idx_sprints_status (status),
  INDEX idx_sprints_created_by_id (created_by_id),
  CONSTRAINT fk_sprints_created_by FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sprint_tasks (
  sprint_id BIGINT UNSIGNED NOT NULL,
  task_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NULL,
  PRIMARY KEY (sprint_id, task_id),
  INDEX idx_sprint_tasks_sprint_id (sprint_id),
  INDEX idx_sprint_tasks_task_id (task_id),
  CONSTRAINT fk_sprint_tasks_sprint FOREIGN KEY (sprint_id) REFERENCES sprints(id) ON DELETE CASCADE,
  CONSTRAINT fk_sprint_tasks_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
