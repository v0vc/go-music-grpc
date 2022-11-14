CREATE TABLE "site" (
    "id" INTEGER NOT NULL CONSTRAINT "PK_site" PRIMARY KEY AUTOINCREMENT,
    "title" TEXT NOT NULL,
    "login" TEXT NULL,
    "pass" TEXT NULL,
    "token" TEXT NULL
);
CREATE TABLE "artist" (
    "id" INTEGER NOT NULL CONSTRAINT "PK_artist" PRIMARY KEY AUTOINCREMENT,
    "siteId" INTEGER NOT NULL,
    "artistId" TEXT NOT NULL,
    "title" TEXT NULL,
    "counter" INTEGER NULL,
    "thumbnail" BLOB NULL,
    "lastDate" TEXT NULL,
    UNIQUE(siteId,artistId),
    CONSTRAINT "FK_artist_site_siteId" FOREIGN KEY ("siteId") REFERENCES "site" ("id") ON DELETE RESTRICT
);
CREATE TABLE "album" (
    "id" INTEGER NOT NULL CONSTRAINT "release" PRIMARY KEY AUTOINCREMENT,
    "albumId" TEXT NOT NULL,
    "title" TEXT NOT NULL,
    "releaseDate" TEXT NULL,
    "releaseType" TEXT NULL,
    "thumbnail" BLOB NULL,
    UNIQUE(albumId,title)
);
CREATE TABLE "artistAlbum" (
    "artistId" INTEGER NOT NULL,
    "albumId" INTEGER NOT NULL,
    CONSTRAINT "artistRelease" PRIMARY KEY ("artistId", "albumId"),
    CONSTRAINT "FK_artistAlbum_artist_artistId" FOREIGN KEY ("artistId") REFERENCES "artist" ("id") ON DELETE CASCADE,
    CONSTRAINT "FK_artistAlbum_album_albumId" FOREIGN KEY ("albumId") REFERENCES "album" ("id") ON DELETE CASCADE
);
CREATE TABLE "track" (
    "id" INTEGER NOT NULL CONSTRAINT "PK_track" PRIMARY KEY AUTOINCREMENT,
    "releaseId" TEXT NOT NULL,
    "title" TEXT NULL,
    "trackId" TEXT NOT NULL,
    "duration" INTEGER NOT NULL,
    CONSTRAINT "FK_track_release_releaseId" FOREIGN KEY ("releaseId") REFERENCES "album" ("id") ON DELETE CASCADE
);