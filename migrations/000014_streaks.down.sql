DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM streak_days) OR EXISTS (SELECT 1 FROM streak_event_claims) OR EXISTS (SELECT 1 FROM streak_definitions) THEN
        RAISE EXCEPTION 'cannot roll back 000014 while Streak definitions or qualified days exist';
    END IF;
END
$$;
DROP TRIGGER IF EXISTS trg_event_processing_streak_compatibility ON event_processing;
DROP FUNCTION IF EXISTS require_streak_aware_event_processing();
DROP TABLE streak_days;
DROP TABLE streak_event_claims;
DROP TABLE streak_definitions;
