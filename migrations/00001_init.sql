-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE IF NOT EXISTS video_source (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT,
  provider    VARCHAR(64) NULL,
  username    VARCHAR(64) NULL,
  password    VARCHAR(64) NULL,
  base_url    TEXT,
  m3u_url     TEXT,
  max_streams INTEGER,
  imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS video_source_track (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  video_source_id INTEGER,
  name            TEXT,
  stream_id       INTEGER,
  logo            TEXT,
  type            TEXT,
  category        TEXT,
  epg_id          TEXT,
  imported_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(video_source_id) REFERENCES video_source(id)
);

CREATE TABLE IF NOT EXISTS guide_source (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  name           TEXT,
  provider       VARCHAR(64) NULL,
  username       VARCHAR(64) NULL,
  password       VARCHAR(64) NULL,
  xmltv_url      TEXT,
  provider_data  TEXT,
  imported_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS guide_source_channel (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  guide_id        INTEGER,
  xmltv_id        TEXT,
  provider_data   TEXT,
  data            TEXT,
  imported_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT channel_unique UNIQUE (guide_id, xmltv_id),
  FOREIGN KEY(guide_id) REFERENCES guide_source(id)
);

CREATE TABLE IF NOT EXISTS guide_source_programme (
  guide_id        INT,
  channel         TEXT,
  start           TIMESTAMP,
  end             TIMESTAMP,
  date            DATE,
  provider_data   TEXT,
  data            TEXT,
  imported_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT programme_unique UNIQUE (guide_id, channel, start, end),
  FOREIGN KEY(guide_id) REFERENCES guide_source(id)
);

CREATE TABLE IF NOT EXISTS lineup (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  name                TEXT,
  ssdp                BOOLEAN DEFAULT TRUE,
  listen_address      TEXT DEFAULT '127.0.0.1',
  discovery_address   TEXT DEFAULT '127.0.0.1',
  port                INTEGER,
  tuners              INTEGER,
  manufacturer        TEXT DEFAULT 'Silicondust',
  model_name          TEXT DEFAULT 'HDHomeRun EXTEND',
  model_number        TEXT DEFAULT 'HDTC-2US',
  firmware_name       TEXT DEFAULT 'hdhomeruntc_atsc',
  firmware_version    TEXT DEFAULT '20150826',
  device_id           TEXT DEFAULT '12345678',
  device_auth         TEXT DEFAULT 'telly',
  device_uuid         TEXT DEFAULT '12345678-AE2A-4E54-BBC9-33AF7D5D6A92',
  created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS lineup_channel (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  lineup_id        INTEGER,
  title            TEXT,
  channel_number   TEXT,
  video_track_id   INTEGER,
  guide_channel_id INTEGER,
  hd               BOOLEAN,
  favorite         BOOLEAN,
  created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(lineup_id) REFERENCES lineup(id),
  FOREIGN KEY(video_track_id) REFERENCES video_source_track(id),
  FOREIGN KEY(guide_channel_id) REFERENCES guide_source_channel(id)
);


-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE video_source;
DROP TABLE video_source_track;
DROP TABLE guide_source;
DROP TABLE guide_source_channel;
DROP TABLE lineup;
DROP TABLE lineup_channel;
