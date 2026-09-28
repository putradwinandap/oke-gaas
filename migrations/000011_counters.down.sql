DO $$
BEGIN
    IF to_regclass('counter_definitions') IS NOT NULL
       AND EXISTS (SELECT 1 FROM counter_definitions) THEN
        RAISE EXCEPTION 'cannot roll back 000011 while Counter definitions are configured';
    END IF;
END
$$;

DROP TRIGGER IF EXISTS trg_event_processing_counter_compatibility ON event_processing;
DROP FUNCTION IF EXISTS require_counter_aware_event_processing();
DROP TABLE IF EXISTS player_counters;
DROP TABLE IF EXISTS counter_definitions;
