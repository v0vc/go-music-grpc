package main

import (
	"context"
	"fmt"
	"github.com/v0vc/go-music-grpc/artist"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

const defaultPort = "4041"

func main() {

	fmt.Println("grpc-music client started")

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	opts := grpc.WithTransportCredentials(insecure.NewCredentials())

	cc, err := grpc.Dial("localhost:4041", opts)
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	defer cc.Close() // Maybe this should be in a separate function and the error handled?

	c := artist.NewArtistServiceClient(cc)

	// sync artist | 211850488 Мюслі UA | 212266807 Lely45
	req := &artist.SyncArtistRequest{
		SiteId:   1,
		ArtistId: "212266807",
	}
	resp, err := c.SyncArtist(context.Background(), req)
	if err != nil {
		fmt.Printf("error happened while updating: %v \n", err)
	}
	fmt.Printf("artist %v was synchronized: new artist - %d, new album - %d, deleted albums - %v, deleted artist - %v \n",
		resp.GetTitle(), len(resp.Artist), len(resp.Album), resp.DeletedAlb, resp.DeletedArt)

	// read artist releases
	/*	readArtistReq := &artist.ReadArtistAlbumRequest{
			Id: 5,
		}
		fmt.Printf("reading artist: %v \n", readArtistReq.GetId())
		readArtistRes, readArtistErr := c.ReadArtistAlbum(context.Background(), readArtistReq)
		if readArtistErr != nil {
			fmt.Printf("error while reading: %v \n", readArtistErr)
		}

		fmt.Printf("artist releases was read: %v \n", readArtistRes)*/

	// list artist
	/*fmt.Println("list artist's")
	listArtistRes, err := c.ListArtist(context.Background(), &artist.ListArtistRequest{})
	if err != nil {
		fmt.Printf("error while reading: %v \n", err)
	}

	fmt.Printf("artist releases was read: %v \n", listArtistRes.Artists)*/

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
	/*	res, err := c.DeleteArtist(context.Background(), &artist.DeleteArtistRequest{Id: 25})
		if err != nil {
			fmt.Printf("error happened while deleting: %v \n", err)
		}
		fmt.Printf("artist was deleted, album count: %v \n", res.Id)*/
}
