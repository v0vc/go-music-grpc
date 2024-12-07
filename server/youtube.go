package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/v0vc/go-music-grpc/artist"
)

func GetChannelIdsFromDb(ctx context.Context, siteId uint32) ([]ArtistRawId, error) {
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

	var artistIds []ArtistRawId

	stmtArt, err := db.PrepareContext(ctx, "select c.ch_id, c.channelId, p.playlistId from main.channel c inner join main.channelPlaylist cP on c.ch_id = cP.channelId inner join main.playlist p on p.pl_id = cP.playlistId where siteId = ? and p.playlistType = 0;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtArt)

	rows, err := stmtArt.QueryContext(ctx, siteId)
	if err != nil {
		log.Println(err)
	}

	for rows.Next() {
		var artId ArtistRawId

		if er := rows.Scan(&artId.RawId, &artId.Id, &artId.PlaylistId); er != nil {
			log.Println(err)
		} else {
			artistIds = append(artistIds, artId)
		}
	}

	return artistIds, err
}

func getChannelIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId interface{}) int {
	stmtArt, err := tx.PrepareContext(ctx, "select ch_id from main.channel where channelId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtArt)

	var artRawId int

	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no channel with id %v\n", artistId)
	case err != nil:
		log.Println(err)
	default:
		fmt.Printf("siteId: %v, channel db id is %d\n", siteId, artRawId)
	}

	return artRawId
}

func SyncArtistYou(ctx context.Context, siteId uint32, artistId ArtistRawId, isAdd bool) (*artist.Artist, error) {
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

	// заберем токен для работы с апи
	token := GetTokenOnlyDb(tx, ctx, siteId)

	var artRawId int

	// сначала проверим, есть ли в базе этот канал
	if artistId.RawId == 0 {
		// кликнули по конкретному каналу
		artRawId = getChannelIdDb(tx, ctx, siteId, artistId.Id)
	} else {
		// синк в рамках всех каналов, все вычислено разом выше
		artRawId = artistId.RawId
	}

	if artRawId != 0 && isAdd {
		// пытались добавить существующего, сделаем просто синк
		isAdd = false
	}

	// при добавлении мы поддерживаем все варианты на Ui (ссылка на видео, на канал и тд)
	if isAdd && strings.HasPrefix(artistId.Id, "@") || len(artistId.Id) == 11 {
		// с ui пришли либо имя канала с @, либо id видео, найдем id канала
		chId, er := geChannelId(ctx, token, artistId.Id)
		if er != nil {
			log.Println(er)
		}
		artistId.Id = chId
	}

	fmt.Printf("Channel id is: %v \n", artistId.Id)

	if isAdd {
		ch, er := getChannel(ctx, artistId.Id, token)
		if er != nil || len(ch.Items) != 1 {
			log.Println(er)
			return nil, tx.Rollback()
		}

		stChannel, er := tx.PrepareContext(ctx, "insert into main.channel(siteId, channelId, title, thumbnail) values (?,?,?,?) on conflict (siteId, channelId) do update set syncState = 1 returning ch_id;")
		if er != nil {
			log.Println(er)
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
			fmt.Printf("Processed channel: %v, id: %v \n", ch.Items[0].Snippet.Title, chId)
		}

		resArtist := &artist.Artist{
			SiteId:    siteId,
			ArtistId:  artistId.Id,
			Title:     ch.Items[0].Snippet.Title,
			Thumbnail: chThumb,
			UserAdded: true,
			NewAlbs:   0,
		}

		stPlaylist, er := tx.PrepareContext(ctx, "insert into main.playlist(playlistId) values (?) on conflict (playlistId, title) do nothing returning pl_id;")
		if er != nil {
			log.Println(er)
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
			fmt.Printf("Processed playlist: %v, id: %v \n", uploadId, plId)
		}

		stChPl, er := tx.PrepareContext(ctx, "insert into main.channelPlaylist(channelId, playlistId) values (?,?) on conflict do nothing;")
		if er != nil {
			log.Println(er)
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
						published:     strings.TrimRight(strings.Replace(vid.Snippet.PublishedAt, "T", " ", 1), "Z"),
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

		stVideo, er := tx.PrepareContext(ctx, "insert into main.video(videoId, title, timestamp, duration, likeCount, viewCount, commentCount, thumbnail) values (?,?,?,?,?,?,?,?) on conflict (videoId, title) do update set syncState = 1 returning vid_id;")
		if er != nil {
			log.Println(er)
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
			normalDuration := ConvertYoutubeDurationToSec(vid.duration)
			vidErr := stVideo.QueryRowContext(ctx, vid.id, vid.title, vid.published, normalDuration, vid.likeCount, vid.viewCount, vid.commentCount, PrepareThumb(vThumb, 15, 64, 64, 90)).Scan(&vidId)
			if vidErr != nil {
				log.Println(vidErr)
			} else {
				fmt.Printf("Processed video: %v \n", vidId)
				vidRawIds = append(vidRawIds, vidId)
				date, _ := time.Parse(time.DateTime, vid.published)

				resArtist.Albums = append(resArtist.Albums, &artist.Album{
					AlbumId:     vid.id,
					Title:       vid.title,
					SubTitle:    fmt.Sprintf("%s   %s   Views: %s   Likes: %s", normalDuration, TimeAgo(date), vid.likeCount, vid.viewCount),
					ReleaseDate: vid.published,
					ReleaseType: 3,
					Thumbnail:   vThumb,
				})
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

		_, er = stArtAlb.ExecContext(ctx, args...)
		if er != nil {
			log.Println(er)
		}

		/*for c := range slices.Chunk(uploads, 50) {
			var sb strings.Builder
			for _, vid := range c{
				sb.WriteString(vid.)
			}
		}*/

		return resArtist, tx.Commit()
	} else {
		// синк

		return nil, nil
	}
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

	stRows, err := db.PrepareContext(ctx, "select v.vid_id, v.title, v.videoId, v.duration, v.timestamp, v.likeCount, v.viewCount, v.thumbnail, v.syncState from main.video v join main.playlistVideo pV on v.vid_id = pV.videoId where pV.playlistId = (select pl_id from main.playlist join main.channelPlaylist cP on playlist.pl_id = cP.playlistId where cP.channelId = (select ch_id from main.channel where channelId = ? and siteId = ? limit 1) and playlistType = 0 limit 1) order by 5 desc;")
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

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.SubTitle, &alb.ReleaseDate, &alb.LikeCount, &alb.ViewCount, &alb.Thumbnail, &alb.SyncState); err != nil {
			log.Println(err)
		} else {
			date, er := time.Parse(time.DateTime, alb.ReleaseDate)
			if er != nil {
				log.Println(er)
			} else {
				alb.SubTitle = fmt.Sprintf("%s   %s   Views: %d   Likes: %d", alb.GetSubTitle(), TimeAgo(date), alb.GetViewCount(), alb.GetLikeCount())
			}
			alb.ReleaseType = 3
			albs = append(albs, &alb)
		}
	}

	return albs, err
}

func GetNewVideosFromDb(ctx context.Context) ([]*artist.Album, error) {
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

	stRows, err := db.PrepareContext(ctx, "select v.vid_id, v.title, v.videoId, v.duration, v.timestamp, v.likeCount, v.viewCount, v.thumbnail, v.syncState from main.video v where v.syncState = 1 order by 5 desc;")
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

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.SubTitle, &alb.ReleaseDate, &alb.LikeCount, &alb.ViewCount, &alb.Thumbnail, &alb.SyncState); err != nil {
			log.Println(err)
		} else {
			date, er := time.Parse(time.DateTime, alb.ReleaseDate)
			if er != nil {
				log.Println(er)
			} else {
				alb.SubTitle = fmt.Sprintf("%s  %s  %d  %d", alb.GetSubTitle(), TimeAgo(date), alb.GetViewCount(), alb.GetLikeCount())
			}
			alb.ReleaseType = 3
			albs = append(albs, &alb)
		}
	}

	return albs, err
}

func DeleteChannelDb(ctx context.Context, siteId uint32, artistId []string) (int64, error) {
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

	var deletedRowCount int64
	stDelete, err := tx.PrepareContext(ctx, "delete from main.channel where channelId = ? and siteId = ?;")
	if err != nil {
		log.Println(err)
	}
	defer func(stDelete *sql.Stmt) {
		er := stDelete.Close()
		if er != nil {
			log.Println(er)
		}
	}(stDelete)

	for _, id := range artistId {
		res, er := stDelete.ExecContext(ctx, id, siteId)
		if er != nil {
			log.Println(er)
		}
		rowCount, er := res.RowsAffected()
		if er != nil {
			log.Println(er)
		} else {
			deletedRowCount = deletedRowCount + rowCount
		}
	}

	return deletedRowCount, tx.Commit()
}
