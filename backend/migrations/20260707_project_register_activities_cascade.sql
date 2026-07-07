SET @project_register_activity_fk := (
  SELECT CONSTRAINT_NAME
  FROM information_schema.KEY_COLUMN_USAGE
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'project_register_activities'
    AND COLUMN_NAME = 'register_id'
    AND REFERENCED_TABLE_NAME = 'project_registers'
  LIMIT 1
);

SET @drop_project_register_activity_fk := IF(
  @project_register_activity_fk IS NULL,
  'SELECT 1',
  CONCAT('ALTER TABLE project_register_activities DROP FOREIGN KEY `', REPLACE(@project_register_activity_fk, '`', '``'), '`')
);

PREPARE drop_project_register_activity_fk_stmt FROM @drop_project_register_activity_fk;
EXECUTE drop_project_register_activity_fk_stmt;
DEALLOCATE PREPARE drop_project_register_activity_fk_stmt;

ALTER TABLE project_register_activities
  ADD CONSTRAINT fk_project_register_activities_register
  FOREIGN KEY (register_id) REFERENCES project_registers(id) ON DELETE CASCADE;
