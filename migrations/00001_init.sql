-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE IF NOT EXISTS video_source (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT,
  provider    VARCHAR(64) NULL,
  username    VARCHAR(64) NULL,
  password    VARCHAR(64) NULL,
  m3u_url     TEXT,
  imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS video_source_track (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  video_source_id INTEGER,
  name            TEXT,
  tags            TEXT,
  raw_line        TEXT,
  stream_url      TEXT,
  hd              BOOLEAN,
  imported_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(video_source_id) REFERENCES video_source(id)
);

CREATE TABLE IF NOT EXISTS guide_source (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT,
  provider    VARCHAR(64) NULL,
  username    VARCHAR(64) NULL,
  password    VARCHAR(64) NULL,
  xmltv_url   TEXT,
  imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS guide_source_channel (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  guide_id        INTEGER,
  xmltv_id        TEXT,
  display_names   TEXT,
  urls            TEXT,
  icons           TEXT,
  channel_number  TEXT,
  hd              BOOLEAN,
  imported_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(guide_id) REFERENCES guide_source(id)
);

CREATE TABLE IF NOT EXISTS lineup (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  name           TEXT,
  created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS lineup_channel (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  title            TEXT,
  channel_number   TEXT,
  video_track_id   INTEGER,
  guide_channel_id TEXT,
  hd               BOOLEAN,
  favorite         BOOLEAN,
  created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE video_source;
DROP TABLE video_source_track;
DROP TABLE guide_source;
DROP TABLE guide_source_channel;
DROP TABLE lineup;
DROP TABLE lineup_channel;
