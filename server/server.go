package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/panjf2000/ants/v2"
	"github.com/v0vc/go-music-grpc/artist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPort      = "4041"
	defaultInterface = "0.0.0.0"
	dbFile           = "./db.sqlite"
	sqlite3          = "sqlite3"
)

var (
	DownloadDir string
	wgSync      sync.WaitGroup
	pool        *ants.MultiPool
)

type server struct {
	artist.ArtistServiceServer
}

func GetArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId interface{}) (int, int) {
	stmtArt, err := tx.PrepareContext(ctx, "select art_id, userAdded from main.artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer stmtArt.Close()

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

func getArtistReleasesIdFromDb(ctx context.Context, siteId uint32, artistId string, newOnly bool) ([]string, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

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
	defer stRows.Close()

	var rows *sql.Rows
	if newOnly {
		rows, err = stRows.QueryContext(ctx, siteId)
	} else {
		rows, err = stRows.QueryContext(ctx, artistId, siteId)
	}

	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

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

func getArtistReleasesFromDb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Album, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, group_concat(ar.artistId, ',') as artIds, a.thumbnail, a.syncState from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.alb_id in (select ab.albumId from main.artistAlbum ab where ab.artistId in (select art.art_id from main.artist art where art.artistId = ? limit 1)) and ar.siteId = ? group by aa.albumId order by a.syncState desc, a.releaseDate desc;")
	if err != nil {
		log.Println(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, artistId, siteId)
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

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

func getNewReleasesFromDb(ctx context.Context, siteId uint32) ([]*artist.Album, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, group_concat(ar.artistId, ',') as artIds, a.thumbnail from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.syncState = 1 and ar.siteId = ? group by aa.albumId order by a.releaseDate desc;")
	if err != nil {
		log.Println(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, siteId)
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

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

func getArtistIdsFromDb(ctx context.Context, siteId uint32) ([]ArtistRawId, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	var artistIds []ArtistRawId

	stmtArt, err := db.PrepareContext(ctx, "select a.art_id, a.artistId from main.artist a where a.siteId = ? and a.userAdded = 1;")
	if err != nil {
		log.Println(err)
	}
	defer stmtArt.Close()

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

func getAlbumTrackFromDb(ctx context.Context, siteId uint32, albumId string) ([]*artist.Track, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select t.trk_id, t.trackId, t.title, t.hasFlac, t.hasLyric, t.quality, t.condition, t.genre, t.trackNum, t.duration from main.albumTrack at join track t on t.trk_id = at.trackId join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = at.albumId join main.artist ar on ar.art_id = aa.artistId where a.albumId = ? and ar.siteId = ? order by t.trackNum;")
	if err != nil {
		log.Println(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, albumId, siteId)
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

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

func clearSyncStateDb(ctx context.Context, siteId uint32) (int64, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=false&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	stRows, err := tx.PrepareContext(ctx, "update main.album set syncState = 0 where album.syncState = 1 and alb_id in (select distinct ab.albumId from main.artistAlbum ab where ab.artistId in (select art.art_id from main.artist art where art.siteId = ?));")
	if err != nil {
		log.Println(err)
	}
	defer stRows.Close()

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

func deleteArtistDb(ctx context.Context, siteId uint32, artistId string) (int64, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	return DeleteBase(ctx, tx, artistId, siteId, true)
}

func DeleteBase(ctx context.Context, tx *sql.Tx, artistId string, siteId uint32, isCommit bool) (int64, error) {
	artId, _ := GetArtistIdDb(tx, ctx, siteId, artistId)
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

			defer stmt.Close()
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

func vacuumDb(ctx context.Context) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v", dbFile))
	if err != nil {
		log.Println(err)
	}

	defer db.Close()
	_, err = db.ExecContext(ctx, "VACUUM;")
	if err != nil {
		log.Println(err)
	}
	_, err = db.ExecContext(ctx, "PRAGMA analysis_limit=400;")
	if err != nil {
		log.Println(err)
	}
	_, err = db.ExecContext(ctx, "PRAGMA optimize;")
	if err != nil {
		log.Println(err)
	}
}

func (*server) SyncArtist(ctx context.Context, req *artist.SyncArtistRequest) (*artist.SyncArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	log.Printf("siteId: %v, sync artist: %v started\n", siteId, artistId)

	var (
		artists []*artist.Artist
		artIds  []ArtistRawId
		err     error
	)
	if artistId == "-1" {
		artIds, err = getArtistIdsFromDb(ctx, siteId)
	} else {
		artIds = append(artIds, ArtistRawId{Id: artistId})
	}

	for _, artId := range artIds {
		wgSync.Add(1)
		_ = pool.Submit(func() {
			switch siteId {
			case 1:
				// артист со сберзвука
				art, er := SyncArtistSb(context.WithoutCancel(ctx), siteId, artId, req.GetIsAdd())
				if er == nil {
					if art != nil {
						artists = append(artists, art)
					}
				} else {
					log.Printf("Sync error: %v", er)
				}
			case 2:
				// артист со спотика
			case 3:
				// артист с дизера
			}
			wgSync.Done()
		})
	}
	wgSync.Wait()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, sync artist: %v completed\n", siteId, artistId)
	}

	return &artist.SyncArtistResponse{
		Artists: artists,
	}, nil
}

func (*server) ReadArtistAlbums(ctx context.Context, req *artist.ReadArtistAlbumRequest) (*artist.ReadArtistAlbumResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	log.Printf("siteId: %v, read releases: %v started\n", siteId, artistId)

	var (
		albums []*artist.Album
		err    error
	)

	if req.GetNewOnly() {
		albums, err = getNewReleasesFromDb(context.WithoutCancel(ctx), siteId)
	} else {
		albums, err = getArtistReleasesFromDb(context.WithoutCancel(ctx), siteId, artistId)
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, read releases: %v completed, total: %v\n", siteId, artistId, len(albums))
	}

	return &artist.ReadArtistAlbumResponse{
		Releases: albums,
	}, err
}

func (*server) SyncAlbum(ctx context.Context, req *artist.SyncAlbumRequest) (*artist.SyncAlbumResponse, error) {
	siteId := req.GetSiteId()
	albumId := req.GetAlbumId()
	log.Printf("siteId: %v, sync album %v started\n", siteId, albumId)

	var (
		tracks []*artist.Track
		err    error
	)

	wgSync.Add(1)
	_ = pool.Submit(func() {
		switch siteId {
		case 1:
			tracks, err = SyncAlbumSb(context.WithoutCancel(ctx), siteId, albumId)
		case 2:
			// "артист со спотика"
		case 3:
			// "артист с дизера"
		}
		wgSync.Done()
	})
	wgSync.Wait()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, sync album %v completed, total: %v\n", siteId, albumId, len(tracks))
	}

	return &artist.SyncAlbumResponse{
		Tracks: tracks,
	}, nil
}

func (*server) ReadAlbumTracks(ctx context.Context, req *artist.ReadAlbumTrackRequest) (*artist.ReadAlbumTrackResponse, error) {
	siteId := req.GetSiteId()
	albumId := req.GetAlbumId()
	log.Printf("siteId: %v, read album %v tracks started\n", siteId, albumId)

	var (
		tracks []*artist.Track
		err    error
	)

	tracks, err = getAlbumTrackFromDb(context.WithoutCancel(ctx), siteId, albumId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, read album %v tracks completed, total: %v\n", siteId, albumId, len(tracks))
	}

	return &artist.ReadAlbumTrackResponse{
		Tracks: tracks,
	}, err
}

func (*server) DeleteArtist(ctx context.Context, req *artist.DeleteArtistRequest) (*artist.DeleteArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	log.Printf("siteId: %v, deleting artist %v started\n", siteId, artistId)

	var (
		res int64
		err error
	)

	wgSync.Add(1)
	_ = pool.Submit(func() {
		res, err = deleteArtistDb(context.WithoutCancel(ctx), siteId, artistId)
		wgSync.Done()
	})
	wgSync.Wait()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, deleting artist %v completed\n", siteId, artistId)
	}

	return &artist.DeleteArtistResponse{RowsAffected: res}, err
}

func (*server) ClearSync(ctx context.Context, req *artist.ClearSyncRequest) (*artist.ClearSyncResponse, error) {
	siteId := req.GetSiteId()
	log.Printf("siteId: %v, clear sync state started\n", siteId)

	var (
		res int64
		err error
	)

	wgSync.Add(1)
	_ = pool.Submit(func() {
		res, err = clearSyncStateDb(context.WithoutCancel(ctx), siteId)
		wgSync.Done()
	})
	wgSync.Wait()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, clear sync state completed\n", siteId)
	}

	return &artist.ClearSyncResponse{RowsAffected: res}, err
}

func (*server) DownloadAlbums(ctx context.Context, req *artist.DownloadAlbumsRequest) (*artist.DownloadAlbumsResponse, error) {
	siteId := req.GetSiteId()
	albIds := req.GetAlbumIds()
	log.Printf("siteId: %v, download albums %v started\n", siteId, albIds)

	var (
		err     error
		resDown map[string]string
	)

	switch siteId {
	case 1:
		// mid, high, flac
		resDown, err = DownloadAlbumSb(context.WithoutCancel(ctx), siteId, albIds, req.GetTrackQuality())
	case 2:
		// "артист со спотика"
	case 3:
		// "артист с дизера"
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, download albums %v completed, total: %v\n", siteId, albIds, len(resDown))
	}

	return &artist.DownloadAlbumsResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) DownloadArtist(ctx context.Context, req *artist.DownloadArtistRequest) (*artist.DownloadAlbumsResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	log.Printf("siteId: %v, download artist %v started\n", siteId, artistId)

	var (
		err     error
		resDown map[string]string
	)

	albIds, _ := getArtistReleasesIdFromDb(ctx, siteId, artistId, false)

	switch siteId {
	case 1:
		// mid, high, flac
		resDown, err = DownloadAlbumSb(context.WithoutCancel(ctx), siteId, albIds, req.GetTrackQuality())
	case 2:
		// "артист со спотика"
	case 3:
		// "артист с дизера"
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, download artist %v completed, total: %v\n", siteId, artistId, len(resDown))
	}

	return &artist.DownloadAlbumsResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) DownloadTracks(ctx context.Context, req *artist.DownloadTracksRequest) (*artist.DownloadTracksResponse, error) {
	siteId := req.GetSiteId()
	trackIds := req.GetTrackIds()
	log.Printf("siteId: %v, download tracks %v started\n", siteId, trackIds)

	var (
		err     error
		resDown map[string]string
	)

	switch siteId {
	case 1:
		// mid, high, flac
		resDown, err = DownloadTracksSb(context.WithoutCancel(ctx), siteId, trackIds, req.GetTrackQuality())
	case 2:
		// "артист со спотика"
	case 3:
		// "артист с дизера"
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, download tracks %v completed, total: %v\n", siteId, trackIds, len(resDown))
	}

	return &artist.DownloadTracksResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) ListArtist(ctx context.Context, req *artist.ListArtistRequest) (*artist.ListArtistResponse, error) {
	siteId := req.GetSiteId()
	log.Printf("siteId: %v, list artists started\n", siteId)

	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer db.Close()

	var arts []*artist.Artist

	stmtArt, er := db.PrepareContext(context.WithoutCancel(ctx), "select ar.art_id, ar.artistId, ar.title, ar.thumbnail, count(al.alb_id) as news from main.artist ar join main.artistAlbum aa on ar.art_id = aa.artistId left outer join main.album al on aa.albumId = al.alb_id and al.syncState = 1 where ar.userAdded = 1 and ar.siteId = ? group by ar.art_id order by ar.title;")
	if er != nil {
		log.Println(er)
	}
	defer stmtArt.Close()

	rows, er := stmtArt.QueryContext(context.WithoutCancel(ctx), siteId)
	if er != nil {
		log.Println(er)
	}
	defer rows.Close()

	for rows.Next() {
		var art artist.Artist
		if e := rows.Scan(&art.Id, &art.ArtistId, &art.Title, &art.Thumbnail, &art.NewAlbs); e != nil {
			if e != nil {
				log.Println(e)
			}
		}
		art.SiteId = siteId
		arts = append(arts, &art)
	}

	log.Printf("siteId: %v, list artists completed, total: %v\n", siteId, len(arts))

	return &artist.ListArtistResponse{
		Artists: arts,
	}, err
}

func main() {
	defer ants.Release()
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("error loading .env file, use default values")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	listenInterface := os.Getenv("LISTEN")
	if listenInterface == "" {
		listenInterface = defaultInterface
	}
	DownloadDir = os.Getenv("BASEDIR")
	if DownloadDir == "" {
		DownloadDir, _ = os.UserHomeDir()
	}

	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	resAddress := listenInterface + ":" + port
	log.Println("grpc-music service started at " + resAddress)

	lis, err := net.Listen("tcp", resAddress)
	if err != nil {
		log.Printf("failed to listen: %v\n", err)
	}

	var opts []grpc.ServerOption
	newServer := grpc.NewServer(opts...)
	artist.RegisterArtistServiceServer(newServer, &server{})
	// Register reflection service on gRPC server.
	// reflection.Register(newServer)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	wg := sync.WaitGroup{}
	wg.Add(1)

	pool, _ = ants.NewMultiPool(1, 1, ants.LeastTasks)
	defer func(pool *ants.MultiPool, timeout time.Duration) {
		er := pool.ReleaseTimeout(timeout)
		if er != nil {
			log.Printf("pool release : %v", er)
		}
	}(pool, 5*time.Second)

	go func() {
		s := <-sigCh
		log.Printf("got signal %v, attempting graceful shutdown\n", s)
		vacuumDb(context.Background())
		newServer.GracefulStop()
		wg.Done()
	}()

	go func() {
		// log.Println("waiting for connections...")
		if er := newServer.Serve(lis); er != nil {
			log.Printf("failed to serve: %v", er)
		}
	}()
	wg.Wait()
	log.Println("clean shutdown")
	log.Println("end of program")
}
