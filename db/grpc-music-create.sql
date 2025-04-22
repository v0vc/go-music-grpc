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
CREATE TABLE channel (
    ch_id INTEGER PRIMARY KEY AUTOINCREMENT,
    siteId INTEGER REFERENCES site (site_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    channelId TEXT NOT NULL,
    title TEXT NOT NULL,
    syncState INTEGER DEFAULT 1 NOT NULL,
    thumbnail BLOB,
    UNIQUE(siteId,channelId)
);
CREATE TABLE playlist (
    pl_id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlistId TEXT,
    title TEXT NOT NULL DEFAULT 'Uploads',
    playlistType INTEGER DEFAULT 0 NOT NULL,
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
    duration INTEGER DEFAULT 0 NOT NULL,
    likeCount INTEGER DEFAULT 0 NOT NULL,
    viewCount INTEGER DEFAULT 0 NOT NULL,
    commentCount INTEGER,
    syncState INTEGER DEFAULT 0 NOT NULL,
    listState INTEGER DEFAULT 0 NOT NULL,
    watchState INTEGER DEFAULT 0 NOT NULL,
    thumbnail BLOB,
    quality REAL GENERATED ALWAYS AS (1.0 * likeCount / viewCount * 100) VIRTUAL,
    UNIQUE(videoId,title)
);
CREATE TABLE playlistVideo (
    playlistId INTEGER REFERENCES playlist (pl_id) ON UPDATE CASCADE ON DELETE CASCADE,
    videoId INTEGER REFERENCES video (vid_id) ON UPDATE CASCADE ON DELETE CASCADE,
    UNIQUE(playlistId,videoId)
);

CREATE TRIGGER IF NOT EXISTS delete_channel BEFORE DELETE ON channel
    BEGIN
        DELETE FROM video WHERE vid_id in (SELECT videoId FROM playlistVideo WHERE playlistId in (SELECT playlistId FROM channelPlaylist WHERE channelId = old.ch_id));
        DELETE FROM playlist WHERE pl_id in (SELECT playlistId FROM channelPlaylist WHERE channelId = old.ch_id);
    END;

CREATE INDEX index_channel_site ON channel(siteId);

CREATE INDEX index_artist_site ON artist(siteId);

CREATE INDEX index_channelPlaylist_channelId ON channelPlaylist(channelId);

CREATE INDEX index_channelPlaylist_playlistId ON channelPlaylist(playlistId);

CREATE INDEX index_playlist_playlistType ON playlist(playlistType);

CREATE INDEX index_video_syncState ON video(syncState);

CREATE INDEX index_playlistVideo_playlistId ON playlistVideo(playlistId);

CREATE INDEX index_playlistVideo_videoId ON playlistVideo(videoId);