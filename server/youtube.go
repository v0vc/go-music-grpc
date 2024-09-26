package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/v0vc/go-music-grpc/artist"
)

const (
	youtubeApi      = "https://www.googleapis.com/youtube/v3/"
	chanelString    = "channels?id=[ID]&key=[KEY]&part=contentDetails,snippet,statistics&fields=items(contentDetails(relatedPlaylists(uploads)),snippet(title,thumbnails(default(url))),statistics(viewCount,subscriberCount))&prettyPrint=false"
	uploadString    = "playlistItems?key=[KEY]&playlistId=[ID]&part=snippet,contentDetails&order=date&fields=nextPageToken,items(snippet(publishedAt,title,resourceId(videoId),thumbnails(default(url))),contentDetails(videoPublishedAt))&maxResults=50&prettyPrint=false"
	statisticString = "videos?id=[VID]&key=[KEY]&part=contentDetails,statistics&fields=items(id,contentDetails(duration),statistics(viewCount,commentCount,likeCount))&prettyPrint=false"
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

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

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

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var uploads *Uploads
	err = json.NewDecoder(response.Body).Decode(&uploads)
	if err != nil || uploads == nil {
		return new(Uploads), err
	}
	return uploads, nil
}

func geStatistics(ctx context.Context, url string) (*Statistics, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(Statistics), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(Statistics), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var stat *Statistics
	err = json.NewDecoder(response.Body).Decode(&stat)
	if err != nil || stat == nil {
		return new(Statistics), err
	}
	return stat, nil
}

func SyncArtistYou(ctx context.Context, siteId uint32, artistId ArtistRawId, isAdd bool) (*artist.Artist, error) {
	var resArtist *artist.Artist

	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

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

	stChannel, err := tx.PrepareContext(ctx, "insert into main.channel(siteId, channelId, title, thumbnail) values (?,?,?,?) on conflict (siteId, channelId) do update set syncState = 1 returning ch_id;")
	if err != nil {
		log.Println(err)
	}
	defer func(stChannel *sql.Stmt) {
		err = stChannel.Close()
		if err != nil {
			log.Println(err)
		}
	}(stChannel)

	var chId int
	chThumb := GetThumb(ctx, ch.Items[0].Snippet.Thumbnails.Default.URL)
	insErr := stChannel.QueryRowContext(ctx, siteId, artistId.Id, ch.Items[0].Snippet.Title, chThumb).Scan(&chId)
	if insErr != nil {
		log.Println(insErr)
	} else {
		log.Printf("Processed channel: %v, id: %v \n", ch.Items[0].Snippet.Title, chId)
	}

	stPlaylist, err := tx.PrepareContext(ctx, "insert into main.playlist(playlistId) values (?) on conflict (playlistId, title) do nothing returning pl_id;")
	if err != nil {
		log.Println(err)
	}
	defer func(stPlaylist *sql.Stmt) {
		err = stPlaylist.Close()
		if err != nil {
			log.Println(err)
		}
	}(stPlaylist)

	uploadId := ch.Items[0].ContentDetails.RelatedPlaylists.Uploads
	var plId int
	insEr := stPlaylist.QueryRowContext(ctx, uploadId).Scan(&plId)
	if insErr != nil {
		log.Println(insEr)
	} else {
		log.Printf("Processed playlist: %v, id: %v \n", uploadId, plId)
	}

	stChPl, err := tx.PrepareContext(ctx, "insert into main.channelPlaylist(channelId, playlistId) values (?,?) on conflict do nothing;")
	if err != nil {
		log.Println(err)
	}
	defer func(stChPl *sql.Stmt) {
		err = stChPl.Close()
		if err != nil {
			log.Println(err)
		}
	}(stChPl)

	_, err = stChPl.ExecContext(ctx, chId, plId)
	if err != nil {
		log.Println(err)
	}

	type vidItem struct {
		id            string
		title         string
		published     time.Time
		duration      string
		likeCount     string
		viewCount     string
		commentCount  string
		thumbnailLink string
		thumbnail     []byte
	}
	var videos []*vidItem

	urlUpload := strings.Replace(strings.Replace(uploadString, "[ID]", uploadId, 1), "[KEY]", token, 1)
	i := 0
	for {
		upl, er := geUpload(ctx, urlUpload)
		if er == nil && upl != nil {
			var sb strings.Builder
			for _, vid := range upl.Items {
				sb.WriteString(vid.Snippet.ResourceID.VideoID + ",")
				videos = append(videos, &vidItem{
					id:            vid.Snippet.ResourceID.VideoID,
					title:         vid.Snippet.Title,
					published:     vid.Snippet.PublishedAt,
					thumbnailLink: vid.Snippet.Thumbnails.Default.URL,
				})
			}
			urlStat := strings.Replace(strings.Replace(statisticString, "[VID]", strings.TrimRight(sb.String(), ","), 1), "[KEY]", token, 1)
			stat, ere := geStatistics(ctx, urlStat)
			if ere == nil && stat != nil {
				for _, vid := range stat.Items {
					idx := slices.IndexFunc(videos, func(c *vidItem) bool { return c.id == vid.ID })
					if idx != -1 {
						videos[idx].duration = vid.ContentDetails.Duration
						videos[idx].likeCount = vid.Statistics.LikeCount
						videos[idx].viewCount = vid.Statistics.ViewCount
						videos[idx].commentCount = vid.Statistics.CommentCount
					} else {
						log.Println("Failed to find video ", vid.ID)
					}
				}
			}
		}
		if upl == nil || upl.NextPageToken == "" {
			break
		} else {
			if i == 0 {
				urlUpload = fmt.Sprintf("%v&pageToken=[PAGE]", urlUpload)
			}
			urlUpload = strings.Replace(urlUpload, "[PAGE]", upl.NextPageToken, 1)
		}
		i++
	}

	stVideo, err := tx.PrepareContext(ctx, "insert into main.video(videoId, title, timestamp, duration, likeCount, viewCount, commentCount, thumbnail) values (?,?,?,?,?,?,?,?) on conflict (videoId, title) do update set syncState = 1 returning vid_id;")
	if err != nil {
		log.Println(err)
	}
	defer func(stVideo *sql.Stmt) {
		err = stVideo.Close()
		if err != nil {
			log.Println(err)
		}
	}(stVideo)

	var vidRawIds []int
	for _, vid := range videos {
		vid.thumbnail = GetThumb(ctx, vid.thumbnailLink)
		var vidId int
		vidErr := stVideo.QueryRowContext(ctx, vid.id, vid.title, vid.published, vid.duration, vid.likeCount, vid.viewCount, vid.commentCount, vid.thumbnail).Scan(&vidId)
		if vidErr != nil {
			log.Println(vidErr)
		} else {
			log.Printf("Processed video: %v \n", vidId)
			vidRawIds = append(vidRawIds, vidId)
		}
	}

	sqlStr := fmt.Sprintf("insert into main.playlistVideo(playlistId, videoId) values %v on conflict (playlistId, videoId) do nothing;", strings.TrimSuffix(strings.Repeat("(?,?),", len(videos)), ","))
	stArtAlb, _ := tx.PrepareContext(ctx, sqlStr)

	defer func(stArtAlb *sql.Stmt) {
		err = stArtAlb.Close()
		if err != nil {
			log.Println(err)
		}
	}(stArtAlb)

	var args []interface{}
	for _, v := range vidRawIds {
		args = append(args, plId, v)
	}

	_, err = stArtAlb.ExecContext(ctx, args...)
	if err != nil {
		log.Println(err)
	}

	/*for c := range slices.Chunk(uploads, 50) {
		var sb strings.Builder
		for _, vid := range c{
			sb.WriteString(vid.)
		}
	}*/

	return resArtist, tx.Commit()
}
