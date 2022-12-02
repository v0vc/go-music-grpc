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

	// create Artist 209521227-тайпан | 211125212-yoxden | 210478992-Max Bitov | 210478991 miss di
	createArtistReq := &artist.CreateArtistRequest{
		SiteId:   1,
		ArtistId: "211125212",
	}
	fmt.Println("creating artist: " + createArtistReq.ArtistId)
	createArtistRes, err := c.CreateArtist(context.Background(), createArtistReq)
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("artist has been created: " + createArtistRes.GetTitle())

	// read artist releases
	/*	readArtistReq := &artist.ReadArtistAlbumRequest{
			Id: 83,
		}
		fmt.Printf("reading artist: %v \n", readArtistReq.GetId())
		readArtistRes, readArtistErr := c.ReadArtistAlbum(context.Background(), readArtistReq)
		if readArtistErr != nil {
			fmt.Printf("error while reading: %v \n", readArtistErr)
		}

		fmt.Printf("artist releases was read: %v \n", readArtistRes)*/

	// update Artist
	/*		newArtist := &artist.Artist{
				Id:    ArtistID,
				Title: "NEW RANDOM TITLE",
			}
			updateRes, updateErr := c.UpdateArtist(context.Background(), &artist.UpdateArtistRequest{Artist: newArtist})
			if updateErr != nil {
				fmt.Printf("error happened while updating: %v \n", updateErr)
			}
			fmt.Printf("artist was updated: %v \n", updateRes)*/

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
	/*	deleteRes, deleteErr := c.DeleteArtist(context.Background(), &artist.DeleteArtistRequest{Id: 35})
		if deleteErr != nil {
			fmt.Printf("error happened while deleting: %v \n", deleteErr)
		}
		fmt.Printf("artist was deleted, album count: %v \n", deleteRes.Id)*/
}
