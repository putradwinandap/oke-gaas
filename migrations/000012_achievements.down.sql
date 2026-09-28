DO $$
BEGIN
    IF to_regclass('achievement_definitions') IS NOT NULL THEN
        IF EXISTS (SELECT 1 FROM achievement_definitions) THEN
            RAISE EXCEPTION 'cannot roll back 000012 while Achievement definitions are configured';
        END IF;
    END IF;
END
$$;

DROP TABLE IF EXISTS achievement_unlocks;
DROP TABLE IF EXISTS achievement_definitions;

-- Restore the Counter-only compatibility guard from migration 000011.
CREATE OR REPLACE FUNCTION require_counter_aware_event_processing() RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM counter_definitions WHERE project_id = NEW.project_id
    ) AND current_setting('oke_gaas.counter_aware', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'Project has Counters and requires a Counter-aware application binary'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
