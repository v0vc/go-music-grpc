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
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}
	//delete from artist where art_id in (select artistId from (select aa.artistId, count(aa.albumId) res, a.userAdded from artistAlbum aa left join artist a on a.art_id = aa.artistId where aa.albumId in (select albumId from artistAlbum where artistId = 42) group by aa.artistId AND a.userAdded = 0));

	stmt, err := db.PrepareContext(ctx, "select count(distinct aa.artistId) res from artistAlbum aa left join artist a on a.art_id = aa.artistId where a.userAdded = 1 and aa.albumId in (select albumId from artistAlbum where artistId = ?);")
	defer stmt.Close()

	var artCount int
	err = stmt.QueryRowContext(ctx, artistId).Scan(&artCount)
	if err != nil {
		log.Fatal(err)
	}

	albumSt, err := db.PrepareContext(ctx, "delete from album where alb_id in (select albumId from (select albumId, count(albumId) res from artistAlbum where albumId in (select albumId from artistAlbum where artistId = ?) group by albumId having res = 1));")
	defer albumSt.Close()

	res, err := albumSt.ExecContext(ctx, artistId)
	if err != nil {
		log.Fatal(err)
	}

	if artCount == 1 {
		artSt, err := db.PrepareContext(ctx, "delete from artist where art_id = ?;")
		defer artSt.Close()
		if err != nil {
			log.Fatal(err)
		}
		_, err = artSt.ExecContext(ctx, artistId)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		artUpSt, err := db.PrepareContext(ctx, "update artist set userAdded = 0 where art_id = ?;")
		defer artUpSt.Close()
		if err != nil {
			log.Fatal(err)
		}
		_, err = artUpSt.ExecContext(ctx, artistId)
		if err != nil {
			log.Fatal(err)
		}
	}
	aff, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	return aff, tx.Commit()
}

func (*server) CreateArtist(ctx context.Context, req *artist.CreateArtistRequest) (*artist.CreateArtistResponse, error) {

	siteId := req.GetSiteId()
	artistId := req.GetArtistId()
	fmt.Printf("creating artist: %v \n", artistId)

	var err error
	var artistName string
	var artId int64
	// идем в бекенд в зависимости от siteId (сбер/спотик etc) и получаем остальные поля объекта и вставляем в базу
	switch siteId {
	case 1:
		artId, artistName, err = GetArtistFromSber(ctx, siteId, artistId)
	case 2:
		// "артист со спотика"
	case 3:
		// "артист с дизера"
	}

	if err != nil {
		fmt.Println(err)
	}
	/*if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}*/
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

func (*server) UpdateArtist(ctx context.Context, req *artist.UpdateArtistRequest) (*artist.UpdateArtistResponse, error) {
	fmt.Println("updating artist")
	/*Artist := req.GetArtist()
	oid, err := primitive.ObjectIDFromHex(Artist.GetId())
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Cannot parse ID"),
		)
	}*/

	// create an empty struct
	/*data := &artistItem{}*/
	/*filter := bson.M{"_id": oid}

	res := collection.FindOne(ctx, filter)
	if err := res.Decode(data); err != nil {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find Artist with specified ID: %v", err),
		)
	}*/

	// we update our internal struct
	/*data.Pid = Artist.GetPid()
	data.Name = Artist.GetName()
	data.Release = Artist.GetRelease()
	data.Description = Artist.GetDescription()

	_, updateErr := collection.ReplaceOne(context.Background(), filter, data)
	if updateErr != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Cannot update object in MongoDB: %v", updateErr),
		)
	}*/

	return &artist.UpdateArtistResponse{
		Artist: nil,
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
