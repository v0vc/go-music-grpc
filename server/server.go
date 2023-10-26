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
	"sync"
	"time"

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
)

var DownloadDir string

type server struct {
	artist.ArtistServiceServer
}

func GetArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId interface{}) (int, int) {
	stmtArt, err := tx.PrepareContext(ctx, "select art_id, userAdded from main.artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtArt.Close()

	var (
		artRawId  int
		userAdded int
	)

	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId, &userAdded)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		fmt.Printf("no artist with id %v \n", artistId)
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Printf("artist db id is %d \n", artRawId)
	}
	return artRawId, userAdded
}

func getArtistReleasesIdFromDb(ctx context.Context, siteId uint32, artistId string) ([]string, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select a.albumId from main.artistAlbum aa inner join main.album a on a.alb_id = aa.albumId inner join main.artist ar on ar.art_id = aa.artistId where ar.artistId = ? and ar.siteId = ?;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, artistId, siteId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var albIds []string
	for rows.Next() {
		var alb string
		if err = rows.Scan(&alb); err != nil {
			log.Fatal(err)
		}
		albIds = append(albIds, alb)
	}

	return albIds, nil
}

func getArtistReleasesFromDb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Album, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, a.thumbnail, a.syncState from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.alb_id in (select ab.albumId from main.artistAlbum ab where ab.artistId in (select art.art_id from main.artist art where art.artistId = ? limit 1)) and ar.siteId = ? group by aa.albumId order by a.syncState desc, a.releaseDate desc;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, artistId, siteId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var albs []*artist.Album
	for rows.Next() {
		var alb artist.Album
		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.ReleaseDate, &alb.ReleaseType, &alb.SubTitle, &alb.Thumbnail, &alb.SyncState); err != nil {
			log.Fatal(err)
		}
		albs = append(albs, &alb)
	}

	return albs, nil
}

func getNewReleasesFromDb(ctx context.Context, siteId uint32) ([]*artist.Album, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, group_concat(ar.title, ', ') as subTitle, a.thumbnail from main.artistAlbum aa join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.syncState = 1 and ar.siteId = ? group by aa.albumId order by a.releaseDate desc;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, siteId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var albs []*artist.Album
	for rows.Next() {
		var alb artist.Album
		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.ReleaseDate, &alb.ReleaseType, &alb.SubTitle, &alb.Thumbnail); err != nil {
			log.Fatal(err)
		}
		albs = append(albs, &alb)
	}

	return albs, nil
}

func getAlbumTrackFromDb(ctx context.Context, siteId uint32, albumId string) ([]*artist.Track, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	stRows, err := db.PrepareContext(ctx, "select t.trk_id, t.trackId, t.title, t.hasFlac, t.hasLyric, t.quality, t.condition, t.genre, t.trackNum, t.duration from main.albumTrack at join track t on t.trk_id = at.trackId join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = at.albumId join main.artist ar on ar.art_id = aa.artistId where a.albumId = ? and ar.siteId = ? order by t.trackNum;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRows.Close()

	rows, err := stRows.QueryContext(ctx, albumId, siteId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var tracks []*artist.Track
	for rows.Next() {
		var track artist.Track
		if err = rows.Scan(&track.Id, &track.TrackId, &track.Title, &track.HasFlac, &track.HasLyric, &track.Quality, &track.Condition, &track.Genre, &track.TrackNum, &track.Duration); err != nil {
			log.Fatal(err)
		}
		tracks = append(tracks, &track)
	}

	return tracks, nil
}

func deleteArtistDb(ctx context.Context, siteId uint32, artistId string) (int64, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}
	artId, _ := GetArtistIdDb(tx, ctx, siteId, artistId)
	var aff int64
	execs := []struct {
		stmt string
		res  int
	}{
		{stmt: fmt.Sprintf("create temporary table _temp_album as select albumId from (select aa.albumId, count(aa.albumId) res from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = %d) and a.userAdded = 1 group by aa.albumId having res = 1);", artId), res: 0},
		{stmt: fmt.Sprintf("create temporary table _temp_artist as select aa.artistId, a.userAdded from main.artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from main.artistAlbum where artistId = %v) group by aa.artistId;", artistId), res: 1},
		{stmt: "select count(1) from _temp_artist where userAdded = 1;", res: 2},
		{stmt: "delete from artist where art_id in (select artistId from _temp_artist where userAdded = 0);", res: 3},
		{stmt: "delete from track where trk_id in (select trackId from main.albumTrack where albumId in (select albumId from _temp_album));", res: 4},
		{stmt: "delete from album where alb_id in (select albumId from _temp_album);", res: 5},
		{stmt: "drop table _temp_album;", res: 6},
		{stmt: "drop table _temp_artist;", res: 7},
	}

	for _, exec := range execs {
		stmt, err := tx.PrepareContext(ctx, exec.stmt)
		if err != nil {
			log.Fatal(err)
		}

		if exec.res == 2 {
			var artCount int
			err = stmt.QueryRowContext(ctx).Scan(&artCount)
			if err != nil {
				log.Fatal(err)
			}
			if artCount == 1 {
				artSt, err := tx.PrepareContext(ctx, "delete from main.artist where art_id = ?;")
				if err != nil {
					log.Fatal(err)
				}
				_, err = artSt.ExecContext(ctx, artId)
				if err != nil {
					log.Fatal(err)
				}
				artSt.Close()
			} else {
				artUpSt, err := tx.PrepareContext(ctx, "update main.artist set userAdded = 0 where art_id = ?;")
				if err != nil {
					log.Fatal(err)
				}
				_, err = artUpSt.ExecContext(ctx, artId)
				if err != nil {
					log.Fatal(err)
				}
				artUpSt.Close()
			}
		} else {
			cc, err := stmt.ExecContext(ctx)
			if err != nil {
				log.Fatal(err)
			}
			if exec.res == 4 {
				aff, _ = cc.RowsAffected()
			}
		}
		stmt.Close()
	}

	return aff, tx.Commit()
}

func (*server) SyncArtist(ctx context.Context, req *artist.SyncArtistRequest) (*artist.SyncArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("siteId: %v, sync artist: %v started \n", siteId, artistId)
	start := time.Now()

	var (
		newArtists       []*artist.Artist
		newAlbums        []*artist.Album
		deletedAlbumIds  []string
		deletedArtistIds []string
		err              error
	)

	switch siteId {
	case 1:
		newArtists, newAlbums, deletedAlbumIds, deletedArtistIds, err = SyncArtistSb(ctx, siteId, artistId)
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
		fmt.Printf("siteId: %v, sync artist: %v finished in %v \n", siteId, artistId, time.Since(start))
	}

	return &artist.SyncArtistResponse{
		Artists:    newArtists,
		Albums:     newAlbums,
		DeletedAlb: deletedAlbumIds,
		DeletedArt: deletedArtistIds,
	}, nil
}

func (*server) ReadArtistAlbums(ctx context.Context, req *artist.ReadArtistAlbumRequest) (*artist.ReadArtistAlbumResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("siteId: %v, read artist releases: %v started \n", siteId, artistId)
	start := time.Now()

	albums, err := getArtistReleasesFromDb(ctx, siteId, artistId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		fmt.Printf("siteId: %v, read artist releases: %v finished in %v, total: %v \n", siteId, artistId, time.Since(start), len(albums))
	}

	return &artist.ReadArtistAlbumResponse{
		Releases: albums,
	}, err
}

func (*server) ReadNewAlbums(ctx context.Context, req *artist.ListArtistRequest) (*artist.ReadArtistAlbumResponse, error) {
	siteId := req.GetSiteId()
	fmt.Printf("siteId: %v, read new releases started \n", siteId)
	start := time.Now()

	albums, err := getNewReleasesFromDb(ctx, siteId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		fmt.Printf("siteId: %v, read new releases finished in %v, total: %v \n", siteId, time.Since(start), len(albums))
	}

	return &artist.ReadArtistAlbumResponse{
		Releases: albums,
	}, err
}

func (*server) SyncAlbum(ctx context.Context, req *artist.SyncAlbumRequest) (*artist.SyncAlbumResponse, error) {
	siteId := req.GetSiteId()
	albumId := req.GetAlbumId()
	fmt.Printf("siteId: %v, sync album %v started \n", siteId, albumId)
	start := time.Now()

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
		fmt.Printf("siteId: %v, sync album %v finished in %v, total: %v \n", siteId, albumId, time.Since(start), len(tracks))
	}

	return &artist.SyncAlbumResponse{
		Tracks: tracks,
	}, nil
}

func (*server) ReadAlbumTracks(ctx context.Context, req *artist.ReadAlbumTrackRequest) (*artist.ReadAlbumTrackResponse, error) {
	siteId := req.GetSiteId()
	albumId := req.GetAlbumId()
	fmt.Printf("siteId: %v, read album %v tracks started \n", siteId, albumId)
	start := time.Now()
	tracks, err := getAlbumTrackFromDb(ctx, siteId, albumId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		fmt.Printf("siteId: %v, read album %v tracks finished in %v, total: %v \n", siteId, albumId, time.Since(start), len(tracks))
	}

	return &artist.ReadAlbumTrackResponse{
		Tracks: tracks,
	}, err
}

func (*server) DeleteArtist(ctx context.Context, req *artist.DeleteArtistRequest) (*artist.DeleteArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("siteId: %v, deleting artist %v started \n", siteId, artistId)
	start := time.Now()
	res, err := deleteArtistDb(ctx, siteId, artistId)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	} else {
		fmt.Printf("siteId: %v, deleting artist %v finished in %v \n", siteId, artistId, time.Since(start))
	}

	return &artist.DeleteArtistResponse{RowsAffected: res}, err
}

func (*server) DownloadAlbums(ctx context.Context, req *artist.DownloadAlbumsRequest) (*artist.DownloadAlbumsResponse, error) {
	siteId := req.GetSiteId()
	albIds := req.GetAlbumIds()
	fmt.Printf("siteId: %v, download albums %v started \n", siteId, albIds)
	start := time.Now()

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
		fmt.Printf("siteId: %v, download albums %v finished in %v, total: %v \n", siteId, albIds, time.Since(start), len(resDown))
	}

	return &artist.DownloadAlbumsResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) DownloadArtist(ctx context.Context, req *artist.DownloadArtistRequest) (*artist.DownloadAlbumsResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("siteId: %v, download artist %v started \n", siteId, artistId)
	start := time.Now()

	var (
		err     error
		resDown map[string]string
	)

	albIds, err := getArtistReleasesIdFromDb(ctx, siteId, artistId)
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
		fmt.Printf("siteId: %v, download artist %v finished in %v, total: %v \n", siteId, artistId, time.Since(start), len(resDown))
	}

	return &artist.DownloadAlbumsResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) DownloadTracks(ctx context.Context, req *artist.DownloadTracksRequest) (*artist.DownloadTracksResponse, error) {
	siteId := req.GetSiteId()
	trackIds := req.GetTrackIds()
	fmt.Printf("siteId: %v, download tracks %v started \n", siteId, trackIds)
	start := time.Now()

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
		fmt.Printf("siteId: %v, download tracks %v finished in %v, total: %v \n", siteId, trackIds, time.Since(start), len(resDown))
	}

	return &artist.DownloadTracksResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) ListArtist(ctx context.Context, req *artist.ListArtistRequest) (*artist.ListArtistResponse, error) {
	siteId := req.GetSiteId()
	fmt.Printf("siteId: %v, get artists started \n", siteId)
	start := time.Now()
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer db.Close()

	stmtArt, err := db.PrepareContext(ctx, "select ar.art_id, ar.artistId, ar.title, ar.thumbnail, count(al.alb_id) as news from artist ar inner join artistAlbum aa on ar.art_id = aa.artistId left outer join album al on aa.albumId = al.alb_id and al.syncState = 1 where ar.userAdded = 1 and ar.siteId = ? group by ar.art_id order by ar.title;")
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
	fmt.Printf("siteId: %v, get artists finished in %v, total: %v \n", siteId, time.Since(start), len(arts))
	return &artist.ListArtistResponse{
		Artists: arts,
	}, err
}

func (*server) ListArtistStream(req *artist.ListArtistStreamRequest, stream artist.ArtistService_ListArtistStreamServer) error {
	siteId := req.GetSiteId()
	fmt.Printf("list artist's: %v \n", siteId)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer db.Close()

	stmtArt, err := db.Prepare("select ar.art_id, ar.artistId, ar.title, ar.thumbnail, count(al.alb_id) as news from artist ar inner join artistAlbum aa on ar.art_id = aa.artistId left outer join album al on aa.albumId = al.alb_id and al.syncState = 1 where ar.userAdded = 1 and ar.siteId = ? group by ar.art_id order by ar.title;")
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
	fmt.Println("grpc-music service started at " + resAddress)

	lis, err := net.Listen("tcp", resAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
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
		fmt.Printf("got signal %v, attempting graceful shutdown \n", s)
		newServer.GracefulStop()
		wg.Done()
	}()

	go func() {
		// fmt.Println("waiting for connections...")
		if err := newServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	wg.Wait()
	fmt.Println("clean shutdown")
	fmt.Println("end of program")
}
