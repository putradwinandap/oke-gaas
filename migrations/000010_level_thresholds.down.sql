DO $$
BEGIN
    IF to_regclass('level_thresholds') IS NOT NULL
       AND EXISTS (SELECT 1 FROM level_thresholds) THEN
        RAISE EXCEPTION 'cannot roll back 000010 while XP level thresholds are configured';
    END IF;
END
$$;

DROP TABLE IF EXISTS level_thresholds;
