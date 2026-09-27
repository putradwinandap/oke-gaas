ALTER TABLE rules
    ADD COLUMN once_per_utc_day boolean NOT NULL DEFAULT false;

ALTER TABLE rules
    ADD CONSTRAINT chk_rules_daily_not_aggregate
    CHECK (NOT once_per_utc_day OR match_every = 1);

CREATE TABLE rule_daily_claims (
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    rule_id varchar(64) NOT NULL,
    rule_version bigint NOT NULL CHECK (rule_version > 0),
    claim_day date NOT NULL,
    claimed_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, player_id, rule_id, rule_version, claim_day),
    CONSTRAINT fk_rule_daily_claims_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT fk_rule_daily_claims_rule_version
        FOREIGN KEY (project_id, rule_id, rule_version)
        REFERENCES rules(project_id, id, version)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);
