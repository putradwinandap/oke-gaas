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
    p.project_id,
    p.id,
    COALESCE(SUM(r.amount), 0) AS xp,
    COALESCE(MAX(r.created_at), p.created_at) AS updated_at
FROM players p
LEFT JOIN reward_grants r
    ON r.project_id = p.project_id
   AND r.player_id = p.id
   AND r.reward_type = 'xp'
GROUP BY p.project_id, p.id, p.created_at;
