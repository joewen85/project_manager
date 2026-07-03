CREATE TABLE IF NOT EXISTS automation_rules (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(150) NOT NULL,
  `trigger` VARCHAR(50) DEFAULT 'task_overdue',
  is_enabled BOOLEAN DEFAULT TRUE,
  conditions JSON NULL,
  actions JSON NULL,
  last_run_at DATETIME(3) NULL,
  created_by_id BIGINT UNSIGNED NULL,
  INDEX idx_automation_rules_trigger (`trigger`),
  INDEX idx_automation_rules_is_enabled (is_enabled),
  INDEX idx_automation_rules_created_by_id (created_by_id),
  CONSTRAINT fk_automation_rules_created_by FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS automation_execution_logs (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  rule_id BIGINT UNSIGNED NOT NULL,
  `trigger` VARCHAR(50),
  status VARCHAR(20),
  matched_count BIGINT DEFAULT 0,
  action_count BIGINT DEFAULT 0,
  message TEXT NULL,
  actor_id BIGINT UNSIGNED DEFAULT 0,
  run_source VARCHAR(20) DEFAULT 'manual',
  INDEX idx_automation_execution_logs_rule_id (rule_id),
  INDEX idx_automation_execution_logs_trigger (`trigger`),
  INDEX idx_automation_execution_logs_status (status),
  INDEX idx_automation_execution_logs_actor_id (actor_id),
  INDEX idx_automation_execution_logs_run_source (run_source),
  CONSTRAINT fk_automation_execution_logs_rule FOREIGN KEY (rule_id) REFERENCES automation_rules(id) ON DELETE CASCADE
);
