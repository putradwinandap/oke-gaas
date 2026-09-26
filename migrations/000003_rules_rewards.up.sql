CREATE TABLE rules (
    project_id varchar(64) NOT NULL,
    id varchar(64) NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    event_type varchar(255) NOT NULL,
    xp_amount bigint NOT NULL CHECK (xp_amount > 0),
    PRIMARY KEY (project_id, id, version),
    CONSTRAINT fk_rules_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX idx_rules_project_event_type
    ON rules(project_id, event_type);

CREATE TABLE reward_grants (
    id varchar(64) PRIMARY KEY,
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    event_id varchar(255) NOT NULL,
    rule_id varchar(64) NOT NULL,
    rule_version bigint NOT NULL CHECK (rule_version > 0),
    reward_type varchar(32) NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    created_at timestamptz NOT NULL,
    CONSTRAINT fk_reward_grants_event
        FOREIGN KEY (project_id, event_id)
        REFERENCES events(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_reward_grants_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_reward_grants_rule_version
        FOREIGN KEY (project_id, rule_id, rule_version)
        REFERENCES rules(project_id, id, version)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX idx_reward_grants_event_rule_type
    ON reward_grants(project_id, event_id, rule_id, reward_type);

CREATE INDEX idx_reward_grants_project_player
    ON reward_grants(project_id, player_id);
