DROP TABLE IF EXISTS rule_match_counts;

ALTER TABLE rules
    DROP CONSTRAINT IF EXISTS chk_rules_match_every_positive;

ALTER TABLE rules
    DROP COLUMN IF EXISTS match_every;
