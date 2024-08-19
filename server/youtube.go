package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/v0vc/go-music-grpc/artist"
)

const (
	youtubeApi   = "https://www.googleapis.com/youtube/v3/"
	chanelString = "channels?id=[ID]&key=[KEY]&part=contentDetails,snippet,statistics&fields=items(contentDetails(relatedPlaylists),snippet(title,thumbnails(default(url))),statistics(viewCount,subscriberCount))&prettyPrint=false"
	uploadString = "playlistItems?key=[KEY]&playlistId=[ID]&part=snippet,contentDetails&order=date&fields=nextPageToken,items(snippet(publishedAt,title,resourceId(videoId)),contentDetails(videoPublishedAt))&maxResults=50&prettyPrint=false"
)

func getChannel(ctx context.Context, channelId string, apiKey string) (*Channel, error) {
	url := strings.Replace(strings.Replace(chanelString, "[ID]", channelId, 1), "[KEY]", apiKey, 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(Channel), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(Channel), err
	}

	defer response.Body.Close()

	var channel *Channel
	err = json.NewDecoder(response.Body).Decode(&channel)
	if err != nil || channel == nil {
		return new(Channel), err
	}
	return channel, nil
}

func geUpload(ctx context.Context, url string) (*Uploads, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(Uploads), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(Uploads), err
	}

	defer response.Body.Close()

	var uploads *Uploads
	err = json.NewDecoder(response.Body).Decode(&uploads)
	if err != nil || uploads == nil {
		return new(Uploads), err
	}
	return uploads, nil
}

func SyncArtistYou(ctx context.Context, siteId uint32, artistId ArtistRawId, isAdd bool) (*artist.Artist, error) {
	var resArtist *artist.Artist

	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	_, _, token := GetTokenDb(tx, ctx, siteId)
	ch, err := getChannel(ctx, artistId.Id, token)
	if err != nil || len(ch.Items) != 1 {
		log.Println(err)
		return resArtist, tx.Rollback()
	}

	var uploads []*Uploads
	urlUpload := strings.Replace(strings.Replace(uploadString, "[ID]", ch.Items[0].ContentDetails.RelatedPlaylists.Uploads, 1), "[KEY]", token, 1)
	i := 0
	for {
		upl, er := geUpload(ctx, urlUpload)
		if er == nil && upl != nil {
			uploads = append(uploads, upl)
		}
		if upl.NextPageToken == "" || er != nil {
			break
		} else {
			if i == 0 {
				urlUpload = fmt.Sprintf("%v&pageToken=[PAGE]", urlUpload)
			}
			urlUpload = strings.Replace(urlUpload, "[PAGE]", upl.NextPageToken, 1)
		}
		i++
	}

	fmt.Println(uploads)

	return resArtist, nil
}
