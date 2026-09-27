ALTER TABLE rules
    ADD COLUMN conditions jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE rules
    ADD CONSTRAINT chk_rules_conditions_object
    CHECK (jsonb_typeof(conditions) = 'object');
