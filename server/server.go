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

type artistItem struct {
	ID        int64
	SiteId    uint32
	ArtistId  string
	Title     string
	Counter   int
	Thumbnail []byte
	Albums    []albumItem
}

type albumItem struct {
	ID          int64
	AlbumId     string
	Title       string
	ReleaseDate string
	ReleaseType string
	Thumbnail   []byte
}

func getArtistData(data *artistItem) *artist.Artist {
	return &artist.Artist{
		Id:        data.ID,
		SiteId:    data.SiteId,
		ArtistId:  data.ArtistId,
		Title:     data.Title,
		Counter:   uint32(data.Counter),
		Thumbnail: data.Thumbnail,
	}
}

func getArtistFromDb(ctx context.Context, dbFile string, artistId int64) (*artistItem, error) {
	data := &artistItem{}

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return data, err
	}
	defer db.Close()

	rows, _ := db.QueryContext(ctx, "select * from artist where id=? limit 1", artistId)
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&data.ID, &data.SiteId, &data.ArtistId, &data.Title, &data.Counter, &data.Thumbnail)
	}

	return data, nil
}

func deleteArtistDb(ctx context.Context, dbFile string, artistId int64) (int64, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return -1, err
	}

	stmt, err := db.PrepareContext(ctx, "delete from artist where id=?")
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, artistId)
	if err != nil {
		return -1, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return -1, err
	}

	return affected, nil
}

func (*server) CreateArtist(ctx context.Context, req *artist.CreateArtistRequest) (*artist.CreateArtistResponse, error) {

	item := artistItem{
		SiteId:   req.GetSiteId(),
		ArtistId: req.GetArtistId(),
	}
	fmt.Println("creating artist: " + item.ArtistId)

	var err error
	var artistName string
	// идем в бекенд в зависимости от siteId (сбер/спотик etc) и получаем остальные поля объекта и вставляем в базу
	switch item.SiteId {
	case 1:
		artistName, err = GetArtistFromSber(ctx, &item)
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
	fmt.Println("artist has been created: " + artistName)
	return &artist.CreateArtistResponse{
		Title: artistName,
	}, nil
}

func (*server) ReadArtist(ctx context.Context, req *artist.ReadArtistRequest) (*artist.ReadArtistResponse, error) {
	fmt.Println("read artist")

	data, err := getArtistFromDb(ctx, dbFile, req.GetId())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	return &artist.ReadArtistResponse{
		Artist: getArtistData(data),
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
	data := &artistItem{}
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
		Artist: getArtistData(data),
	}, nil

}

func (*server) DeleteArtist(ctx context.Context, req *artist.DeleteArtistRequest) (*artist.DeleteArtistResponse, error) {
	fmt.Println("deleting artist")

	res, err := deleteArtistDb(ctx, dbFile, req.GetId())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("cannot delete artist in DB: %v", err),
		)
	}
	if res == 0 {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("cannot find artist in DB: %v", err),
		)
	}
	return &artist.DeleteArtistResponse{Id: res}, nil
}

func (*server) ListArtist(_ *artist.ListArtistRequest, stream artist.ArtistService_ListArtistServer) error {
	fmt.Println("list artist's")

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Unknown internal error: %v", err),
		)
	}

	rows, err := db.Query("select * from artist")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Unknown internal error: %v", err),
		)
	}
	defer rows.Close()

	for rows.Next() {
		data := &artistItem{}
		rows.Scan(&data.ID, &data.SiteId, &data.ArtistId, &data.Title, &data.Counter, &data.Thumbnail)
		err = stream.Send(&artist.ListArtistResponse{Artist: getArtistData(data)})
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
			fmt.Sprintf("Unknown internal error: %v", err),
		)
	}

	// Check for row scan error.
	if err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Unknown internal error: %v", err),
		)
	}

	// Check for errors during row iteration.
	if err = rows.Err(); err != nil {
		return status.Errorf(
			codes.Internal,
			fmt.Sprintf("Unknown internal error: %v", err),
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
