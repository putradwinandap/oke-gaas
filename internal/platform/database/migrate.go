package database

import (
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrateCoreForDevelopment creates or updates the current core schema for
// local development and tests only. Production schema evolution must use the
// versioned SQL migrations under /migrations.
func AutoMigrateCoreForDevelopment(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&projectRecord{},
		&playerRecord{},
		&eventRecord{},
		&ruleRecord{},
		&rewardGrantRecord{},
		&playerStateRecord{},
		&eventProcessingRecord{},
	); err != nil {
		return fmt.Errorf("auto-migrate core development schema: %w", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_players_project_id_unique
		ON players(project_id, id)
	`).Error; err != nil {
		return fmt.Errorf("create development player project identity index: %w", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_reward_grants_event_rule_type
		ON reward_grants(project_id, event_id, rule_id, reward_type)
	`).Error; err != nil {
		return fmt.Errorf("create development reward grant idempotency index: %w", err)
	}

	constraints := []struct {
		table     string
		name      string
		statement string
	}{
		{
			table: "events",
			name:  "fk_events_project",
			statement: `
				ALTER TABLE events
				ADD CONSTRAINT fk_events_project
				FOREIGN KEY (project_id)
				REFERENCES projects(id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "events",
			name:  "fk_events_player_project",
			statement: `
				ALTER TABLE events
				ADD CONSTRAINT fk_events_player_project
				FOREIGN KEY (project_id, player_id)
				REFERENCES players(project_id, id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "rules",
			name:  "fk_rules_project",
			statement: `
				ALTER TABLE rules
				ADD CONSTRAINT fk_rules_project
				FOREIGN KEY (project_id)
				REFERENCES projects(id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "reward_grants",
			name:  "fk_reward_grants_event",
			statement: `
				ALTER TABLE reward_grants
				ADD CONSTRAINT fk_reward_grants_event
				FOREIGN KEY (project_id, event_id)
				REFERENCES events(project_id, id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "reward_grants",
			name:  "fk_reward_grants_player_project",
			statement: `
				ALTER TABLE reward_grants
				ADD CONSTRAINT fk_reward_grants_player_project
				FOREIGN KEY (project_id, player_id)
				REFERENCES players(project_id, id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "player_states",
			name:  "fk_player_states_player_project",
			statement: `
				ALTER TABLE player_states
				ADD CONSTRAINT fk_player_states_player_project
				FOREIGN KEY (project_id, player_id)
				REFERENCES players(project_id, id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "event_processing",
			name:  "fk_event_processing_event",
			statement: `
				ALTER TABLE event_processing
				ADD CONSTRAINT fk_event_processing_event
				FOREIGN KEY (project_id, event_id)
				REFERENCES events(project_id, id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
		{
			table: "reward_grants",
			name:  "fk_reward_grants_rule_version",
			statement: `
				ALTER TABLE reward_grants
				ADD CONSTRAINT fk_reward_grants_rule_version
				FOREIGN KEY (project_id, rule_id, rule_version)
				REFERENCES rules(project_id, id, version)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT
			`,
		},
	}

	if err := db.Exec(`
		INSERT INTO player_states (project_id, player_id, xp, updated_at)
		SELECT
			p.project_id,
			p.id,
			COALESCE(SUM(r.amount), 0),
			COALESCE(MAX(r.created_at), p.created_at)
		FROM players p
		LEFT JOIN reward_grants r
			ON r.project_id = p.project_id
		   AND r.player_id = p.id
		   AND r.reward_type = 'xp'
		GROUP BY p.project_id, p.id, p.created_at
		ON CONFLICT (project_id, player_id) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("backfill development player state: %w", err)
	}

	if err := db.Exec(`
		INSERT INTO event_processing (project_id, event_id, processed_at)
		SELECT project_id, id, received_at
		FROM events
		ON CONFLICT (project_id, event_id) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("backfill development event processing: %w", err)
	}

	for _, constraint := range constraints {
		if err := ensureDevelopmentForeignKey(
			db,
			constraint.table,
			constraint.name,
			constraint.statement,
		); err != nil {
			return fmt.Errorf("create development %s constraint: %w", constraint.name, err)
		}
	}
	return nil
}

func ensureDevelopmentForeignKey(db *gorm.DB, tableName, constraintName, statement string) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = ?
			  AND conrelid = ?::regclass
			  AND contype = 'f'
		)
	`, constraintName, tableName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check constraint %s on %s: %w", constraintName, tableName, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("add constraint %s on %s: %w", constraintName, tableName, err)
	}
	return nil
}
