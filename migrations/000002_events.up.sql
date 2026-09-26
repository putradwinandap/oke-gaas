CREATE UNIQUE INDEX idx_players_project_id_unique
    ON players(project_id, id);

CREATE TABLE events (
    project_id varchar(64) NOT NULL,
    id varchar(255) NOT NULL,
    player_id varchar(64) NOT NULL,
    event_type varchar(255) NOT NULL,
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL,
    properties jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT events_pkey PRIMARY KEY (project_id, id),
    CONSTRAINT fk_events_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_events_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX idx_events_player_id
    ON events(player_id);

CREATE INDEX idx_events_event_type
    ON events(event_type);

CREATE INDEX idx_events_project_received_at
    ON events(project_id, received_at);
