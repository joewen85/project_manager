CREATE TABLE IF NOT EXISTS webhook_subscriptions (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(150) NOT NULL,
  event VARCHAR(60) NOT NULL,
  url VARCHAR(600) NOT NULL,
  is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  last_delivery_status VARCHAR(20) NULL,
  last_delivered_at DATETIME(3) NULL,
  last_error TEXT NULL,
  created_by_id BIGINT UNSIGNED NOT NULL,
  INDEX idx_webhook_subscriptions_name (name),
  INDEX idx_webhook_subscriptions_event (event),
  INDEX idx_webhook_subscriptions_is_enabled (is_enabled),
  INDEX idx_webhook_subscriptions_created_by_id (created_by_id),
  CONSTRAINT fk_webhook_subscriptions_created_by FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  subscription_id BIGINT UNSIGNED NOT NULL,
  event VARCHAR(60) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  payload LONGTEXT NULL,
  response_status INT NOT NULL DEFAULT 0,
  error_message TEXT NULL,
  next_retry_at DATETIME(3) NULL,
  delivered_at DATETIME(3) NULL,
  INDEX idx_webhook_deliveries_subscription_id (subscription_id),
  INDEX idx_webhook_deliveries_event (event),
  INDEX idx_webhook_deliveries_status (status),
  CONSTRAINT fk_webhook_deliveries_subscription FOREIGN KEY (subscription_id) REFERENCES webhook_subscriptions(id) ON DELETE CASCADE
);
