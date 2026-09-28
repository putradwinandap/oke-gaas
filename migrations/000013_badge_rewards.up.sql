CREATE TABLE badge_definitions (
    project_id varchar(64) NOT NULL,
    id varchar(64) NOT NULL,
    name varchar(255) NOT NULL,
    description varchar(1024) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, id),
    CONSTRAINT fk_badge_definitions_project FOREIGN KEY (project_id)
        REFERENCES projects(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_badges_project_name ON badge_definitions(project_id, name);

ALTER TABLE rules ADD COLUMN reward_type varchar(32) NOT NULL DEFAULT 'xp';
ALTER TABLE rules ADD COLUMN badge_id varchar(64);
ALTER TABLE rules DROP CONSTRAINT rules_xp_amount_check;
ALTER TABLE rules ADD CONSTRAINT chk_rules_reward_configuration CHECK (
    (reward_type = 'xp' AND xp_amount > 0 AND badge_id IS NULL)
    OR (reward_type = 'badge' AND xp_amount = 0 AND badge_id IS NOT NULL)
);
ALTER TABLE rules ADD CONSTRAINT fk_rules_badge_project FOREIGN KEY (project_id, badge_id)
    REFERENCES badge_definitions(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT;

ALTER TABLE reward_grants ADD COLUMN badge_id varchar(64);
ALTER TABLE reward_grants DROP CONSTRAINT reward_grants_amount_check;
ALTER TABLE reward_grants ADD CONSTRAINT chk_reward_grants_payload CHECK (
    (reward_type = 'xp' AND amount > 0 AND badge_id IS NULL)
    OR (reward_type = 'badge' AND amount = 0 AND badge_id IS NOT NULL)
);
ALTER TABLE reward_grants ADD CONSTRAINT fk_reward_grants_badge_project FOREIGN KEY (project_id, badge_id)
    REFERENCES badge_definitions(project_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT;
CREATE UNIQUE INDEX uq_reward_grants_player_badge ON reward_grants(project_id, player_id, badge_id)
    WHERE reward_type = 'badge';

-- Older binaries do not understand Badge rules. Fail closed after a Project
-- configures one, until a Badge-aware application binary processes Events.
CREATE FUNCTION require_badge_aware_event_processing() RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM rules WHERE project_id = NEW.project_id AND reward_type = 'badge')
       AND current_setting('oke_gaas.badge_aware', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'Project has Badge Rules and requires a Badge-aware application binary'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_event_processing_badge_compatibility
    BEFORE INSERT ON event_processing FOR EACH ROW
    EXECUTE FUNCTION require_badge_aware_event_processing();
