CREATE TABLE IF NOT EXISTS task_comments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  task_id BIGINT UNSIGNED NOT NULL,
  author_id BIGINT UNSIGNED NOT NULL,
  content TEXT NOT NULL,
  attachments JSON NULL,
  is_deleted BOOLEAN DEFAULT FALSE,
  INDEX idx_task_comments_task_id (task_id),
  INDEX idx_task_comments_author_id (author_id),
  INDEX idx_task_comment_deleted_created (task_id, is_deleted, created_at),
  CONSTRAINT fk_task_comments_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_comments_author FOREIGN KEY (author_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS task_comment_mentions (
  task_comment_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (task_comment_id, user_id),
  INDEX idx_task_comment_mentions_user_id (user_id),
  CONSTRAINT fk_task_comment_mentions_comment FOREIGN KEY (task_comment_id) REFERENCES task_comments(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_comment_mentions_user FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS task_activities (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  task_id BIGINT UNSIGNED NOT NULL,
  actor_id BIGINT UNSIGNED NULL,
  type VARCHAR(50) NOT NULL,
  summary VARCHAR(255) NOT NULL,
  detail TEXT NULL,
  comment_id BIGINT UNSIGNED NULL,
  INDEX idx_task_activities_task_id (task_id),
  INDEX idx_task_activities_actor_id (actor_id),
  INDEX idx_task_activities_type (type),
  INDEX idx_task_activities_comment_id (comment_id),
  CONSTRAINT fk_task_activities_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_activities_actor FOREIGN KEY (actor_id) REFERENCES users(id),
  CONSTRAINT fk_task_activities_comment FOREIGN KEY (comment_id) REFERENCES task_comments(id) ON DELETE SET NULL
);
