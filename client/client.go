package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/v0vc/go-music-grpc/artist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("running in " + curDir)

	log.Println("grpc-music client started")

	opts := grpc.WithTransportCredentials(insecure.NewCredentials())

	cc, err := grpc.NewClient("localhost:4041", opts)
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	defer func(cc *grpc.ClientConn) {
		er := cc.Close()
		if er != nil {
			log.Println(er)
		}
	}(cc)

	c := artist.NewArtistServiceClient(cc)

	idsFile, err := os.OpenFile(filepath.Join(curDir, "ids.txt"), os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
	rd := bufio.NewReader(idsFile)
	for {
		line, er := rd.ReadString('\n')
		if er != nil {
			if er == io.EOF {
				break
			}
			log.Fatalf("read file line error: %v", er)
		}

		id := strings.Trim(strings.TrimSpace(line), "\"")
		fmt.Printf("started %v \n", id)
		fmt.Println(id)
		req := &artist.SyncArtistRequest{
			SiteId:   1,
			ArtistId: id,
			IsAdd:    true,
		}
		_, er = c.SyncArtist(context.Background(), req)
		if er != nil {
			fmt.Printf("error happened: %v \n", er)
		}
		fmt.Printf("added: %v \n", req.ArtistId)

		sec := time.Duration(10+rand.Intn(10)) * time.Second
		fmt.Printf("sleep %v sec... \n", sec.Seconds())
		time.Sleep(sec)
	}
}
