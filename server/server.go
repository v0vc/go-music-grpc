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

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
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

var DownloadDir string

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

func getArtistReleasesIdFromDb(ctx context.Context, siteId uint32, artistId string) []string {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select a.albumId from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where ar.artistId = ? and ar.siteId = ?;")
	if err != nil {
		log.Println(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, artistId, siteId)
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

	return albIds
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
	artId, _ := GetArtistIdDb(tx, ctx, siteId, artistId)
	execs := []struct {
		stmt string
		res  int
	}{
		{stmt: fmt.Sprintf("update main.artist set userAdded = 0 where art_id = %d", artId), res: 0},
		{stmt: fmt.Sprintf("create temporary table _temp_album as select albumId from (select aa.albumId, count(aa.artistId) res from main.artistAlbum aa where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) group by aa.albumId having res = 1) union select albumId from (select aa.albumId, count(aa.artistId) res from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) group by aa.albumId having res > 1) except select aa.albumId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = (select art_id from main.artist where artistId = %v and siteId = 1 limit 1)) and a.userAdded = 1 group by aa.albumId;", artId, artId, artistId), res: 1},
		{stmt: fmt.Sprintf("delete from main.artist where art_id in (select aa.artistId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId = %d) group by aa.artistId except select aa.artistId from main.artistAlbum aa where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId in (select aa.artistId from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select aa.albumId from main.artistAlbum aa where aa.artistId = %d) and a.userAdded = 1 and a.art_id <> %d group by aa.artistId)) group by aa.artistId);", artId, artId, artId), res: 2},
		{stmt: "delete from main.album where alb_id in (select albumId from _temp_album);", res: 3},
		{stmt: "drop table _temp_album;", res: 4},
	}

	var aff int64

	for _, exec := range execs {
		func() {
			stmt, er := tx.PrepareContext(ctx, exec.stmt)
			if er != nil {
				log.Println(er)
			}

			defer stmt.Close()
			cc, er := stmt.ExecContext(ctx)
			if er != nil {
				log.Println(err)
			} else if exec.res == 3 {
				aff, err = cc.RowsAffected()
				if err != nil {
					log.Println(err)
				}
			}
		}()
	}
	return aff, tx.Commit()
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
		switch siteId {
		case 1:
			art, er := SyncArtistSb(ctx, siteId, artId, req.GetIsAdd())
			if er == nil {
				if art != nil {
					artists = append(artists, art)
				}
			} else {
				log.Printf("Sync error: %v", er)
			}
		case 2:
			// "артист со спотика"
		case 3:
			// "артист с дизера"
		}
	}

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
	log.Printf("siteId: %v, read artist releases: %v started\n", siteId, artistId)

	albums, err := getArtistReleasesFromDb(ctx, siteId, artistId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, read artist releases: %v completed, total: %v\n", siteId, artistId, len(albums))
	}

	return &artist.ReadArtistAlbumResponse{
		Releases: albums,
	}, err
}

func (*server) ReadNewAlbums(ctx context.Context, req *artist.ListArtistRequest) (*artist.ReadArtistAlbumResponse, error) {
	siteId := req.GetSiteId()
	log.Printf("siteId: %v, read new releases started\n", siteId)

	albums, err := getNewReleasesFromDb(ctx, siteId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		log.Printf("siteId: %v, read new releases completed, total: %v\n", siteId, len(albums))
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

	switch siteId {
	case 1:
		tracks, err = SyncAlbumSb(ctx, siteId, albumId)
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

	tracks, err := getAlbumTrackFromDb(ctx, siteId, albumId)

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

	res, err := deleteArtistDb(ctx, siteId, artistId)

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

	res, err := clearSyncStateDb(ctx, siteId)

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
		resDown, err = DownloadAlbumSb(ctx, siteId, albIds, req.GetTrackQuality())
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

	albIds := getArtistReleasesIdFromDb(ctx, siteId, artistId)

	switch siteId {
	case 1:
		// mid, high, flac
		resDown, err = DownloadAlbumSb(ctx, siteId, albIds, req.GetTrackQuality())
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
		resDown, err = DownloadTracksSb(ctx, siteId, trackIds, req.GetTrackQuality())
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

	stmtArt, err := db.PrepareContext(ctx, "select ar.art_id, ar.artistId, ar.title, ar.thumbnail, count(al.alb_id) as news from main.artist ar join main.artistAlbum aa on ar.art_id = aa.artistId left outer join main.album al on aa.albumId = al.alb_id and al.syncState = 1 where ar.userAdded = 1 and ar.siteId = ? group by ar.art_id order by ar.title;")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer stmtArt.Close()

	rows, err := stmtArt.QueryContext(ctx, siteId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer rows.Close()

	var arts []*artist.Artist

	for rows.Next() {
		var art artist.Artist
		if er := rows.Scan(&art.Id, &art.ArtistId, &art.Title, &art.Thumbnail, &art.NewAlbs); er != nil {
			return nil, status.Errorf(
				codes.Internal,
				fmt.Sprintf("Internal error: %v", er),
			)
		}
		art.SiteId = siteId
		arts = append(arts, &art)
	}
	log.Printf("siteId: %v, list artists completed, total: %v\n", siteId, len(arts))

	return &artist.ListArtistResponse{
		Artists: arts,
	}, err
}

func (*server) ListArtistStream(req *artist.ListArtistRequest, stream artist.ArtistService_ListArtistStreamServer) error {
	siteId := req.GetSiteId()
	log.Printf("siteId: %v, list artists stream started\n", siteId)

	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	defer db.Close()
	stmtArt, err := db.Prepare("select ar.art_id, ar.artistId, ar.title, ar.thumbnail, count(al.alb_id) as news from main.artist ar join main.artistAlbum aa on ar.art_id = aa.artistId left outer join main.album al on aa.albumId = al.alb_id and al.syncState = 1 where ar.userAdded = 1 and ar.siteId = ? group by ar.art_id order by ar.title;")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	defer stmtArt.Close()
	rows, err := stmtArt.Query(siteId)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	defer rows.Close()

	for rows.Next() {
		var art artist.Artist
		if er := rows.Scan(&art.Id, &art.ArtistId, &art.Title, &art.Thumbnail, &art.NewAlbs); er != nil {
			return status.Errorf(
				codes.Internal,
				fmt.Sprintf("error while getting data from DB: %v", er),
			)
		}
		art.SiteId = siteId
		err = stream.Send(&artist.ListArtistStreamResponse{Artist: &art})
		if err != nil {
			return status.Errorf(
				codes.Internal,
				fmt.Sprintf("error while getting data from DB: %v", err),
			)
		}
	}
	// Check for errors during rows "Close".
	// This may be more important if multiple statements are executed
	// in a single batch and rows were written as well as read.
	if closeErr := rows.Close(); closeErr != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	// Check for row scan error.
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	return nil
}

func main() {
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

	go func() {
		s := <-sigCh
		log.Printf("got signal %v, attempting graceful shutdown\n", s)
		vacuumDb(context.Background())
		newServer.GracefulStop()
		wg.Done()
	}()

	go func() {
		// fmt.Println("waiting for connections...")
		if er := newServer.Serve(lis); er != nil {
			log.Printf("failed to serve: %v", er)
		}
	}()
	wg.Wait()
	log.Println("clean shutdown")
	log.Println("end of program")
}
