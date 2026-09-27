DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM rules
        WHERE once_per_utc_day
    ) THEN
        RAISE EXCEPTION
            'cannot roll back migration 000009 while daily rules exist; use a forward-fix or restore a pre-daily-rule backup';
    END IF;
END
$$;

DROP TABLE IF EXISTS rule_daily_claims;

ALTER TABLE rules
    DROP CONSTRAINT IF EXISTS chk_rules_daily_not_aggregate;

ALTER TABLE rules
    DROP COLUMN IF EXISTS once_per_utc_day;
