
-- +migrate Up

CREATE UNIQUE INDEX track_unique ON video_source_track(video_source_id, stream_id);

-- +migrate Down
