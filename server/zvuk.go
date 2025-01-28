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

func getArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId string) (int, error) {
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
	return artId, err
}

func getAlbumThumbsDb(tx *sql.Tx, ctx context.Context, albIds []string) (map[string][]byte, error) {
	sqlStr := fmt.Sprintf("select albumId, thumbnail from main.album where albumId in (?%v);", strings.Repeat(",?", len(albIds)-1))
	stmtAlb, err := tx.PrepareContext(ctx, sqlStr)
	if err != nil {
		log.Println(err)
	}
	defer func(stmtAlb *sql.Stmt) {
		err = stmtAlb.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtAlb)

	args := make([]interface{}, len(albIds))
	for i, artId := range albIds {
		args[i] = artId
	}

	rows, err := stmtAlb.QueryContext(ctx, args...)

	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	res := make(map[string][]byte)

	for rows.Next() {
		var (
			artId string
			thumb []byte
		)
		if er := rows.Scan(&artId, &thumb); er != nil {
			log.Println(er)
		} else {
			_, ok := res[artId]
			if !ok {
				res[artId] = thumb
			}
		}
	}

	return res, err
}

func getArtistIdAddDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId interface{}) (int, int, []byte) {
	stmtArt, err := tx.PrepareContext(ctx, "select art_id, userAdded, thumbnail from main.artist where artistId = ? and siteId = ? limit 1;")
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
		artRawId, userAdded int
		thumbnail           []byte
	)

	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId, &userAdded, &thumbnail)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no artist with id %v\n", artistId)
	case err != nil:
		log.Println(err)
	default:
		fmt.Printf("siteId: %v, artist db id is %d\n", siteId, artRawId)
	}

	return artRawId, userAdded, thumbnail
}

func getExistIdsDb(tx *sql.Tx, ctx context.Context, artId int, siteId uint32) ([]string, []string) {
	var (
		existAlbumIds  []string
		existArtistIds []string
	)

	if artId != 0 {
		rows, err := tx.QueryContext(ctx, "select al.albumId, a.artistId from main.artistAlbum aa join main.artist a on a.art_id = aa.artistId join album al on al.alb_id = aa.albumId where aa.albumId in (select albumId from main.artistAlbum where artistId = ?) and a.siteId = ?;", artId, siteId)
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
			} else {
				if albId != "" && !slices2.Contains(existAlbumIds, albId) {
					existAlbumIds = append(existAlbumIds, albId)
				}

				if artisId != "" && !slices2.Contains(existArtistIds, artisId) {
					existArtistIds = append(existArtistIds, artisId)
				}
			}
		}
	}
	return existAlbumIds, existArtistIds
}

func deleteBase(ctx context.Context, tx *sql.Tx, artistId string, siteId uint32) (int64, error) {
	var (
		aff int64
		err error
	)

	artId, err := getArtistIdDb(tx, ctx, siteId, artistId)
	if err != nil {
		return -1, err
	}
	rnd := RandStringBytesMask(4)

	execs := []string{
		fmt.Sprintf("update main.artist set userAdded = 0, thumbnail = null where art_id = %d;", artId),
		fmt.Sprintf("create temporary table _temp_album_%s as select albumId from (select aa.albumId, count(aa.artistId) res from main.artistAlbum aa where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) group by aa.albumId having res = 1) union select albumId from (select aa.albumId, count(aa.artistId) res from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) group by aa.albumId having res > 1) except select aa.albumId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = (select art_id from main.artist where artistId = %v and siteId = 1 limit 1)) and a.userAdded = 1 group by aa.albumId;", rnd, artId, artId, artistId),
		fmt.Sprintf("delete from main.artist where art_id in (select aa.artistId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId = %d) group by aa.artistId except select aa.artistId from main.artistAlbum aa where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId in (select aa.artistId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId = %d) and a.userAdded = 1 and a.art_id <> %d group by aa.artistId)) group by aa.artistId);", artId, artId, artId),
		fmt.Sprintf("delete from main.album where alb_id in (select albumId from _temp_album_%s);", rnd),
		fmt.Sprintf("drop table _temp_album_%s;", rnd),
	}

	for i, exec := range execs {
		func() {
			stmt, er := tx.PrepareContext(ctx, exec)
			if er != nil {
				log.Println(er)
			}

			defer func(stmt *sql.Stmt) {
				err = stmt.Close()
				if err != nil {
					log.Println(err)
				}
			}(stmt)
			cc, e := stmt.ExecContext(ctx)
			if e != nil {
				log.Println(e)
			} else if i == 2 {
				aff, err = cc.RowsAffected()
				if err != nil {
					log.Println(err)
				}
			}
		}()
	}
	return aff, tx.Commit()
}

func DownloadAlbum(ctx context.Context, siteId uint32, albIds []string, trackQuality string) (map[string]string, error) {
	token := GetTokenOnlyDbWoTx(ctx, siteId)
	mDownloaded := make(map[string]string)
	mTracks := make(map[string]*AlbumInfo)

	for _, albumId := range albIds {
		var tryCount int
	L1:
		item, err, canContinue := getAlbumTracks(ctx, albumId, token)
		if err != nil && !canContinue {
			return mDownloaded, err
		}
		if item == nil && canContinue {
			tryCount += 1
			if tryCount == 4 {
				continue
			}

			RandomPause(3, 7)

			goto L1
		}
		if item == nil {
			log.Println("Can't get release info from api, skipped..")
			continue
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

	for trackId, albInfo := range mTracks {
		downloadFiles(ctx, trackId, token, trackQuality, albInfo, mDownloaded)
		RandomPause(3, 7)
	}
	return mDownloaded, nil
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
		} else {
			alb.ArtistIds = append(alb.ArtistIds, strings.Split(artIds, ",")...)
			albs = append(albs, &alb)
		}
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

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, group_concat(ar.artistId, ',') as artIds, a.thumbnail, a.syncState from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.alb_id in (select ab.albumId from main.artistAlbum ab where ab.artistId = (select art.art_id from main.artist art where art.artistId = ? limit 1)) and ar.siteId = ? group by aa.albumId order by 9 desc, 4 desc;")
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
		} else {
			alb.ArtistIds = append(alb.ArtistIds, strings.Split(artIds, ",")...)
			albs = append(albs, &alb)
		}
	}

	return albs, err
}

func DeleteArtistsDb(ctx context.Context, siteId uint32, artistId []string, isUserAdd bool) (int64, error) {
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
	if isUserAdd {
		for _, aid := range artistId {
			aff, er := deleteBase(ctx, tx, aid, siteId)
			if er != nil {
				log.Println(er)
				return 0, tx.Rollback()
			} else {
				log.Printf("deleted artist: %v, rows: %v \n", aid, aff)
				deletedRowCount = deletedRowCount + aff
			}
		}
		return deletedRowCount, err
	} else {
		stmt, er := tx.PrepareContext(ctx, fmt.Sprintf("delete from main.artist where artistId in (? %s) and userAdded == 0 and siteId = ?;", strings.Repeat(",?", len(artistId)-1)))
		if er != nil {
			log.Println(er)
		}

		defer func(stmt *sql.Stmt) {
			err = stmt.Close()
			if err != nil {
				log.Println(err)
			}
		}(stmt)

		args := make([]interface{}, len(artistId))
		for i, artId := range artistId {
			args[i] = artId
		}
		args = append(args, siteId)

		cc, er := stmt.ExecContext(ctx, args...)
		if er != nil {
			log.Println(er)
		}
		deletedRowCount, er = cc.RowsAffected()
		if er != nil {
			log.Println(er)
		}
		return deletedRowCount, tx.Commit()
	}
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
		} else {
			albIds = append(albIds, alb)
		}
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
		} else {
			artistIds = append(artistIds, artId)
		}
	}

	return artistIds, err
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
		} else {
			art.SiteId = siteId
			arts = append(arts, &art)
		}
	}
	return arts, err
}

func ClearAlbSyncStateDb(ctx context.Context, siteId uint32) (int64, error) {
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

	stRows, err := tx.PrepareContext(ctx, "update main.album set syncState = 0 where album.alb_id in (select a.alb_id from main.album a inner join main.artistAlbum aA on a.alb_id = aA.albumId inner join main.artist ar on ar.art_id = aA.artistId where ar.siteId = ? and a.syncState = 1 group by a.alb_id);")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.ExecContext(ctx, siteId)
	if err != nil {
		log.Println(err)
	}
	aff, err := rows.RowsAffected()
	if err != nil {
		log.Println(err)
	}
	return aff, tx.Commit()
}

func SyncArtist(ctx context.Context, siteId uint32, artistId ArtistRawId, isAdd bool) (*artist.Artist, []string, error) {
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

	token := GetTokenOnlyDb(tx, ctx, siteId)
	item, err := getArtistReleases(ctx, artistId.Id, token)
	if item == nil || err != nil {
		log.Println(err)
		return nil, []string{}, tx.Rollback()
	}
	/*if needTokenUpd {
		UpdateTokenDb(tx, ctx, token, siteId)
	}*/

	var (
		artRawId, userAdded int
		thumb               []byte
	)

	if artistId.RawId == 0 {
		artRawId, userAdded, thumb = getArtistIdAddDb(tx, ctx, siteId, artistId.Id)
	} else {
		artRawId = artistId.RawId
		userAdded = 1
	}

	if userAdded == 1 && isAdd {
		// пытались добавить существующего, сделаем просто синк
		isAdd = false
	}

	var existArtistIds, existAlbumIds, netAlbumIds, netArtistIds []string
	mArtist := make(map[string]int)

	if artRawId != 0 {
		existAlbumIds, existArtistIds = getExistIdsDb(tx, ctx, artRawId, siteId)
		mArtist[artistId.Id] = artRawId
	}

	for _, data := range item.GetArtists {
		for _, release := range data.Discography.All.Releases {
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
	fmt.Printf("siteId: %v, artistId: %d, deleted: %d\n", siteId, artRawId, len(deletedArtistIds))

	newAlbumIds := FindDifference(netAlbumIds, existAlbumIds)
	fmt.Printf("siteId: %v, artistId: %d, new albums: %d\n", siteId, artRawId, len(newAlbumIds))

	newArtistIds := FindDifference(netArtistIds, existArtistIds)
	fmt.Printf("siteId: %v, artistId: %d, new artists: %d\n", siteId, artRawId, len(newArtistIds))

	var (
		artists            []*artist.Artist
		albums             []*artist.Album
		processedArtistIds []string
		processedAlbumIds  []string
		albThumbDb         map[string][]byte
	)
	if isAdd {
		albThumbDb, err = getAlbumThumbsDb(tx, ctx, netAlbumIds)
		if err != nil {
			log.Println(err)
		}
	}

	resArtist := &artist.Artist{
		SiteId:    siteId,
		ArtistId:  item.GetArtists[0].ID,
		Title:     item.GetArtists[0].Title,
		UserAdded: true,
	}
	if thumb == nil {
		thumb = GetThumb(ctx, strings.Replace(item.GetArtists[0].Image.Src, "{size}", thumbSize, 1))
		resArtist.Thumbnail = thumb
	} else {
		resArtist.Thumbnail = thumb
	}
	artists = append(artists, resArtist)

	for _, data := range item.GetArtists {
		for _, release := range data.Discography.All.Releases {
			if release.ID == "" {
				continue
			}
			alb := &artist.Album{
				Title:       strings.TrimSpace(release.Title),
				ReleaseDate: strings.Replace(release.Date, "T", " ", 1),
				ReleaseType: MapReleaseType(release.Type),
			}
			if slices2.Contains(newAlbumIds, release.ID) && !slices2.Contains(processedAlbumIds, release.ID) {
				alb.AlbumId = release.ID
				alb.Thumbnail = GetThumb(ctx, strings.Replace(release.Image.Src, "{size}", thumbSize, 1))
				if !isAdd {
					alb.SyncState = 1
				}
				processedAlbumIds = append(processedAlbumIds, release.ID)
			} else {
				if isAdd {
					th, ok := albThumbDb[release.ID]
					if ok {
						alb.Thumbnail = th
					} else {
						alb.Thumbnail = GetThumb(ctx, strings.Replace(release.Image.Src, "{size}", thumbSize, 1))
					}
				}
			}

			sb := make([]string, len(release.Artists))
			for i, author := range release.Artists {
				if author.ID == "" {
					continue
				}
				sb[i] = author.Title
				alb.ArtistIds = append(alb.ArtistIds, author.ID)
				if author.ID == artistId.Id {
					continue
				}

				if slices2.Contains(newArtistIds, author.ID) && !slices2.Contains(processedArtistIds, author.ID) {
					art := &artist.Artist{
						ArtistId: author.ID,
						Title:    strings.TrimSpace(author.Title),
					}
					artists = append(artists, art)
					processedArtistIds = append(processedArtistIds, author.ID)
				}
			}

			if isAdd || alb.AlbumId != "" {
				alb.SubTitle = strings.Join(sb, ", ")
				if !slices2.Contains(alb.ArtistIds, artistId.Id) {
					// api bug
					alb.ArtistIds = append(alb.ArtistIds, artistId.Id)
					alb.SubTitle = fmt.Sprintf("%v, %v", alb.SubTitle, resArtist.Title)
				}
				albums = append(albums, alb)
			}
		}
	}

	type artAlb struct {
		art int
		alb int
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
					fmt.Printf("processed artist: %v, id: %v \n", art.GetTitle(), artId)
				}
				mArtist[art.GetArtistId()] = artId
			}
		}
	}

	if albums != nil {
		var stAlbum *sql.Stmt
		if newAlbumIds != nil {
			stAlbum, err = tx.PrepareContext(ctx, "insert into main.album(albumId, title, releaseDate, releaseType, thumbnail, syncState) values (?,?,?,?,?,?) on conflict (albumId, title) do update set syncState = 0 returning alb_id;")
			defer func(stAlbum *sql.Stmt) {
				err = stAlbum.Close()
				if err != nil {
					log.Println(err)
				}
			}(stAlbum)
		}

		for _, album := range albums {
			if album.GetAlbumId() != "" && stAlbum != nil {
				err = stAlbum.QueryRowContext(ctx, album.GetAlbumId(), album.GetTitle(), album.GetReleaseDate(), album.GetReleaseType(), album.GetThumbnail(), album.GetSyncState()).Scan(&albId)
				if err != nil {
					log.Println(err)
				} else {
					fmt.Printf("processed album: %v, id: %v \n", album.GetTitle(), albId)
				}

				for _, arId := range album.GetArtistIds() {
					artId, ok := mArtist[arId]
					if ok {
						artAlbs = append(artAlbs, &artAlb{
							art: artId,
							alb: albId,
						})
					} else {
						artId, err = getArtistIdDb(tx, ctx, siteId, arId)
						if err == nil && artId != 0 {
							mArtist[arId] = artId
							artAlbs = append(artAlbs, &artAlb{
								art: artId,
								alb: albId,
							})
						}
					}
				}
			}
			resArtist.Albums = append(resArtist.Albums, album)
		}
	} else if len(artists) > 1 {
		fmt.Printf("siteId: %v, artistId: %d, new relations found, processing..\n", siteId, artRawId)
		mAlbum := make(map[string]int)

		for _, data := range item.GetArtists {
			for _, release := range data.Discography.All.Releases {
				for _, ar := range release.Artists {
					if release.ID == "" {
						continue
					}

					for _, art := range artists[1:] {
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

	if artRawId != 0 && userAdded != 1 && thumb != nil {
		fmt.Printf("siteId: %v, artistId: %d, avatar has been updated\n", siteId, artRawId)
		stArtistUpd, _ := tx.PrepareContext(ctx, "update main.artist set userAdded = 1, thumbnail = ? where art_id = ? and siteId = ?;")

		defer func(stArtistUpd *sql.Stmt) {
			err = stArtistUpd.Close()
			if err != nil {
				log.Println(err)
			}
		}(stArtistUpd)

		_, err = stArtistUpd.ExecContext(ctx, thumb, artRawId, siteId)
		if err != nil {
			log.Println(err)
		}
	}

	if artAlbs != nil {
		fmt.Printf("siteId: %v, artistId: %d, relations: %d\n", siteId, artRawId, len(artAlbs))
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

	return resArtist, deletedArtistIds, tx.Commit()
}
