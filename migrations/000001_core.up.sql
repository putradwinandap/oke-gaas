CREATE TABLE projects (
    id varchar(64) PRIMARY KEY,
    name varchar(255) NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE players (
    id varchar(64) PRIMARY KEY,
    project_id varchar(64) NOT NULL,
    external_id varchar(255) NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT fk_players_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX idx_players_project_id
    ON players(project_id);

CREATE UNIQUE INDEX idx_players_project_external
    ON players(project_id, external_id);
