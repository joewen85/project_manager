ALTER TABLE projects
  ADD COLUMN budget_amount DECIMAL(14,2) NOT NULL DEFAULT 0,
  ADD COLUMN actual_cost_amount DECIMAL(14,2) NOT NULL DEFAULT 0,
  ADD COLUMN expected_revenue_amount DECIMAL(14,2) NOT NULL DEFAULT 0,
  ADD COLUMN contract_no VARCHAR(120),
  ADD COLUMN contract_attachments JSON NULL,
  ADD INDEX idx_projects_contract_no (contract_no);
