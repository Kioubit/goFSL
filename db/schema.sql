CREATE TABLE IF NOT EXISTS 'files'
(
    ID                 INTEGER PRIMARY KEY AUTOINCREMENT,
    Expiry             INTEGER NOT NULL,
    DownloadsRemaining INTEGER NOT NULL,
    MaxChunkSize       INTEGER NOT NULL,
    DownloadSize       INTEGER NOT NULL,
    UserMetaData       TEXT    NOT NULL,
    DeletionToken      TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS 'KV'
(
    Key   TEXT PRIMARY KEY,
    Value TEXT
);