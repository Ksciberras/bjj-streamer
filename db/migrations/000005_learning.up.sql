CREATE TABLE playback_progress (
    user_id UUID NOT NULL REFERENCES users(id),
    video_id UUID NOT NULL REFERENCES videos(id),
    position_seconds DOUBLE PRECISION NOT NULL CHECK (position_seconds >= 0 AND position_seconds < 'Infinity'::DOUBLE PRECISION),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, video_id)
);

CREATE TABLE notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    video_id UUID NOT NULL REFERENCES videos(id),
    timestamp_seconds DOUBLE PRECISION NOT NULL CHECK (timestamp_seconds >= 0 AND timestamp_seconds < 'Infinity'::DOUBLE PRECISION),
    body TEXT NOT NULL CHECK (char_length(btrim(body)) BETWEEN 1 AND 5000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX notes_user_video_timestamp_idx ON notes (user_id, video_id, timestamp_seconds, created_at);
