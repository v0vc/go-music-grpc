CREATE TABLE site (
    site_id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    login TEXT,
    pass TEXT,
    token TEXT
);
CREATE TABLE artist (
    art_id INTEGER PRIMARY KEY AUTOINCREMENT,
    siteId INTEGER REFERENCES site (site_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    artistId TEXT NOT NULL,
    title TEXT,
    userAdded INTEGER default 0,
    syncState INTEGER default 1,
    thumbnail BLOB,
    UNIQUE(siteId,artistId)
);
CREATE TABLE album (
    alb_id INTEGER PRIMARY KEY AUTOINCREMENT,
    albumId TEXT,
    title TEXT,
    releaseDate TEXT,
    releaseType TEXT,
    syncState INTEGER default 0,
    thumbnail BLOB,
    UNIQUE(albumId,title)
);
CREATE TABLE artistAlbum (
    artistId INTEGER REFERENCES artist (art_id) ON UPDATE CASCADE ON DELETE CASCADE,
    albumId INTEGER REFERENCES album (alb_id) ON UPDATE CASCADE ON DELETE CASCADE,
    UNIQUE(artistId,albumId)
);
CREATE TABLE track (
    trk_id INTEGER PRIMARY KEY AUTOINCREMENT,
    albumId INTEGER REFERENCES album (alb_id) ON UPDATE CASCADE ON DELETE CASCADE,
    trackId TEXT NOT NULL,
    title TEXT NOT NULL,
    duration INTEGER default 0,
    UNIQUE(albumId,trackId)
);