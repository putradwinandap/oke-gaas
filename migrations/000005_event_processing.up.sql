CREATE TABLE event_processing (
    project_id varchar(64) NOT NULL,
    event_id varchar(255) NOT NULL,
    processed_at timestamptz NOT NULL,
    CONSTRAINT event_processing_pkey PRIMARY KEY (project_id, event_id),
    CONSTRAINT fk_event_processing_event
        FOREIGN KEY (project_id, event_id)
        REFERENCES events(project_id, id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

-- Events that predate transactional progression are considered settled.
-- This prevents deployment from retroactively applying today's rules to historical events.
INSERT INTO event_processing (project_id, event_id, processed_at)
SELECT project_id, id, received_at
FROM events
ON CONFLICT (project_id, event_id) DO NOTHING;
