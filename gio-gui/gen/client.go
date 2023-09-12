// Package gen implements data generators.
package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/v0vc/go-music-grpc/artist"

	"github.com/v0vc/go-music-grpc/gio-gui/model"
)

// inflection point in the theoretical message timeline.
// RowTracker with serial before inflection are older, and messages after it
// are newer.
// const inflection = math.MaxInt64 / 2

var lock = &sync.Mutex{}

var singleInstance artist.ArtistServiceClient

var cc *grpc.ClientConn

var err error

var singleInstanceNoAvatar []byte

func GetNoAvatarInstance() []byte {
	if singleInstanceNoAvatar == nil {
		lock.Lock()
		defer lock.Unlock()

		emptyAva := "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"
		reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(emptyAva))
		buff := bytes.Buffer{}
		buff.ReadFrom(reader)
		singleInstanceNoAvatar = buff.Bytes()
	}
	return singleInstanceNoAvatar
}

func GetClientInstance() (artist.ArtistServiceClient, error) {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		fmt.Println("Creating single instance now.")
		cc, err = grpc.Dial("localhost:4041", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("could not connect: %v\n", err)
			cc.Close()
			return nil, err
		}
		singleInstance = artist.NewArtistServiceClient(cc)
	} else {
		fmt.Println("Single instance already created.")
		// defer cc.Close()
	}
	return singleInstance, nil
}

type Generator struct {
	// old is the serial counter for old messages.
	old syncInt
	// new is the serial counter for new messages.
	new syncInt
}

func (g *Generator) GetChannels(siteId uint32) *model.Rooms {
	var rooms model.Rooms
	baseRoom := model.Room{
		Name:   "-= NEW =-",
		Id:     "-1",
		Image:  nil,
		IsBase: true,
	}
	rooms.Add(baseRoom)
	client, _ := GetClientInstance()
	res, _ := client.ListArtist(context.Background(), &artist.ListArtistRequest{SiteId: siteId})
	for _, artist := range res.Artists {
		thumb := artist.GetThumbnail()
		if thumb == nil {
			thumb = GetNoAvatarInstance()
		}
		im, _, _ := image.Decode(bytes.NewReader(thumb))
		rooms.Add(model.Room{
			Name:   artist.GetTitle(),
			Id:     artist.GetArtistId(),
			Image:  im,
			IsBase: false,
		})
	}
	return &rooms
}

func (g *Generator) GetNewAlbums(siteId uint32) []model.Message {
	client, _ := GetClientInstance()
	res, _ := client.ReadNewAlbums(context.Background(), &artist.ListArtistRequest{
		SiteId: siteId,
	})
	// var albums = make([]model.Message, 0)
	var albums []model.Message
	for _, alb := range res.Releases {
		at, _ := time.Parse("2006-01-02T00:00:00", alb.GetReleaseDate())
		serial := g.old.Increment()
		thumb := alb.GetThumbnail()
		if thumb == nil {
			thumb = GetNoAvatarInstance()
		}
		im, _, _ := image.Decode(bytes.NewReader(thumb))
		al := model.Message{
			SerialID: fmt.Sprintf("%05d", serial),
			Sender:   alb.GetTitle(),
			Content:  alb.GetReleaseType(),
			SentAt:   at,
			Avatar:   im,
			Read:     false,
		}
		albums = append(albums, al)
	}
	return albums
}

func (g *Generator) GetArtistAlbums(siteId uint32, artistId string) []model.Message {
	client, _ := GetClientInstance()
	res, _ := client.ReadArtistAlbums(context.Background(), &artist.ReadArtistAlbumRequest{
		SiteId:   siteId,
		ArtistId: artistId,
	})
	var albums []model.Message
	// index := len(res.Releases)
	// cur := time.Now().Unix()
	for _, alb := range res.Releases {
		at, _ := time.Parse("2006-01-02T00:00:00", alb.GetReleaseDate())
		serial := g.old.Increment()
		// at     = time.Now().Add(time.Hour * time.Duration(serial) * -1)
		thumb := alb.GetThumbnail()
		if thumb == nil {
			thumb = GetNoAvatarInstance()
		}
		im, _, _ := image.Decode(bytes.NewReader(thumb))
		al := model.Message{
			// SerialID: fmt.Sprintf("%05d", cur-at.Unix()),
			SerialID: fmt.Sprintf("%05d", serial),
			Sender:   alb.GetTitle(),
			Content:  alb.GetReleaseType(),
			SentAt:   at,
			Avatar:   im,
			/*			Read: func() bool {
						return serial > index
					}(),*/
			Read: false,
		}
		albums = append(albums, al)
	}
	return albums
}

func (g *Generator) GenNewMessage(sender, content string) model.Message {
	serial := g.new.Decrement()
	im, _, _ := image.Decode(bytes.NewReader(GetNoAvatarInstance()))
	return model.Message{
		SerialID: fmt.Sprintf("%05d", serial),
		Sender:   sender,
		Content:  content,
		SentAt:   time.Now(),
		Avatar:   im,
		Read:     true,
		// Status: "TEST",
	}
}

// syncInt is a synchronized integer.
type syncInt struct {
	v int
	sync.Mutex
}

// Increment and return a copy of the underlying value.
func (si *syncInt) Increment() int {
	var v int
	si.Lock()
	si.v++
	v = si.v
	si.Unlock()
	return v
}

func (si *syncInt) Decrement() int {
	var v int
	si.Lock()
	si.v--
	v = si.v
	si.Unlock()
	return v
}
