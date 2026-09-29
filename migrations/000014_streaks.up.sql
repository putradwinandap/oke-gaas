CREATE TABLE streak_definitions (
    project_id varchar(64) NOT NULL,
    id varchar(64) NOT NULL,
    name varchar(255) NOT NULL,
    event_type varchar(255) NOT NULL,
    conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, id),
    CONSTRAINT fk_streak_definitions_project FOREIGN KEY (project_id)
        REFERENCES projects(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_streaks_project_name ON streak_definitions(project_id, name);
CREATE INDEX idx_streak_definitions_project_event_type ON streak_definitions(project_id, event_type, id);

CREATE TABLE streak_days (
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    streak_id varchar(64) NOT NULL,
    qualified_day date NOT NULL,
    PRIMARY KEY (project_id, player_id, streak_id, qualified_day),
    CONSTRAINT fk_streak_days_player_project FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_streak_days_definition FOREIGN KEY (project_id, streak_id)
        REFERENCES streak_definitions(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
CREATE INDEX idx_streak_days_player_history ON streak_days(project_id, player_id, streak_id, qualified_day DESC);

CREATE TABLE streak_event_claims (
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    streak_id varchar(64) NOT NULL,
    event_id varchar(255) NOT NULL,
    qualified_day date NOT NULL,
    qualified_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, event_id, streak_id),
    CONSTRAINT fk_streak_event_claims_player_project FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_streak_event_claims_definition FOREIGN KEY (project_id, streak_id)
        REFERENCES streak_definitions(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_streak_event_claims_event FOREIGN KEY (project_id, event_id)
        REFERENCES events(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
CREATE INDEX idx_streak_event_claims_day ON streak_event_claims(project_id, player_id, streak_id, qualified_day);

CREATE OR REPLACE FUNCTION require_streak_aware_event_processing() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM streak_definitions WHERE project_id = NEW.project_id)
       AND current_setting('oke_gaas.streak_aware', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'Project has Streak definitions and requires a Streak-aware application binary'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_event_processing_streak_compatibility
    BEFORE INSERT ON event_processing FOR EACH ROW
    EXECUTE FUNCTION require_streak_aware_event_processing();
