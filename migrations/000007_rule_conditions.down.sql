ALTER TABLE rules
    DROP CONSTRAINT IF EXISTS chk_rules_conditions_object;

ALTER TABLE rules
    DROP COLUMN IF EXISTS conditions;
