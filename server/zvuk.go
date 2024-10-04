package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	slices2 "golang.org/x/exp/slices"

	"github.com/v0vc/go-music-grpc/artist"
)

/*func runExec(tx *sql.Tx, ctx context.Context, ids []string, command string) {
	if ids != nil {
		stDelete, err := tx.PrepareContext(ctx, command)
		if err != nil {
			log.Fatal(err)
		}
		defer stDelete.Close()

		for _, id := range ids {
			_, _ = stDelete.ExecContext(ctx, id)
		}
	}
}*/

func getAlbumIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, albumId string) int {
	stmtAlb, err := tx.PrepareContext(ctx, "select aa.albumId from main.artistAlbum aa join album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.albumId = ? and ar.siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtAlb *sql.Stmt) {
		err = stmtAlb.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtAlb)

	var albId int
	err = stmtAlb.QueryRowContext(ctx, albumId, siteId).Scan(&albId)
	if err != nil {
		log.Println(err)
	}
	return albId
}

func getArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId string) int {
	stmtAlb, err := tx.PrepareContext(ctx, "select art_id from main.artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtAlb *sql.Stmt) {
		err = stmtAlb.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtAlb)

	var artId int
	err = stmtAlb.QueryRowContext(ctx, artistId, siteId).Scan(&artId)
	if err != nil {
		log.Println(err)
	}
	return artId
}

func getArtistIdAddDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId interface{}) (int, int) {
	stmtArt, err := tx.PrepareContext(ctx, "select art_id, userAdded from main.artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtArt)

	var (
		artRawId  int
		userAdded int
	)

	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId, &userAdded)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no artist with id %v\n", artistId)
	case err != nil:
		log.Println(err)
	default:
		log.Printf("siteId: %v, artist db id is %d\n", siteId, artRawId)
	}
	return artRawId, userAdded
}

func getExistIdsDb(tx *sql.Tx, ctx context.Context, artId int) ([]string, []string) {
	var (
		existAlbumIds  []string
		existArtistIds []string
	)

	if artId != 0 {
		rows, err := tx.QueryContext(ctx, "select al.albumId, a.artistId res from main.artistAlbum aa join main.artist a on a.art_id = aa.artistId join album al on al.alb_id = aa.albumId where aa.albumId in (select albumId from main.artistAlbum where artistId = ?);", artId)
		if err != nil {
			log.Println(err)
		}

		defer func(rows *sql.Rows) {
			err = rows.Close()
			if err != nil {
				log.Println(err)
			}
		}(rows)

		for rows.Next() {
			var (
				albId   string
				artisId string
			)

			if er := rows.Scan(&albId, &artisId); er != nil {
				log.Println(er)
			}

			if albId != "" && !slices2.Contains(existAlbumIds, albId) {
				existAlbumIds = append(existAlbumIds, albId)
			}

			if artisId != "" && !slices2.Contains(existArtistIds, artisId) {
				existArtistIds = append(existArtistIds, artisId)
			}
		}
	}
	return existAlbumIds, existArtistIds
}

func getTrackFromDb(tx *sql.Tx, ctx context.Context, siteId uint32, ids []string, isAlbum bool) (map[string]*AlbumInfo, []string) {
	var sqlStr string

	if len(ids) == 1 {
		if isAlbum {
			sqlStr = "select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.albumId in (select alb_id from album where albumId = ? limit 1) and ar.siteId = ? group by at.trackId;"
		} else {
			sqlStr = "select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.trackId in (select trk_id from track where trackId = ? limit 1) and ar.siteId = ? group by at.trackId;"
		}
	} else {
		if isAlbum {
			sqlStr = fmt.Sprintf("select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.albumId in (select alb_id from album where albumId in (? %v)) and ar.siteId = ? group by at.trackId;", strings.Repeat(",?", len(ids)-1))
		} else {
			sqlStr = fmt.Sprintf("select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.trackId in (select trk_id from track where trackId in (? %v)) and ar.siteId = ? group by at.trackId;", strings.Repeat(",?", len(ids)-1))
		}
	}

	stRows, err := tx.PrepareContext(ctx, sqlStr)
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	args := make([]interface{}, len(ids))
	for i, trackId := range ids {
		args[i] = trackId
	}

	args = append(args, siteId)
	rows, err := stRows.QueryContext(ctx, args...)
	if err != nil {
		log.Println(err)
	}

	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	mTracks := make(map[string]*AlbumInfo)

	var mAlbum []string

	for rows.Next() {
		var (
			trackId string
			alb     AlbumInfo
		)
		if er := rows.Scan(&alb.ArtistTitle, &alb.AlbumTitle, &alb.AlbumId, &alb.AlbumYear, &alb.AlbumCover, &trackId, &alb.TrackNum, &alb.TrackTotal, &alb.TrackTitle, &alb.TrackGenre); er != nil {
			log.Println(er)
		}
		_, ok := mTracks[trackId]
		if !ok {
			mTracks[trackId] = &alb
		}
		if isAlbum && !slices2.Contains(mAlbum, alb.AlbumId) {
			mAlbum = append(mAlbum, alb.AlbumId)
		}
	}

	return mTracks, mAlbum
}

func deleteBase(ctx context.Context, tx *sql.Tx, artistId string, siteId uint32, isCommit bool) (int64, error) {
	artId := getArtistIdDb(tx, ctx, siteId, artistId)
	rnd := RandStringBytesMask(4)

	execs := []struct {
		stmt string
		res  int
	}{
		{stmt: fmt.Sprintf("update main.artist set userAdded = 0 where art_id = %d", artId), res: 0},
		{stmt: fmt.Sprintf("create temporary table _temp_album_%v as select albumId from (select aa.albumId, count(aa.artistId) res from main.artistAlbum aa where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) group by aa.albumId having res = 1) union select albumId from (select aa.albumId, count(aa.artistId) res from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) group by aa.albumId having res > 1) except select aa.albumId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = (select art_id from main.artist where artistId = %v and siteId = 1 limit 1)) and a.userAdded = 1 group by aa.albumId;", rnd, artId, artId, artistId), res: 1},
		{stmt: fmt.Sprintf("delete from main.artist where art_id in (select aa.artistId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId = %d) group by aa.artistId except select aa.artistId from main.artistAlbum aa where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId in (select aa.artistId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId = %d) and a.userAdded = 1 and a.art_id <> %d group by aa.artistId)) group by aa.artistId);", artId, artId, artId), res: 2},
		{stmt: fmt.Sprintf("delete from main.album where alb_id in (select albumId from _temp_album_%v);", rnd), res: 3},
		{stmt: fmt.Sprintf("drop table _temp_album_%v;", rnd), res: 4},
	}

	var (
		aff int64
		err error
	)

	for _, exec := range execs {
		func() {
			stmt, er := tx.PrepareContext(ctx, exec.stmt)
			if er != nil {
				log.Println(er)
			}

			defer func(stmt *sql.Stmt) {
				err = stmt.Close()
				if err != nil {
					log.Println(err)
				}
			}(stmt)
			cc, er := stmt.ExecContext(ctx)
			if er != nil {
				log.Println(er)
			} else if exec.res == 3 {
				aff, err = cc.RowsAffected()
				if err != nil {
					log.Println(err)
				}
			}
		}()
	}
	if isCommit {
		return aff, tx.Commit()
	}
	return aff, err
}

func DownloadTracks(ctx context.Context, siteId uint32, trackIds []string, trackQuality string) (map[string]string, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=false&cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	mTracks, _ := getTrackFromDb(tx, ctx, siteId, trackIds, false)
	_, _, token := GetTokenDb(tx, ctx, siteId)
	err = tx.Rollback()
	if err != nil {
		log.Println(err)
	}
	mDownloaded := make(map[string]string)

	for _, trackId := range trackIds {
		albInfo, dbExist := mTracks[trackId]
		if dbExist {
			downloadFiles(ctx, trackId, token, trackQuality, albInfo, mDownloaded)
		} else {
			log.Println("Track not found in database, please sync")
			// нет в базе, можно продумать как формировать пути скачивания без данных в базе, типа лить в базовую папку без прохода по темплейтам альбома, хз
		}

		RandomPause(3, 7)
	}

	return mDownloaded, err
}

func DownloadAlbum(ctx context.Context, siteId uint32, albIds []string, trackQuality string) (map[string]string, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=false&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	mTracks, dbAlbums := getTrackFromDb(tx, ctx, siteId, albIds, true)
	login, pass, token := GetTokenDb(tx, ctx, siteId)

	mDownloaded := make(map[string]string)

	notDbAlbumIds := FindDifference(albIds, dbAlbums)
	for _, albumId := range notDbAlbumIds {
		var tryCount int
	L1:
		item, tokenNew, needTokenUpd, er := getAlbumTracks(ctx, albumId, token, login, pass)
		if er != nil {
			return mDownloaded, er
		}
		if needTokenUpd {
			UpdateTokenDb(tx, ctx, tokenNew, siteId)
		}

		if item == nil {
			tryCount += 1
			if tryCount == 4 {
				continue
			}

			RandomPause(3, 7)

			goto L1
		}

		for trId, track := range item.Result.Tracks {
			if trId != "" {
				_, ok := mTracks[trId]
				if !ok {
					var alb AlbumInfo
					alb.AlbumId = strconv.Itoa(track.ReleaseID)
					alb.ArtistTitle = strings.Join(item.Result.Releases[alb.AlbumId].ArtistNames, ", ")
					alb.AlbumTitle = item.Result.Releases[alb.AlbumId].Title
					alb.AlbumYear = strconv.Itoa(item.Result.Releases[alb.AlbumId].Date)[:4]
					alb.AlbumCover = item.Result.Releases[alb.AlbumId].Image.Src
					alb.TrackNum = strconv.Itoa(track.Position)
					alb.TrackTotal = strconv.Itoa(len(item.Result.Tracks))
					alb.TrackTitle = track.Title
					alb.TrackGenre = strings.Join(track.Genres, ", ")
					mTracks[trId] = &alb
				}
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Println(err)
	}

	for trackId, albInfo := range mTracks {
		downloadFiles(ctx, trackId, token, trackQuality, albInfo, mDownloaded)
		RandomPause(3, 7)
	}
	return mDownloaded, err
}

func GetNewReleasesFromDb(ctx context.Context, siteId uint32) ([]*artist.Album, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, group_concat(ar.artistId, ',') as artIds, a.thumbnail from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.syncState = 1 and ar.siteId = ? group by aa.albumId order by 4 desc;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx, siteId)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var albs []*artist.Album

	for rows.Next() {
		var alb artist.Album

		var artIds string

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.ReleaseDate, &alb.ReleaseType, &alb.SubTitle, &artIds, &alb.Thumbnail); err != nil {
			log.Println(err)
		}

		alb.ArtistIds = append(alb.ArtistIds, strings.Split(artIds, ",")...)
		albs = append(albs, &alb)
	}

	return albs, err
}

func GetArtistReleasesFromDb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Album, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, group_concat(ar.artistId, ',') as artIds, a.thumbnail, a.syncState from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.alb_id in (select ab.albumId from main.artistAlbum ab where ab.artistId in (select art.art_id from main.artist art where art.artistId = ? limit 1)) and ar.siteId = ? group by aa.albumId order by 9 desc, 4 desc;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx, artistId, siteId)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var albs []*artist.Album

	for rows.Next() {
		var (
			alb    artist.Album
			artIds string
		)

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.ReleaseDate, &alb.ReleaseType, &alb.SubTitle, &artIds, &alb.Thumbnail, &alb.SyncState); err != nil {
			log.Println(err)
		}

		alb.ArtistIds = append(alb.ArtistIds, strings.Split(artIds, ",")...)
		albs = append(albs, &alb)
	}

	return albs, err
}

func DeleteArtistsDb(ctx context.Context, siteId uint32, artistId []string) (int64, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	var deletedRowCount int64
	for _, aid := range artistId {
		aff, er := deleteBase(ctx, tx, aid, siteId, false)
		if er != nil {
			log.Println(er)
			return 0, tx.Rollback()
		} else {
			log.Printf("deleted artist: %v, rows: %v \n", aid, aff)
			deletedRowCount = deletedRowCount + aff
		}
	}
	return deletedRowCount, tx.Commit()
}

func GetArtistReleasesIdFromDb(ctx context.Context, siteId uint32, artistId string, newOnly bool) ([]string, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	var str string

	if newOnly {
		str = "select a.albumId from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.syncState = 1 and ar.siteId = ? group by aa.albumId;"
	} else {
		str = "select a.albumId from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where ar.artistId = ? and ar.siteId = ?;"
	}
	stRows, err := db.PrepareContext(ctx, str)
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	var rows *sql.Rows
	if newOnly {
		rows, err = stRows.QueryContext(ctx, siteId)
	} else {
		rows, err = stRows.QueryContext(ctx, artistId, siteId)
	}

	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var albIds []string

	for rows.Next() {
		var alb string
		if err = rows.Scan(&alb); err != nil {
			log.Println(err)
		}
		albIds = append(albIds, alb)
	}

	return albIds, err
}

func GetArtistIdsFromDb(ctx context.Context, siteId uint32) ([]ArtistRawId, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	var artistIds []ArtistRawId

	stmtArt, err := db.PrepareContext(ctx, "select a.art_id, a.artistId from main.artist a where a.siteId = ? and a.userAdded = 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtArt)

	rows, err := stmtArt.QueryContext(ctx, siteId)
	if err != nil {
		log.Println(err)
	}

	for rows.Next() {
		var artId ArtistRawId

		if er := rows.Scan(&artId.RawId, &artId.Id); er != nil {
			log.Println(err)
		}

		artistIds = append(artistIds, artId)
	}

	return artistIds, err
}

func GetAlbumTrackFromDb(ctx context.Context, siteId uint32, albumId string) ([]*artist.Track, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	stRows, err := db.PrepareContext(ctx, "select t.trk_id, t.trackId, t.title, t.hasFlac, t.hasLyric, t.quality, t.condition, t.genre, t.trackNum, t.duration from main.albumTrack at join track t on t.trk_id = at.trackId join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = at.albumId join main.artist ar on ar.art_id = aa.artistId where a.albumId = ? and ar.siteId = ? order by t.trackNum;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx, albumId, siteId)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var tracks []*artist.Track

	for rows.Next() {
		var track artist.Track
		if err = rows.Scan(&track.Id, &track.TrackId, &track.Title, &track.HasFlac, &track.HasLyric, &track.Quality, &track.Condition, &track.Genre, &track.TrackNum, &track.Duration); err != nil {
			log.Println(err)
		}

		tracks = append(tracks, &track)
	}

	return tracks, err
}

func GetArtists(ctx context.Context, siteId uint32) ([]*artist.Artist, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))

	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	var arts []*artist.Artist

	stmt, err := db.PrepareContext(context.WithoutCancel(ctx), "select ar.art_id, ar.artistId, ar.title, ar.thumbnail, count(al.alb_id) as news from main.artist ar join main.artistAlbum aa on ar.art_id = aa.artistId left outer join main.album al on aa.albumId = al.alb_id and al.syncState = 1 where ar.userAdded = 1 and ar.siteId = ? group by ar.art_id order by 3;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmt)

	rows, err := stmt.QueryContext(context.WithoutCancel(ctx), siteId)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	for rows.Next() {
		var art artist.Artist
		if e := rows.Scan(&art.Id, &art.ArtistId, &art.Title, &art.Thumbnail, &art.NewAlbs); e != nil {
			log.Println(e)
		}
		art.SiteId = siteId
		arts = append(arts, &art)
	}
	return arts, err
}

func SyncArtist(ctx context.Context, siteId uint32, artistId ArtistRawId, isAdd bool, isDelete bool) (*artist.Artist, []string, error) {
	var resArtist *artist.Artist

	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	login, pass, token := GetTokenDb(tx, ctx, siteId)
	item, token, needTokenUpd, err := getArtistReleases(ctx, artistId.Id, token, login, pass)
	if item == nil || err != nil {
		log.Println(err)
		return resArtist, []string{}, tx.Rollback()
	}
	if needTokenUpd {
		UpdateTokenDb(tx, ctx, token, siteId)
	}

	var (
		artRawId int
		uAdd     int
	)

	if artistId.RawId == 0 {
		artRawId, uAdd = getArtistIdAddDb(tx, ctx, siteId, artistId.Id)
	} else {
		artRawId = artistId.RawId
		uAdd = 1
	}
	if uAdd == 1 && isAdd {
		// пытались добавить существующего, сделаем просто синк
		isAdd = false
	}

	var existArtistIds, existAlbumIds, netAlbumIds, netArtistIds []string
	mArtist := make(map[string]int)

	if artRawId != 0 {
		existAlbumIds, existArtistIds = getExistIdsDb(tx, ctx, artRawId)
		mArtist[artistId.Id] = artRawId
	}

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			if !slices2.Contains(netAlbumIds, release.ID) {
				netAlbumIds = append(netAlbumIds, release.ID)
			}

			for _, author := range release.Artists {
				if author.ID == "" {
					continue
				}
				if !slices2.Contains(netArtistIds, author.ID) {
					netArtistIds = append(netArtistIds, author.ID)
				}
			}
		}
	}

	deletedArtistIds := FindDifference(existArtistIds, netArtistIds)
	log.Printf("siteId: %v, artistId: %d, deleted: %d\n", siteId, artRawId, len(deletedArtistIds))

	newAlbumIds := FindDifference(netAlbumIds, existAlbumIds)
	log.Printf("siteId: %v, artistId: %d, new albums: %d\n", siteId, artRawId, len(newAlbumIds))

	newArtistIds := FindDifference(netArtistIds, existArtistIds)
	log.Printf("siteId: %v, artistId: %d, new artists: %d\n", siteId, artRawId, len(newArtistIds))

	type artAlb struct {
		art int
		alb int
	}

	var (
		processedAlbumIds  []string
		processedArtistIds []string
		thumb              []byte
		artists            []*artist.Artist
		albums             []*artist.Album
	)

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			alb := &artist.Album{}
			if slices2.Contains(newAlbumIds, release.ID) && !slices2.Contains(processedAlbumIds, release.ID) {
				alb.AlbumId = release.ID
				alb.Title = strings.TrimSpace(release.Title)
				alb.ReleaseDate = release.Date
				alb.ReleaseType = MapReleaseType(release.Type)
				alb.Thumbnail = GetThumb(ctx, strings.Replace(release.Image.Src, "{size}", thumbSize, 1))
				if !isAdd {
					alb.SyncState = 1
				}
				processedAlbumIds = append(processedAlbumIds, release.ID)
			} else {
				if isAdd {
					alb.Title = strings.TrimSpace(release.Title)
					alb.ReleaseDate = release.Date
					alb.ReleaseType = MapReleaseType(release.Type)
					alb.Thumbnail = GetThumb(ctx, strings.Replace(release.Image.Src, "{size}", thumbSize, 1))
				}
			}

			if isAdd || alb.AlbumId != "" {
				sb := make([]string, len(release.Artists))
				for i, author := range release.Artists {
					if author.ID == "" {
						continue
					}
					sb[i] = author.Title
					alb.ArtistIds = append(alb.ArtistIds, author.ID)

					if slices2.Contains(newArtistIds, author.ID) && !slices2.Contains(processedArtistIds, author.ID) {
						art := &artist.Artist{
							ArtistId: author.ID,
							Title:    strings.TrimSpace(author.Title),
						}
						if isAdd && art.GetArtistId() == artistId.Id && art.Thumbnail == nil {
							art.Thumbnail = GetThumb(ctx, strings.Replace(author.Image.Src, "{size}", thumbSize, 1))
							art.UserAdded = true
							resArtist = art
						}
						artists = append(artists, art)
						processedArtistIds = append(processedArtistIds, author.ID)
					} else if artRawId != 0 && author.ID == artistId.Id && thumb == nil && resArtist == nil {
						thumb = GetThumb(ctx, strings.Replace(author.Image.Src, "{size}", thumbSize, 1))
						resArtist = &artist.Artist{
							SiteId:    siteId,
							ArtistId:  artistId.Id,
							Title:     author.Title,
							Thumbnail: thumb,
							UserAdded: true,
						}
					}
				}

				alb.SubTitle = strings.Join(sb, ", ")
				if !slices2.Contains(alb.ArtistIds, artistId.Id) {
					// api bug
					alb.ArtistIds = append(alb.ArtistIds, artistId.Id)
					if resArtist != nil {
						alb.SubTitle = fmt.Sprintf("%v, %v", alb.SubTitle, resArtist.Title)
					}
				}
				albums = append(albums, alb)
			}
		}
	}

	var (
		artAlbs []*artAlb
		albId   int
	)
	if artists != nil {
		stArtist, _ := tx.PrepareContext(ctx, "insert into main.artist(siteId, artistId, title) values (?,?,?) on conflict (siteId, artistId) do update set syncState = 1 returning art_id;")
		defer func(stArtist *sql.Stmt) {
			err = stArtist.Close()
			if err != nil {
				log.Println(err)
			}
		}(stArtist)

		stArtistUser, _ := tx.PrepareContext(ctx, "insert into main.artist(siteId, artistId, title, userAdded, thumbnail) values (?,?,?,?,?) on conflict (siteId, artistId) do update set userAdded = 1 returning art_id;")
		defer func(stArtistUser *sql.Stmt) {
			err = stArtistUser.Close()
			if err != nil {
				log.Println(err)
			}
		}(stArtistUser)

		for _, art := range artists {
			artId, ok := mArtist[art.GetArtistId()]
			if !ok {
				var insErr error
				if art.GetUserAdded() {
					insErr = stArtistUser.QueryRowContext(ctx, siteId, art.GetArtistId(), art.GetTitle(), 1, art.GetThumbnail()).Scan(&artId)
				} else {
					insErr = stArtist.QueryRowContext(ctx, siteId, art.GetArtistId(), art.GetTitle()).Scan(&artId)
				}
				if insErr != nil {
					log.Println(insErr)
				} else {
					log.Printf("Processed artist: %v, id: %v \n", art.GetTitle(), artId)
				}
				mArtist[art.GetArtistId()] = artId
			}
		}
	}

	if albums != nil {
		stAlbum, _ := tx.PrepareContext(ctx, "insert into main.album(albumId, title, releaseDate, releaseType, thumbnail, syncState) values (?,?,?,?,?,?) on conflict (albumId, title) do update set syncState = 0 returning alb_id;")
		defer func(stAlbum *sql.Stmt) {
			err = stAlbum.Close()
			if err != nil {
				log.Println(err)
			}
		}(stAlbum)

		for _, album := range albums {
			if album.GetAlbumId() != "" {
				err = stAlbum.QueryRowContext(ctx, album.GetAlbumId(), album.GetTitle(), album.GetReleaseDate(), album.GetReleaseType(), album.GetThumbnail(), album.GetSyncState()).Scan(&albId)
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("Processed album: %v, id: %v \n", album.GetTitle(), albId)
				}

				for _, arId := range album.GetArtistIds() {
					artId, ok := mArtist[arId]
					if ok {
						artAlbs = append(artAlbs, &artAlb{
							art: artId,
							alb: albId,
						})
					} else {
						artId = getArtistIdDb(tx, ctx, siteId, arId)
						if artId != 0 {
							mArtist[arId] = artId
							artAlbs = append(artAlbs, &artAlb{
								art: artId,
								alb: albId,
							})
						}
					}
				}
			}
			if resArtist != nil {
				resArtist.Albums = append(resArtist.Albums, album)
			}
		}
	} else if artists != nil {
		log.Printf("siteId: %v, artistId: %d, new relations found, processing..\n", siteId, artRawId)
		mAlbum := make(map[string]int)

		for _, data := range item.GetArtists {
			for _, release := range data.Releases {
				for _, ar := range release.Artists {
					if release.ID == "" {
						continue
					}

					for _, art := range artists {
						if ar.ID == art.GetArtistId() {
							alId, ok := mAlbum[release.ID]
							if !ok {
								alId = getAlbumIdDb(tx, ctx, siteId, release.ID)
								mAlbum[release.ID] = alId
								artId, exist := mArtist[art.GetArtistId()]
								if exist {
									artAlbs = append(artAlbs, &artAlb{
										art: artId,
										alb: alId,
									})
								}
							} else {
								artId, exist := mArtist[art.GetArtistId()]
								if exist {
									artAlbs = append(artAlbs, &artAlb{
										art: artId,
										alb: alId,
									})
								}
							}
						}
					}
				}
			}
		}
	}

	if isAdd && artRawId != 0 && uAdd != 1 && thumb != nil {
		log.Printf("siteId: %v, artistId: %d, avatar has been updated\n", siteId, artRawId)
		stArtistUpd, _ := tx.PrepareContext(ctx, "update main.artist set userAdded = 1, thumbnail = ? where art_id = ?;")

		defer func(stArtistUpd *sql.Stmt) {
			err = stArtistUpd.Close()
			if err != nil {
				log.Println(err)
			}
		}(stArtistUpd)

		_, err = stArtistUpd.ExecContext(ctx, thumb, artRawId)
		if err != nil {
			log.Println(err)
		}
	}

	if artAlbs != nil {
		log.Printf("siteId: %v, artistId: %d, relations: %d\n", siteId, artRawId, len(artAlbs))
		sqlStr := fmt.Sprintf("insert into main.artistAlbum(artistId, albumId) values %v on conflict (artistId, albumId) do nothing;", strings.TrimSuffix(strings.Repeat("(?,?),", len(artAlbs)), ","))
		stArtAlb, _ := tx.PrepareContext(ctx, sqlStr)

		defer func(stArtAlb *sql.Stmt) {
			err = stArtAlb.Close()
			if err != nil {
				log.Println(err)
			}
		}(stArtAlb)

		var args []interface{}
		for _, artAl := range artAlbs {
			args = append(args, &artAl.art, &artAl.alb)
		}

		_, err = stArtAlb.ExecContext(ctx, args...)
		if err != nil {
			log.Println(err)
		}
	}

	if isDelete {
		for _, aid := range deletedArtistIds {
			aff, er := deleteBase(ctx, tx, aid, siteId, false)
			if er != nil {
				log.Println(er)
			} else {
				log.Printf("deleted artist: %v, rows: %v \n", aid, aff)
			}
		}
	}

	return resArtist, deletedArtistIds, tx.Commit()
}
