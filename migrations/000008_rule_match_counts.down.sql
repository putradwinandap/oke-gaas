DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM rules
        WHERE match_every > 1
    ) THEN
        RAISE EXCEPTION
            'cannot roll back migration 000008 while aggregate rules exist (match_every > 1); use a forward-fix or restore a pre-aggregate backup';
    END IF;
END
$$;

DROP TABLE IF EXISTS rule_match_counts;

ALTER TABLE rules
    DROP CONSTRAINT IF EXISTS chk_rules_match_every_positive;

ALTER TABLE rules
    DROP COLUMN IF EXISTS match_every;
