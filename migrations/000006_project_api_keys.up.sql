CREATE TABLE project_api_keys (
    project_id VARCHAR(64) PRIMARY KEY,
    secret_hash BYTEA NOT NULL,
    CONSTRAINT fk_project_api_keys_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE
);
