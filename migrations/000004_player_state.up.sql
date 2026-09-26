CREATE TABLE player_states (
    project_id varchar(64) NOT NULL,
    player_id varchar(64) NOT NULL,
    xp bigint NOT NULL DEFAULT 0 CHECK (xp >= 0),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, player_id),
    CONSTRAINT fk_player_states_player_project
        FOREIGN KEY (project_id, player_id)
        REFERENCES players(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX idx_player_states_project_xp
    ON player_states(project_id, xp DESC, player_id);

INSERT INTO player_states (project_id, player_id, xp, updated_at)
SELECT
    project_id,
    player_id,
    SUM(amount) AS xp,
    MAX(created_at) AS updated_at
FROM reward_grants
WHERE reward_type = 'xp'
GROUP BY project_id, player_id;
