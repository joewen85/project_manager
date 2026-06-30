CREATE TABLE IF NOT EXISTS portal_invites (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(150) NOT NULL,
  company VARCHAR(150) NULL,
  contact_name VARCHAR(120) NULL,
  contact_email VARCHAR(160) NULL,
  contact_type VARCHAR(40) DEFAULT 'customer',
  token_prefix VARCHAR(32) NOT NULL,
  token_last_four VARCHAR(8) NOT NULL,
  token_hash VARCHAR(128) NOT NULL,
  is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  expires_at DATETIME(3) NULL,
  revoked_at DATETIME(3) NULL,
  last_used_at DATETIME(3) NULL,
  last_used_ip VARCHAR(50) NULL,
  allowed_attachments JSON NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  created_by_id BIGINT UNSIGNED NOT NULL,
  UNIQUE INDEX idx_portal_invites_token_hash (token_hash),
  INDEX idx_portal_invites_name (name),
  INDEX idx_portal_invites_contact_type (contact_type),
  INDEX idx_portal_invites_token_prefix (token_prefix),
  INDEX idx_portal_invites_is_enabled (is_enabled),
  INDEX idx_portal_invites_revoked_at (revoked_at),
  INDEX idx_portal_invites_project_id (project_id),
  INDEX idx_portal_invites_created_by_id (created_by_id),
  CONSTRAINT fk_portal_invites_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
  CONSTRAINT fk_portal_invites_created_by FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE RESTRICT
);

ALTER TABLE tasks
  ADD COLUMN external_visible BOOLEAN NOT NULL DEFAULT FALSE,
  ADD INDEX idx_tasks_external_visible (external_visible);

ALTER TABLE task_comments
  ADD COLUMN source VARCHAR(30) NOT NULL DEFAULT 'internal',
  ADD COLUMN portal_invite_id BIGINT UNSIGNED NULL,
  ADD COLUMN external_name VARCHAR(120) NULL,
  ADD COLUMN external_email VARCHAR(160) NULL,
  ADD COLUMN external_company VARCHAR(150) NULL,
  ADD INDEX idx_task_comments_source (source),
  ADD INDEX idx_task_comments_portal_invite_id (portal_invite_id),
  ADD CONSTRAINT fk_task_comments_portal_invite FOREIGN KEY (portal_invite_id) REFERENCES portal_invites(id) ON DELETE SET NULL;

ALTER TABLE work_requests
  ADD COLUMN attachments JSON NULL,
  ADD COLUMN source VARCHAR(30) NOT NULL DEFAULT 'internal',
  ADD COLUMN portal_invite_id BIGINT UNSIGNED NULL,
  ADD COLUMN external_name VARCHAR(120) NULL,
  ADD COLUMN external_email VARCHAR(160) NULL,
  ADD COLUMN external_company VARCHAR(150) NULL,
  ADD INDEX idx_work_requests_source (source),
  ADD INDEX idx_work_requests_portal_invite_id (portal_invite_id),
  ADD CONSTRAINT fk_work_requests_portal_invite FOREIGN KEY (portal_invite_id) REFERENCES portal_invites(id) ON DELETE SET NULL;
