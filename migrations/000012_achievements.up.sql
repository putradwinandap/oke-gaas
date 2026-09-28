CREATE TABLE achievement_definitions (
    project_id varchar(64) NOT NULL,
    id varchar(64) NOT NULL,
    name varchar(255) NOT NULL,
    counter_id varchar(64) NOT NULL,
    target bigint NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, id),
    CONSTRAINT chk_achievements_target_positive CHECK (target > 0),
    CONSTRAINT fk_achievement_definitions_counter
        FOREIGN KEY (project_id, counter_id)
        REFERENCES counter_definitions(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX uq_achievements_project_name
    ON achievement_definitions(project_id, name);

CREATE TABLE achievement_unlocks (
    id varchar(64) PRIMARY KEY,
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    achievement_id varchar(64) NOT NULL,
    event_id varchar(255) NOT NULL,
    counter_value bigint NOT NULL,
    unlocked_at timestamptz NOT NULL,
    CONSTRAINT uq_achievement_unlock_player UNIQUE (project_id, player_id, achievement_id),
    CONSTRAINT chk_achievement_unlock_counter_value_positive CHECK (counter_value > 0),
    CONSTRAINT fk_achievement_unlocks_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_achievement_unlocks_definition
        FOREIGN KEY (project_id, achievement_id)
        REFERENCES achievement_definitions(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_achievement_unlocks_event
        FOREIGN KEY (project_id, event_id)
        REFERENCES events(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX idx_achievement_unlocks_player
    ON achievement_unlocks(project_id, player_id, unlocked_at, achievement_id);

CREATE OR REPLACE FUNCTION require_counter_aware_event_processing() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM counter_definitions WHERE project_id = NEW.project_id)
       AND current_setting('oke_gaas.counter_aware', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'Project has Counters and requires a Counter-aware application binary'
            USING ERRCODE = '55000';
    END IF;
    IF EXISTS (SELECT 1 FROM achievement_definitions WHERE project_id = NEW.project_id)
       AND current_setting('oke_gaas.achievement_aware', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'Project has Achievements and requires an Achievement-aware application binary'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
