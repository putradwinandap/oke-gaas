CREATE TABLE counter_definitions (
    project_id varchar(64) NOT NULL,
    id varchar(64) NOT NULL,
    name varchar(255) NOT NULL,
    event_type varchar(255) NOT NULL,
    conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, id),
    CONSTRAINT fk_counter_definitions_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX uq_counters_project_name
    ON counter_definitions(project_id, name);

CREATE INDEX idx_counter_definitions_project_event_type
    ON counter_definitions(project_id, event_type, id);

CREATE TABLE player_counters (
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    counter_id varchar(64) NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, player_id, counter_id),
    CONSTRAINT chk_player_counters_nonnegative
        CHECK (value >= 0),
    CONSTRAINT fk_player_counters_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_player_counters_definition
        FOREIGN KEY (project_id, counter_id)
        REFERENCES counter_definitions(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

-- Older binaries do not maintain Player Counter state. Permit them to process
-- Events until a Project configures a Counter, then fail closed unless the
-- transaction explicitly declares Counter-aware processing.
CREATE FUNCTION require_counter_aware_event_processing() RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM counter_definitions WHERE project_id = NEW.project_id
    ) AND current_setting('oke_gaas.counter_aware', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'Project has Counters and requires a Counter-aware application binary'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_event_processing_counter_compatibility
    BEFORE INSERT ON event_processing
    FOR EACH ROW
    EXECUTE FUNCTION require_counter_aware_event_processing();
