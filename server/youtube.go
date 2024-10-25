package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/v0vc/go-music-grpc/artist"
)

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
		published     string
		duration      string
		likeCount     string
		viewCount     string
		commentCount  string
		thumbnailLink string
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
		vThumb := GetThumb(ctx, vid.thumbnailLink)
		var vidId int
		vidErr := stVideo.QueryRowContext(ctx, vid.id, vid.title, strings.TrimRight(strings.Replace(vid.published, "T", " ", 1), "Z"), ConvertYoutubeDurationToSec(vid.duration), vid.likeCount, vid.viewCount, vid.commentCount, PrepareThumb(vThumb, 15, 64, 64, 90)).Scan(&vidId)
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

func GetChannels(ctx context.Context, siteId uint32) ([]*artist.Artist, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))

	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	var arts []*artist.Artist

	stmt, err := db.PrepareContext(context.WithoutCancel(ctx), "select ch.ch_id, ch.channelId, ch.title, ch.thumbnail, COUNT(IIF(v.syncState > 0, 1, NULL)) as news from main.channel ch join main.channelPlaylist cp on ch.ch_id = cp.channelId inner join main.playlistVideo plv on plv.playlistId = cp.playlistId inner join main.video v on v.vid_id = plv.videoId where ch.siteId = ? group by ch.ch_id order by 3;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmt)

	rows, err := stmt.QueryContext(context.WithoutCancel(ctx), siteId)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	for rows.Next() {
		var art artist.Artist
		if e := rows.Scan(&art.Id, &art.ArtistId, &art.Title, &art.Thumbnail, &art.NewAlbs); e != nil {
			log.Println(e)
		} else {
			art.SiteId = siteId
			arts = append(arts, &art)
		}
	}
	return arts, err
}

func GetChannelVideosFromDb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Album, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	stRows, err := db.PrepareContext(ctx, "select v.vid_id, v.title, v.videoId, v.duration, v.timestamp, v.thumbnail, v.syncState from main.video v join main.playlistVideo pV on v.vid_id = pV.videoId where pV.playlistId = (select pl_id from main.playlist join main.channelPlaylist cP on playlist.pl_id = cP.playlistId where cP.channelId = (select ch_id from main.channel where channelId = ? and siteId = ? limit 1) and playlistType = 0 limit 1) order by 5 desc;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx, artistId, siteId)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var albs []*artist.Album

	for rows.Next() {
		var alb artist.Album

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.SubTitle, &alb.ReleaseDate, &alb.Thumbnail, &alb.SyncState); err != nil {
			log.Println(err)
		} else {
			date, er := time.Parse(time.DateTime, alb.ReleaseDate)
			if er != nil {
				log.Println(er)
			} else {
				alb.SubTitle = fmt.Sprintf("%s  %s", alb.GetSubTitle(), TimeAgo(date))
			}
			alb.ReleaseType = 3
			albs = append(albs, &alb)
		}
	}

	return albs, err
}

func GetNewVideosFromDb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Album, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	stRows, err := db.PrepareContext(ctx, "select v.vid_id, v.title, v.videoId, v.duration, v.timestamp, v.thumbnail, v.syncState from main.video v where v.syncState = 1 order by 5 desc;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx)
	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var albs []*artist.Album

	for rows.Next() {
		var alb artist.Album

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.SubTitle, &alb.ReleaseDate, &alb.Thumbnail, &alb.SyncState); err != nil {
			log.Println(err)
		} else {
			date, er := time.Parse(time.DateTime, alb.ReleaseDate)
			if er != nil {
				log.Println(er)
			} else {
				alb.SubTitle = fmt.Sprintf("%s  %s", alb.GetSubTitle(), TimeAgo(date))
			}
			alb.ReleaseType = 3
			albs = append(albs, &alb)
		}
	}

	return albs, err
}
