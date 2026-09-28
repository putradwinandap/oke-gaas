DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM reward_grants WHERE reward_type = 'badge')
       OR EXISTS (SELECT 1 FROM rules WHERE reward_type = 'badge')
       OR EXISTS (SELECT 1 FROM badge_definitions) THEN
        RAISE EXCEPTION 'cannot roll back 000013 while Badge definitions, Rules, or grants exist';
    END IF;
END
$$;
DROP TRIGGER IF EXISTS trg_event_processing_badge_compatibility ON event_processing;
DROP FUNCTION IF EXISTS require_badge_aware_event_processing();
ALTER TABLE reward_grants DROP CONSTRAINT fk_reward_grants_badge_project;
ALTER TABLE reward_grants DROP CONSTRAINT chk_reward_grants_payload;
DROP INDEX IF EXISTS uq_reward_grants_player_badge;
ALTER TABLE reward_grants DROP COLUMN badge_id;
ALTER TABLE reward_grants ADD CONSTRAINT reward_grants_amount_check CHECK (amount > 0);
ALTER TABLE rules DROP CONSTRAINT fk_rules_badge_project;
ALTER TABLE rules DROP CONSTRAINT chk_rules_reward_configuration;
ALTER TABLE rules DROP COLUMN badge_id;
ALTER TABLE rules DROP COLUMN reward_type;
ALTER TABLE rules ADD CONSTRAINT rules_xp_amount_check CHECK (xp_amount > 0);
DROP TABLE badge_definitions;
