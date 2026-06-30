CREATE TABLE IF NOT EXISTS saved_reports (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(150) NOT NULL,
  description TEXT NULL,
  type VARCHAR(40) NOT NULL,
  filters JSON NULL,
  chart_config JSON NULL,
  created_by_id BIGINT UNSIGNED NOT NULL,
  INDEX idx_saved_reports_name (name),
  INDEX idx_saved_reports_type (type),
  INDEX idx_saved_reports_created_by_id (created_by_id),
  CONSTRAINT fk_saved_reports_created_by FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE CASCADE
);
