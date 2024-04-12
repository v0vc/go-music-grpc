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
    releaseType INTEGER default 0,
    trackTotal INTEGER default 0,
    syncState INTEGER default 0,
    thumbnail BLOB,
    UNIQUE(albumId,title)
);
CREATE TABLE artistAlbum (
    artistId INTEGER REFERENCES artist (art_id) ON DELETE CASCADE,
    albumId INTEGER REFERENCES album (alb_id) ON DELETE CASCADE,
    UNIQUE(artistId,albumId)
);
CREATE TABLE track (
    trk_id INTEGER PRIMARY KEY AUTOINCREMENT,
    trackId TEXT NOT NULL,
    title TEXT NOT NULL,
    hasFlac INTEGER,
    hasLyric INTEGER,
    quality TEXT NULL,
    condition TEXT NULL,
    genre TEXT NULL,
    trackNum INTEGER,
    duration INTEGER,
    UNIQUE(trackId,title)
);
CREATE TABLE albumTrack (
    albumId INTEGER REFERENCES album (alb_id) ON UPDATE CASCADE ON DELETE CASCADE,
    trackId INTEGER REFERENCES track (trk_id) ON UPDATE CASCADE ON DELETE CASCADE,
    UNIQUE(albumId,trackId)
);
CREATE TABLE trackArtist (
    trackId INTEGER REFERENCES track (trk_id) ON UPDATE CASCADE ON DELETE CASCADE,
    artistId INTEGER REFERENCES artist (art_id) ON UPDATE CASCADE ON DELETE CASCADE,
    UNIQUE(trackId, artistId)
);