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
CREATE TABLE channel (
    ch_id INTEGER PRIMARY KEY AUTOINCREMENT,
    siteId INTEGER REFERENCES site (site_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    channelId TEXT NOT NULL,
    title TEXT NOT NULL,
    syncState INTEGER default 1,
    thumbnail BLOB,
    UNIQUE(siteId,channelId)
);
CREATE TABLE playlist (
    pl_id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlistId TEXT,
    title TEXT NOT NULL DEFAULT 'Uploads',
    playlistType INTEGER default 0,
    thumbnail BLOB,
    UNIQUE(playlistId,title)
);
CREATE TABLE channelPlaylist (
    channelId INTEGER REFERENCES channel (ch_id) ON DELETE CASCADE,
    playlistId INTEGER REFERENCES playlist (pl_id) ON DELETE CASCADE,
    UNIQUE(channelId,playlistId)
);
CREATE TABLE video (
    vid_id INTEGER PRIMARY KEY AUTOINCREMENT,
    videoId TEXT NOT NULL,
    title TEXT NOT NULL,
    timestamp TEXT,
    duration INTEGER,
    likeCount INTEGER,
    viewCount INTEGER,
    commentCount INTEGER,
    syncState INTEGER default 0,
    thumbnail BLOB,
    quality REAL GENERATED ALWAYS AS (1.0 * likeCount / viewCount * 100) VIRTUAL,
    UNIQUE(videoId,title)
);
CREATE TABLE playlistVideo (
    playlistId INTEGER REFERENCES playlist (pl_id) ON UPDATE CASCADE ON DELETE CASCADE,
    videoId INTEGER REFERENCES video (vid_id) ON UPDATE CASCADE ON DELETE CASCADE,
    UNIQUE(playlistId,videoId)
);

CREATE TRIGGER delete_channel BEFORE DELETE ON channel
    BEGIN
        DELETE FROM video WHERE vid_id in (SELECT videoId FROM playlistVideo WHERE playlistId in (SELECT playlistId FROM channelPlaylist WHERE channelId = old.ch_id));
        DELETE FROM playlist WHERE pl_id in (SELECT playlistId FROM channelPlaylist WHERE channelId = old.ch_id);
    END;
