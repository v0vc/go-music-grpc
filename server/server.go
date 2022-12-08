package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/v0vc/go-music-grpc/artist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"os"
	"os/signal"
)

const defaultPort = "4041"
const defaultInterface = "0.0.0.0"
const dbFile = "./db/db.sqlite"

type server struct {
	artist.ArtistServiceServer
}

func getArtistReleasesFromDb(ctx context.Context, artistId int64) ([]*artist.Album, error) {
	var albs []*artist.Album
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "select a.alb_id, a.title, a.albumId, a.releaseDate, a.releaseType, a.thumbnail from artistAlbum aa inner join album a on a.alb_id = aa.albumId  where aa.artistId = ?", artistId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var alb artist.Album
		if err := rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.ReleaseDate, &alb.ReleaseType, &alb.Thumbnail); err != nil {
			return nil, err
		}
		albs = append(albs, &alb)
	}

	return albs, nil
}

func deleteArtistDb(ctx context.Context, artistId int64) (int64, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	var aff int64
	execs := []struct {
		stmt string
		res  int
	}{
		{stmt: fmt.Sprintf("create temporary table _temp_album as select albumId from (select aa.albumId, count(aa.albumId) res from artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from artistAlbum where artistId = %d) and a.userAdded = 1 group by aa.albumId having res = 1);", artistId), res: 0},
		{stmt: fmt.Sprintf("create temporary table _temp_artist as select aa.artistId, a.userAdded from artistAlbum aa join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from artistAlbum where artistId = %d) group by aa.artistId;", artistId), res: 1},
		{stmt: "select count(1) from _temp_artist where userAdded = 1;", res: 2},
		{stmt: "delete from artist where art_id in (select artistId from _temp_artist where userAdded = 0);", res: 3},
		{stmt: "delete from album where alb_id in (select albumId from _temp_album);", res: 4},
		{stmt: "drop table _temp_album;", res: 5},
		{stmt: "drop table _temp_artist;", res: 6},
	}

	for _, exec := range execs {
		stmt, err := db.PrepareContext(ctx, exec.stmt)
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
				artSt, err := db.PrepareContext(ctx, "delete from artist where art_id = ?;")
				if err != nil {
					log.Fatal(err)
				}
				_, err = artSt.ExecContext(ctx, artistId)
				if err != nil {
					log.Fatal(err)
				}
				artSt.Close()
			} else {
				artUpSt, err := db.PrepareContext(ctx, "update artist set userAdded = 0 where art_id = ?;")
				if err != nil {
					log.Fatal(err)
				}
				_, err = artUpSt.ExecContext(ctx, artistId)
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

func (*server) CreateArtist(ctx context.Context, req *artist.CreateArtistRequest) (*artist.CreateArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("creating artist: %v \n", artistId)

	var artistName string
	var artId int64
	var err error

	// идем в бекенд в зависимости от siteId (сбер/спотик etc) и получаем остальные поля объекта и вставляем в базу
	switch siteId {
	case 1:
		artId, artistName, err = GetArtistSb(ctx, siteId, artistId)
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
	}
	fmt.Printf("artist has been created: %v \n", artistName)
	return &artist.CreateArtistResponse{
		Title: artistName,
		Id:    artId,
	}, err
}

func (*server) ReadArtistAlbum(ctx context.Context, req *artist.ReadArtistAlbumRequest) (*artist.ReadArtistAlbumResponse, error) {
	fmt.Printf("read artist releases: %v \n", req.GetId())
	albums, err := getArtistReleasesFromDb(ctx, req.GetId())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	return &artist.ReadArtistAlbumResponse{
		Releases: albums,
	}, err
}

func (*server) SyncArtist(ctx context.Context, req *artist.SyncArtistRequest) (*artist.SyncArtistResponse, error) {
	siteId := req.GetSiteId()
	artistId := req.GetId()
	fmt.Printf("sync artist: %v \n", artistId)

	var newArtists []*artist.Artist
	var newAlbums []*artist.Album
	var deletedAlbums []string
	var err error

	switch siteId {
	case 1:
		newArtists, newAlbums, deletedAlbums, err = SyncArtistSb(ctx, siteId, artistId)
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
	}

	return &artist.SyncArtistResponse{
		Artist:  newArtists,
		Album:   newAlbums,
		Deleted: deletedAlbums,
	}, nil
}

func (*server) DeleteArtist(ctx context.Context, req *artist.DeleteArtistRequest) (*artist.DeleteArtistResponse, error) {
	fmt.Printf("deleting artist: %v \n", req)
	res, err := deleteArtistDb(ctx, req.GetId())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	return &artist.DeleteArtistResponse{Id: res}, err
}

func (*server) ListArtist(ctx context.Context, _ *artist.ListArtistRequest) (*artist.ListArtistResponse, error) {
	fmt.Println("list artist's")
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "select art_id, siteId, artistId, title, counter, thumbnail from artist where userAdded = 1")
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
		if err := rows.Scan(&art.Id, &art.SiteId, &art.ArtistId, &art.Title, &art.Counter, &art.Thumbnail); err != nil {
			return nil, status.Errorf(
				codes.Internal,
				fmt.Sprintf("Internal error: %v", err),
			)
		}
		arts = append(arts, &art)
	}

	return &artist.ListArtistResponse{
		Artists: arts,
	}, err
}

func (*server) ListStreamArtist(_ *artist.ListStreamArtistRequest, stream artist.ArtistService_ListStreamArtistServer) error {
	fmt.Println("list artist's")
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer db.Close()

	rows, err := db.Query("select art_id, siteId, artistId, title, counter, thumbnail from artist where userAdded = 1")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	defer rows.Close()

	for rows.Next() {
		var art artist.Artist
		if err := rows.Scan(&art.Id, &art.SiteId, &art.ArtistId, &art.Title, &art.Counter, &art.Thumbnail); err != nil {
			return status.Errorf(
				codes.Internal,
				fmt.Sprintf("error while getting data from DB: %v", err),
			)
		}
		err = stream.Send(&artist.ListStreamArtistResponse{Artist: &art})
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
		log.Fatal("error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	listenInterface := os.Getenv("LISTEN")
	if listenInterface == "" {
		listenInterface = defaultInterface
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
	s := grpc.NewServer(opts...)
	artist.RegisterArtistServiceServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)

	go func() {
		fmt.Println("starting server...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// block until a signal is received
	<-ch
	// close the connection maybe

	// finally, we stop the server
	fmt.Println("stopping the server")
	s.Stop()
	fmt.Println("end of program")
}
