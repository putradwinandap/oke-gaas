ALTER TABLE rules
    ADD COLUMN match_every bigint NOT NULL DEFAULT 1;

ALTER TABLE rules
    ADD CONSTRAINT chk_rules_match_every_positive
    CHECK (match_every > 0);

CREATE TABLE rule_match_counts (
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    rule_id varchar(64) NOT NULL,
    rule_version bigint NOT NULL CHECK (rule_version > 0),
    match_count bigint NOT NULL DEFAULT 0 CHECK (match_count >= 0),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, player_id, rule_id, rule_version),
    CONSTRAINT fk_rule_match_counts_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_rule_match_counts_rule_version
        FOREIGN KEY (project_id, rule_id, rule_version)
        REFERENCES rules(project_id, id, version)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);
