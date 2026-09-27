CREATE TABLE level_thresholds (
    project_id varchar(64) NOT NULL,
    level integer NOT NULL CHECK (level >= 2),
    min_xp bigint NOT NULL CHECK (min_xp > 0),
    PRIMARY KEY (project_id, level),
    CONSTRAINT fk_level_thresholds_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX idx_level_thresholds_project_min_xp
    ON level_thresholds(project_id, min_xp);

CREATE INDEX idx_level_thresholds_project_resolve
    ON level_thresholds(project_id, min_xp DESC);
