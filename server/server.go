package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	slices2 "golang.org/x/exp/slices"

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
	YouDir  string
	ZvukDir string
	wgSync  sync.WaitGroup
	pool    *ants.MultiPool
)

type server struct {
	artist.ArtistServiceServer
}

type ArtistRawId struct {
	isPlSync       bool
	RawId, RawPlId int
	Id, PlaylistId string
	vidIds         []string
}

func GetTokenOnlyDbWoTx(ctx context.Context, siteId uint32) string {
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
	stmt, err := tx.PrepareContext(ctx, "select token from main.site where site_id = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmt)

	var token sql.NullString
	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no token for sourceId: %d", siteId)
	case err != nil:
		log.Println(err)
	}

	return token.String
}

func GetTokenOnlyDb(tx *sql.Tx, ctx context.Context, siteId uint32) string {
	stmt, err := tx.PrepareContext(ctx, "select token from main.site where site_id = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmt)

	var token sql.NullString
	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no token for sourceId: %d", siteId)
	case err != nil:
		log.Println(err)
	}

	return token.String
}

func GetThumb(ctx context.Context, url string) []byte {
	// rkn block fix
	if strings.Contains(url, "yt3.ggpht.com") {
		url = strings.Replace(url, "yt3", "yt4", 1)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}

	response, err := http.DefaultClient.Do(req)

	if err != nil || response == nil {
		return nil
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusNotModified {
		res, er := io.ReadAll(response.Body)
		if er != nil {
			return nil
		}
		return res

	}
	return nil
}

func vacuumDb(ctx context.Context) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v", dbFile))
	if err != nil {
		log.Println(err)
	}

	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)
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
	fmt.Printf("siteId: %v, sync artist: %v started\n", siteId, artistId)

	var (
		artists    []*artist.Artist
		artIds     []ArtistRawId
		deletedIds []string
		err        error
	)

	var deletedArtIds []string
	wgSync.Add(1)
	var art *artist.Artist
	_ = pool.Submit(func() {
		switch siteId {
		case 1:
			// автор со сберзвука
			if artistId == "-1" {
				artIds, err = GetArtistIdsFromDb(ctx, siteId)
			} else {
				artIds = append(artIds, ArtistRawId{Id: artistId})
			}
			for _, artId := range artIds {
				art, deletedArtIds, err = SyncArtist(context.WithoutCancel(ctx), siteId, artId, req.GetIsAdd())
				for _, id := range deletedArtIds {
					if !slices2.Contains(deletedIds, id) {
						deletedIds = append(deletedIds, id)
					}
				}
				if err != nil {
					log.Printf("Sync error: %v", err)
				} else {
					artists = append(artists, art)
				}
			}
		case 2:
			// автор со спотика
		case 3:
			// автор с дизера
		case 4:
			// автор с ютуба
			if artistId == "-1" {
				artIds, err = GetChannelIdsFromDb(ctx, siteId)
			} else {
				artIds = append(artIds, ArtistRawId{Id: artistId, isPlSync: true})
			}

			for _, artId := range artIds {
				art, err = SyncArtistYou(context.WithoutCancel(ctx), siteId, artId, req.GetIsAdd())
				if err != nil {
					log.Printf("Sync error: %v", err)
				} else {
					artists = append(artists, art)
				}
			}
		}
		wgSync.Done()
	})

	wgSync.Wait()

	// post actions
	/*if siteId == 1 && deletedIds != nil {
		fmt.Printf("unused artists: %v\n", deletedIds)
		deletedRowCount, er := DeleteArtistsDb(context.WithoutCancel(ctx), siteId, deletedIds, false)
		if er != nil {
			log.Printf("delete unused artists failed: %v", er)
		} else {
			fmt.Printf("siteId: %v, delete unused artists completed, total : %v\n", siteId, deletedRowCount)
		}
	}*/

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"Internal error",
		)
	} else {
		var resCount int
		for _, ar := range artists {
			for range ar.Albums {
				resCount++
			}
		}
		fmt.Printf("siteId: %v, sync: %v completed, new : %v\n", siteId, artistId, resCount)
	}

	return &artist.SyncArtistResponse{
		Artists: artists,
	}, nil
}

func (*server) ReadArtistAlbums(ctx context.Context, req *artist.ReadArtistAlbumRequest) (*artist.ReadArtistAlbumResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	if artistId == "" {
		fmt.Printf("siteId: %v, read new items started\n", siteId)
	} else {
		fmt.Printf("siteId: %v, read items: %v started\n", siteId, artistId)
	}

	var (
		albums    []*artist.Album
		playlists []*artist.Playlist
		err       error
	)

	switch siteId {
	case 1:
		// автор со сберзвука
		if req.GetNewOnly() {
			albums, err = GetNewReleasesFromDb(context.WithoutCancel(ctx), siteId)
		} else {
			albums, err = GetArtistReleasesFromDb(context.WithoutCancel(ctx), siteId, artistId)
		}
	case 2:
		// автор со спотика
	case 3:
		// автор с дизера
	case 4:
		// автор с ютуба
		if req.GetNewOnly() {
			albums, err = GetNewVideosFromDb(context.WithoutCancel(ctx), siteId)
		} else {
			albums, playlists, err = GetChannelVideosFromDb(context.WithoutCancel(ctx), siteId, artistId)
		}
	}

	if err != nil {
		log.Printf("Read error: %v", err)
		return nil, status.Errorf(
			codes.Internal,
			"Internal error",
		)
	} else {
		if artistId == "" {
			fmt.Printf("siteId: %v, read new items completed, total: %v\n", siteId, len(albums))
		} else {
			fmt.Printf("siteId: %v, read items: %v completed, total: %v\n", siteId, artistId, len(albums))
		}
	}

	return &artist.ReadArtistAlbumResponse{
		Releases:  albums,
		Playlists: playlists,
	}, err
}

func (*server) DeleteArtist(ctx context.Context, req *artist.DeleteArtistRequest) (*artist.DeleteArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("siteId: %v, deleting artist %v started\n", siteId, artistId)

	var (
		res int64
		err error
	)

	wgSync.Add(1)
	_ = pool.Submit(func() {
		switch siteId {
		case 1:
			// автор со сберзвука
			res, err = DeleteArtistsDb(context.WithoutCancel(ctx), siteId, []string{artistId}, true)
		case 2:
			// автор со спотика
		case 3:
			// автор с дизера
		case 4:
			// автор с ютуба
			res, err = DeleteChannelDb(context.WithoutCancel(ctx), siteId, []string{artistId})
		}
		wgSync.Done()
	})
	wgSync.Wait()

	if err != nil {
		log.Printf("Delete error: %v", err)
		return nil, status.Errorf(
			codes.Internal,
			"Internal error",
		)
	} else {
		fmt.Printf("siteId: %v, deleting artist %v completed\n", siteId, artistId)
	}

	return &artist.DeleteArtistResponse{RowsAffected: res}, err
}

func (*server) ClearSync(ctx context.Context, req *artist.ClearSyncRequest) (*artist.ClearSyncResponse, error) {
	siteId := req.GetSiteId()
	fmt.Printf("siteId: %v, clear sync state started\n", siteId)

	var (
		res int64
		err error
	)

	wgSync.Add(1)
	_ = pool.Submit(func() {
		switch siteId {
		case 1:
			// автор со сберзвука
			res, err = ClearAlbSyncStateDb(context.WithoutCancel(ctx), siteId)
		case 2:
			// автор со спотика
		case 3:
			// автор с дизера
		case 4:
			// автор с ютуба
			res, err = ClearVidSyncStateDb(context.WithoutCancel(ctx), siteId)
		}
		wgSync.Done()
	})
	wgSync.Wait()

	if err != nil {
		log.Printf("Clear error: %v", err)
		return nil, status.Errorf(
			codes.Internal,
			"Internal error",
		)
	} else {
		fmt.Printf("siteId: %v, clear sync state completed\n", siteId)
	}

	return &artist.ClearSyncResponse{RowsAffected: res}, err
}

func (*server) DownloadAlbums(ctx context.Context, req *artist.DownloadAlbumsRequest) (*artist.DownloadAlbumsResponse, error) {
	siteId := req.GetSiteId()
	albIds := req.GetAlbumIds()
	fmt.Printf("siteId: %v, download %v started\n", siteId, albIds)
	var (
		err     error
		resDown map[string]string
	)

	switch siteId {
	case 1:
		// mid, high, flac
		resDown, err = DownloadAlbum(context.WithoutCancel(ctx), siteId, albIds, req.GetTrackQuality())
	case 2:
		// "артист со спотика"
	case 3:
		// "артист с дизера"
	case 4:
		// автор с ютуба
		resDown, err = DownloadVideos(context.WithoutCancel(ctx), albIds, req.GetTrackQuality(), req.GetIsPl())
	}

	if err != nil {
		log.Printf("Download error: %v", err)
		return nil, status.Errorf(
			codes.Internal,
			"Internal error",
		)
	} else {
		fmt.Printf("siteId: %v, download %v completed, total: %v\n", siteId, albIds, len(resDown))
	}

	return &artist.DownloadAlbumsResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) DownloadArtist(ctx context.Context, req *artist.DownloadArtistRequest) (*artist.DownloadAlbumsResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("siteId: %v, download author %v started\n", siteId, artistId)

	var (
		err     error
		resDown map[string]string
	)

	switch siteId {
	case 1:
		// mid, high, flac
		albIds, _ := GetArtistReleasesIdFromDb(ctx, siteId, artistId, false)
		resDown, err = DownloadAlbum(context.WithoutCancel(ctx), siteId, albIds, req.GetTrackQuality())
	case 2:
		// "артист со спотика"
	case 3:
		// "артист с дизера"
	case 4:
		// автор с ютуба
		vidIds, _ := GetChannelVideosIdFromDb(ctx, siteId, artistId, false)
		resDown, err = DownloadVideos(context.WithoutCancel(ctx), vidIds, req.GetTrackQuality(), false)
	}

	if err != nil {
		log.Printf("Download error: %v", err)
		return nil, status.Errorf(
			codes.Internal,
			"Internal error",
		)
	} else {
		fmt.Printf("siteId: %v, download artist %v completed, total: %v\n", siteId, artistId, len(resDown))
	}

	return &artist.DownloadAlbumsResponse{
		Downloaded: resDown,
	}, nil
}

func (*server) ListArtist(ctx context.Context, req *artist.ListArtistRequest) (*artist.ListArtistResponse, error) {
	siteId := req.GetSiteId()
	fmt.Printf("siteId: %v, list started\n", siteId)

	var (
		arts []*artist.Artist
		err  error
	)

	wgSync.Add(1)
	_ = pool.Submit(func() {
		switch siteId {
		case 1:
			// автор со сберзвука
			arts, err = GetArtists(context.WithoutCancel(ctx), siteId)
		case 2:
			// автор со спотика
		case 3:
			// автор с дизера
		case 4:
			// автор с ютуба
			arts, err = GetChannels(context.WithoutCancel(ctx), siteId)
		}
		wgSync.Done()
	})
	wgSync.Wait()

	fmt.Printf("siteId: %v, list completed, total: %v\n", siteId, len(arts))

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
	ZvukDir = os.Getenv("ZVUKDIR")
	if ZvukDir == "" {
		ZvukDir, _ = os.UserHomeDir()
	}
	YouDir = os.Getenv("YOUDIR")
	if YouDir == "" {
		YouDir, _ = os.UserHomeDir()
	}

	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	resAddress := listenInterface + ":" + port
	fmt.Println("grpc-music service started at " + resAddress)

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
		fmt.Printf("got signal %v, attempting graceful shutdown\n", s)
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
	fmt.Println("clean shutdown")
	fmt.Println("end of program")
}
