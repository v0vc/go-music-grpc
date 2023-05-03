package main

import (
	"context"
	"fmt"
	"github.com/v0vc/go-music-grpc/artist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
)

const defaultPort = "4041"

func main() {

	fmt.Println("grpc-music client started")

	/*	err := godotenv.Load(".env")
		if err != nil {
			log.Fatal("Error loading .env file")
		}

		port := os.Getenv("PORT")
		if port == "" {
			port = defaultPort
		}*/

	opts := grpc.WithTransportCredentials(insecure.NewCredentials())

	cc, err := grpc.Dial("localhost:4041", opts)
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	defer cc.Close() // Maybe this should be in a separate function and the error handled?

	c := artist.NewArtistServiceClient(cc)
	//c := getClientInstance()

	// sync artist | 211850488 Мюслі UA | 212266807 Lely45
	/*req := &artist.SyncArtistRequest{
		SiteId:   1,
		ArtistId: "211850488",
	}
	resp, err := c.SyncArtist(context.Background(), req)
	if err != nil {
		fmt.Printf("error happened while updating: %v \n", err)
	}
	fmt.Printf("artist was synchronized: new artist - %d, new album - %d, deleted albums - %v, deleted artist - %v \n",
		len(resp.Artists), len(resp.Albums), resp.DeletedAlb, resp.DeletedArt)*/

	// read artist releases
	/*readArtistReq := &artist.ReadArtistAlbumRequest{
		SiteId:   1,
		ArtistId: "212266807",
	}
	fmt.Printf("reading artist: %v \n", readArtistReq.GetArtistId())
	readArtistRes, readArtistErr := c.ReadArtistAlbums(context.Background(), readArtistReq)
	if readArtistErr != nil {
		fmt.Printf("error while reading: %v \n", readArtistErr)
	}

	fmt.Printf("artist releases was read: %v \n", readArtistRes)*/

	// list artist
	fmt.Println("list artist's")
	listArtistRes, err := c.ListArtist(context.Background(), &artist.ListArtistRequest{SiteId: 1})
	if err != nil {
		fmt.Printf("error while reading: %v \n", err)
	}

	fmt.Printf("artist releases was read: %v \n", listArtistRes.Artists)

	// list Artists Stream
	/*stream, err := c.ListStreamArtist(context.Background(), &artist.ListStreamArtistRequest{})
	if err != nil {
		log.Fatalf("error while calling ListArtist RPC: %v", err)
	}
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("something happened: %v", err)
		}
		fmt.Println(res.GetArtist())
	}*/

	// delete Artist
	/*	res, err := c.DeleteArtist(context.Background(), &artist.DeleteArtistRequest{
			SiteId:   1,
			ArtistId: "212266807",
		})
		if err != nil {
			fmt.Printf("error happened while deleting: %v \n", err)
		}
		fmt.Printf("artist was deleted, album count: %v \n", res.RowsAffected)*/

	// sync album
	/*req := &artist.SyncAlbumRequest{
		SiteId:  1,
		AlbumId: "16026887",
	}
	resp, err := c.SyncAlbum(context.Background(), req)
	if err != nil {
		fmt.Printf("error happened while updating: %v \n", err)
	}
	fmt.Printf("album %v was synchronized: tracks - %d \n", req.GetAlbumId(), len(resp.Tracks))*/

	// read album tracks
	/*req := &artist.ReadAlbumTrackRequest{
		SiteId:  1,
		AlbumId: "16026887",
	}
	fmt.Printf("reading album: %v \n", req.GetAlbumId())
	resp, err := c.ReadAlbumTracks(context.Background(), req)
	if err != nil {
		fmt.Printf("error while reading: %v \n", err)
	}

	fmt.Printf("album tracks was read: %v \n", resp)*/

	// download album tracks (mid, high, flac)
	/*req := &artist.DownloadTracksRequest{
		SiteId:       1,
		TrackIds:     []string{"89734686", "89734684", "126268642"},
		TrackQuality: "flac",
	}
	fmt.Printf("download tracks: %v \n", req.GetTrackIds())
	resp, err := c.DownloadTracks(context.Background(), req)
	if err != nil {
		fmt.Printf("error while downloading: %v \n", err)
	}
	for trackId, dSize := range resp.Downloaded {
		fmt.Printf("track %v was downloaded: %v \n", trackId, dSize)
	}*/

	// download albums (mid, high, flac)
	/*	req := &artist.DownloadAlbumsRequest{
			SiteId:       1,
			AlbumIds:     []string{"7356267", "16026887"},
			TrackQuality: "flac",
		}
		fmt.Printf("download albums: %v \n", req.GetAlbumIds())
		resp, err := c.DownloadAlbums(context.Background(), req)
		if err != nil {
			fmt.Printf("error while downloading: %v \n", err)
		}
		for trackId, dSize := range resp.Downloaded {
			fmt.Printf("track %v was downloaded: %v \n", trackId, dSize)
		}*/
}
