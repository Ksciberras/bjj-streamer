CREATE TABLE learning_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('started', 'resumed', 'completed')),
    position_seconds DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (position_seconds >= 0 AND position_seconds < 'Infinity'::DOUBLE PRECISION),
    occurred_on DATE NOT NULL DEFAULT CURRENT_DATE,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, video_id, event_type, occurred_on)
);

CREATE INDEX learning_events_organization_time_idx
    ON learning_events (organization_id, occurred_at DESC);
CREATE INDEX learning_events_video_time_idx
    ON learning_events (video_id, occurred_at DESC);
