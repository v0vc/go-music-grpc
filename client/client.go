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

	// create Artist
	Artist := &artist.Artist{
		SiteId:   1,
		ArtistId: "209521227",
	}
	fmt.Println("creating the artist:" + Artist.ArtistId)

	createArtistRes, err := c.CreateArtist(context.Background(), &artist.CreateArtistRequest{ArtistId: Artist.ArtistId, SiteId: Artist.SiteId})
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}
	fmt.Printf("artist has been created: %v \n", createArtistRes.GetTitle())

	// read Artist
	/*fmt.Println("reading the artist")
	readArtistReq := &artist.ReadArtistRequest{Id: ArtistID}
	readArtistRes, readArtistErr := c.ReadArtist(context.Background(), readArtistReq)
	if readArtistErr != nil {
		fmt.Printf("error happened while reading: %v \n", readArtistErr)
	}
	fmt.Printf("artist was read: %v \n", readArtistRes)

	// update Artist
	newArtist := &artist.Artist{
		Id:    ArtistID,
		Title: "NEW RANDOM TITLE",
	}
	updateRes, updateErr := c.UpdateArtist(context.Background(), &artist.UpdateArtistRequest{Artist: newArtist})
	if updateErr != nil {
		fmt.Printf("error happened while updating: %v \n", updateErr)
	}
	fmt.Printf("artist was updated: %v\n", updateRes)

	// list Artists
	stream, err := c.ListArtist(context.Background(), &artist.ListArtistRequest{})
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
	}

	// delete Artist
	deleteRes, deleteErr := c.DeleteArtist(context.Background(), &artist.DeleteArtistRequest{Id: ArtistID})
	if deleteErr != nil {
		fmt.Printf("error happened while deleting: %v \n", deleteErr)
	}
	fmt.Printf("artist was deleted: %v \n", deleteRes)*/
}
