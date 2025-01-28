package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	slices2 "golang.org/x/exp/slices"

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

	stmtArt, err := db.PrepareContext(ctx, "select c.ch_id, c.channelId, p.pl_id, p.playlistId, group_concat(v.videoId, ',') from main.video v inner join main.playlistVideo pV on v.vid_id = pV.videoId inner join main.playlist p on p.pl_id = pV.playlistId inner join main.channelPlaylist cP on p.pl_id = cP.playlistId inner join main.channel c on c.ch_id = cP.channelId where p.playlistType = 0 and c.siteId = ? group by c.ch_id;")
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
		var (
			artId ArtistRawId
			ids   string
		)

		if er := rows.Scan(&artId.RawId, &artId.Id, &artId.RawId, &artId.PlaylistId, &ids); er != nil {
			log.Println(err)
		} else {
			artId.vidIds = strings.Split(ids, ",")
			artistIds = append(artistIds, artId)
		}
	}

	return artistIds, err
}

func getChannelIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, channelId interface{}, syncPl bool) ArtistRawId {
	stmtArt, err := tx.PrepareContext(ctx, "select c.ch_id, c.channelId, p.pl_id, p.playlistId, group_concat(v.videoId, ',') from main.video v inner join main.playlistVideo pV on v.vid_id = pV.videoId inner join main.playlist p on p.pl_id = pV.playlistId inner join main.channelPlaylist cP on p.pl_id = cP.playlistId inner join main.channel c on c.ch_id = cP.channelId where c.channelId = ? and siteId = ? and p.playlistType = 0 limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmtArt)

	var (
		artId ArtistRawId
		ids   string
	)

	err = stmtArt.QueryRowContext(ctx, channelId, siteId).Scan(&artId.RawId, &artId.Id, &artId.RawPlId, &artId.PlaylistId, &ids)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no channel with id %v\n", channelId)
	case err != nil:
		log.Println(err)
	default:
		fmt.Printf("siteId: %v, channel db id is %d\n", siteId, artId.RawId)
		artId.vidIds = strings.Split(ids, ",")
		artId.isPlSync = syncPl
	}

	return artId
}

func SyncArtistYou(ctx context.Context, siteId uint32, channelId ArtistRawId, isAdd bool) (*artist.Artist, error) {
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

	// при добавлении мы поддерживаем все варианты на Ui (ссылка на видео, на канал и тд)
	if isAdd && strings.HasPrefix(channelId.Id, "@") || len(channelId.Id) == 11 {
		// с ui пришли либо имя канала с @, либо id видео, найдем id канала
		chId, er := GetChannelId(ctx, token, channelId.Id)
		if er != nil {
			log.Println(er)
		}
		channelId.Id = chId
	}

	if channelId.RawId != 0 && isAdd {
		// пытались добавить существующего, сделаем просто синк
		isAdd = false
	}

	fmt.Printf("channel id is: %v \n", channelId.Id)

	if isAdd {
		ch, er := GetChannel(ctx, channelId.Id, token)
		if er != nil || len(ch.Items) != 1 {
			log.Println(er)
			return nil, tx.Rollback()
		}

		stChannel, er := tx.PrepareContext(ctx, "insert into main.channel(siteId, channelId, title, thumbnail) values (?,?,?,?) on conflict (siteId, channelId) do update set syncState = 1 returning ch_id;")
		if er != nil {
			log.Println(er)
		}
		defer func(stChannel *sql.Stmt) {
			er = stChannel.Close()
			if er != nil {
				log.Println(er)
			}
		}(stChannel)

		var chId int
		chThumb := GetThumb(ctx, ch.Items[0].Snippet.Thumbnails.Default.URL)
		insErr := stChannel.QueryRowContext(ctx, siteId, channelId.Id, ch.Items[0].Snippet.Title, chThumb).Scan(&chId)
		if insErr != nil {
			log.Println(insErr)
		} else {
			fmt.Printf("processed channel: %v, id: %v \n", ch.Items[0].Snippet.Title, chId)
		}

		resArtist := &artist.Artist{
			SiteId:    siteId,
			ArtistId:  channelId.Id,
			Title:     ch.Items[0].Snippet.Title,
			Thumbnail: chThumb,
		}

		stPlaylist, er := tx.PrepareContext(ctx, "insert into main.playlist(playlistId,title,playlistType) values (?,?,?) on conflict (playlistId, title) do nothing returning pl_id;")
		if er != nil {
			log.Println(er)
		}
		defer func(stPlaylist *sql.Stmt) {
			er = stPlaylist.Close()
			if er != nil {
				log.Println(er)
			}
		}(stPlaylist)

		stChPl, er := tx.PrepareContext(ctx, "insert into main.channelPlaylist(channelId, playlistId) values (?,?) on conflict do nothing;")
		if er != nil {
			log.Println(er)
		}
		defer func(stChPl *sql.Stmt) {
			er = stChPl.Close()
			if er != nil {
				log.Println(er)
			}
		}(stChPl)

		allPl := GetPlaylists(ctx, channelId.Id, token)
		allPl = append(allPl, &plItem{
			id:     ch.Items[0].ContentDetails.RelatedPlaylists.Uploads,
			title:  "Uploads",
			typePl: 0,
		})

		for i, item := range allPl {
			var plId int
			insEr := stPlaylist.QueryRowContext(ctx, item.id, item.title, item.typePl).Scan(&plId)
			if insEr != nil {
				log.Println(insEr)
			} else {
				fmt.Printf("processed playlist: %v, id: %v \n", item.id, plId)
				item.rawId = plId
				_, er = stChPl.ExecContext(ctx, chId, plId)
				if er != nil {
					log.Println(er)
				} else if i == len(allPl)-1 {
					channelId.RawPlId = plId
				}
			}
		}

		uploadPl := allPl[len(allPl)-1]
		videos := GetUploadVid(ctx, uploadPl.id, token)
		var uploadVidIds map[string]int
		if videos != nil {
			uploadVidIds = processVideos(ctx, tx, videos, resArtist, uploadPl.rawId, channelId.Id, 0, 0)
		} else {
			log.Println("can't get video from api")
		}

		var notUploadId []string
		for _, pl := range allPl[:len(allPl)-1] {
			netPlIds := GetPlaylistVidIds(ctx, pl.id, token)
			plVid := make(map[string]int)
			for _, vId := range netPlIds {
				rawVidId, ok := uploadVidIds[vId]
				if ok {
					plVid[vId] = rawVidId
				} else {
					notUploadId = append(notUploadId, vId)
				}
			}
			if len(plVid) == 0 {
				deletePlaylistById(ctx, tx, pl.rawId)
			} else {
				insertPlaylistVideoIds(ctx, tx, plVid, pl.rawId)
			}
		}

		insertUnlisted(ctx, tx, notUploadId, token, channelId, resArtist)

		return resArtist, tx.Commit()

	} else {
		// синк
		// сначала проверим, есть ли в базе этот канал
		if channelId.RawId == 0 {
			// кликнули по конкретному каналу
			channelId = getChannelIdDb(tx, ctx, siteId, channelId.Id, channelId.isPlSync)
		}
		// дропнем признаки предыдущей синхронизации
		stVidUpd, _ := tx.PrepareContext(ctx, fmt.Sprintf("update main.video set syncState = 0 where syncState = 1 and videoId in (? %v);", strings.Repeat(",?", len(channelId.vidIds)-1)))

		defer func(stVidUpd *sql.Stmt) {
			err = stVidUpd.Close()
			if err != nil {
				log.Println(err)
			}
		}(stVidUpd)

		args := make([]interface{}, len(channelId.vidIds))
		for i, artId := range channelId.vidIds {
			args[i] = artId
		}

		_, er := stVidUpd.QueryContext(ctx, args...)
		if er != nil {
			log.Println(er)
		}

		// получим актуальные айдишники из апи
		netIds := GetPlaylistVidIds(ctx, channelId.PlaylistId, token)
		// сравним
		newVidIds := FindDifference(netIds, channelId.vidIds)
		fmt.Printf("siteId: %v, channelId: %d, new videos: %d\n", siteId, channelId.RawId, len(newVidIds))

		resArtist := &artist.Artist{
			SiteId:   siteId,
			ArtistId: channelId.Id,
			NewAlbs:  int32(len(newVidIds)),
		}

		for c := range slices.Chunk(newVidIds, 50) {
			var sb strings.Builder
			for _, vid := range c {
				sb.WriteString(vid + ",")
			}
			videos := GetVidByIds(ctx, strings.TrimRight(sb.String(), ","), token)
			if videos != nil {
				processVideos(ctx, tx, videos, resArtist, channelId.RawPlId, channelId.Id, 1, 0)
			} else {
				log.Println("Can't get video from api")
			}
		}

		if channelId.isPlSync {
			// TODO синк полный с плейлистами

			stPlRem, er := tx.PrepareContext(ctx, "delete from main.playlist where pl_id in (select cp.playlistId from main.channelPlaylist cp inner join main.playlist p on cp.playlistId = p.pl_id where cp.channelId = ? and p.playlistType = 1);")
			if er != nil {
				log.Println(er)
			}
			defer func(stPlRem *sql.Stmt) {
				er = stPlRem.Close()
				if er != nil {
					log.Println(er)
				}
			}(stPlRem)

			_, er = stPlRem.ExecContext(ctx, channelId.RawId)
			if er != nil {
				log.Println(er)
			}

			stPlaylist, er := tx.PrepareContext(ctx, "insert into main.playlist(playlistId,title,playlistType) values (?,?,?) on conflict (playlistId, title) do nothing returning pl_id;")
			if er != nil {
				log.Println(er)
			}
			defer func(stPlaylist *sql.Stmt) {
				er = stPlaylist.Close()
				if er != nil {
					log.Println(er)
				}
			}(stPlaylist)

			stChPl, er := tx.PrepareContext(ctx, "insert into main.channelPlaylist(channelId, playlistId) values (?,?) on conflict do nothing;")
			if er != nil {
				log.Println(er)
			}
			defer func(stChPl *sql.Stmt) {
				er = stChPl.Close()
				if er != nil {
					log.Println(er)
				}
			}(stChPl)

			allPl := GetPlaylists(ctx, channelId.Id, token)

			for _, item := range allPl {
				var plId int
				insEr := stPlaylist.QueryRowContext(ctx, item.id, item.title, item.typePl).Scan(&plId)
				if insEr != nil {
					log.Println(insEr)
				} else {
					fmt.Printf("processed playlist: %v, id: %v \n", item.id, plId)
					item.rawId = plId
					_, er = stChPl.ExecContext(ctx, channelId.RawId, plId)
					if er != nil {
						log.Println(er)
					}
				}
			}

			var notUploadId []string
			for _, pl := range allPl {
				netPlIds := GetPlaylistVidIds(ctx, pl.id, token)
				plVid := make(map[string]int)
				for _, vId := range netPlIds {
					if !slices2.Contains(channelId.vidIds, vId) {
						notUploadId = append(notUploadId, vId)
					}
				}
				if len(plVid) == 0 {
					deletePlaylistById(ctx, tx, pl.rawId)
				} else {
					insertPlaylistVideoIds(ctx, tx, plVid, pl.rawId)
				}
			}

			insertUnlisted(ctx, tx, notUploadId, token, channelId, resArtist)
		}

		return resArtist, tx.Commit()
	}
}

func insertUnlisted(ctx context.Context, tx *sql.Tx, notUploadId []string, token string, channelId ArtistRawId, resArtist *artist.Artist) {
	for c := range slices.Chunk(notUploadId, 50) {
		var sb strings.Builder
		for _, vid := range c {
			sb.WriteString(vid + ",")
		}
		notListed := strings.TrimRight(sb.String(), ",")
		fmt.Println("found unlisted video(s): " + notListed)
		chVideosIds, e := GetChannelIdsByVid(ctx, token, notListed, channelId.Id)
		if e != nil {
			log.Println(e)
		} else if chVideosIds != "" {
			fmt.Println("processable unlisted video(s): " + chVideosIds)
			unlistedVideos := GetVidByIds(ctx, chVideosIds, token)
			if unlistedVideos != nil {
				processVideos(ctx, tx, unlistedVideos, resArtist, channelId.RawPlId, channelId.Id, 0, 1)
			} else {
				log.Println("can't get unlisted video from api")
			}
		}
	}
}

func processVideos(ctx context.Context, tx *sql.Tx, videos []*vidItem, resArtist *artist.Artist, plId int, channelId string, syncState int32, listState int32) map[string]int {
	stVideo, err := tx.PrepareContext(ctx, "insert into main.video(videoId, title, timestamp, duration, likeCount, viewCount, commentCount, syncState, listState, thumbnail) values (?,?,?,?,?,?,?,?,?,?) on conflict (videoId, title) do update set syncState = 1 returning vid_id;")
	if err != nil {
		log.Println(err)
	}
	defer func(stVideo *sql.Stmt) {
		err = stVideo.Close()
		if err != nil {
			log.Println(err)
		}
	}(stVideo)

	mVidRawIds := make(map[string]int)
	for _, vid := range videos {
		vThumb := GetThumb(ctx, vid.thumbnailLink)
		if vThumb != nil {
			vThumb = PrepareThumb(vThumb, 15, 64, 64, 90)
		}

		var vidId int
		normalDuration := ConvertYoutubeDurationToSec(vid.duration)
		vidErr := stVideo.QueryRowContext(ctx, vid.id, vid.title, vid.published, normalDuration, vid.likeCount, vid.viewCount, vid.commentCount, syncState, listState, vThumb).Scan(&vidId)
		if vidErr != nil {
			log.Println(vidErr)
		} else {
			fmt.Printf("processed video: %v \n", vid.id)
			mVidRawIds[vid.id] = vidId
			date, _ := time.Parse(time.DateTime, vid.published)
			subTitle := fmt.Sprintf("%s   %s   Views: %s   Likes: %s", normalDuration, TimeAgo(date), vid.likeCount, vid.viewCount)
			if listState == 1 {
				subTitle += "   👀"
			}

			resArtist.Albums = append(resArtist.Albums, &artist.Album{
				AlbumId:     vid.id,
				Title:       vid.title,
				SubTitle:    subTitle,
				ReleaseDate: vid.published,
				ReleaseType: 3,
				Thumbnail:   vThumb,
				SyncState:   syncState,
				ArtistIds:   []string{channelId},
			})
		}
	}

	if len(mVidRawIds) == 0 {
		deletePlaylistById(ctx, tx, plId)
	} else {
		insertPlaylistVideoIds(ctx, tx, mVidRawIds, plId)
	}

	return mVidRawIds
}

func deletePlaylistById(ctx context.Context, tx *sql.Tx, plId int) {
	stPl, err := tx.PrepareContext(ctx, "delete from main.playlist where pl_id=?;")
	if err != nil {
		log.Println(err)
	}

	defer func(stArtAlb *sql.Stmt) {
		er := stArtAlb.Close()
		if er != nil {
			log.Println(er)
		}
	}(stPl)

	_, err = stPl.ExecContext(ctx, plId)
	if err != nil {
		log.Println(err)
	}
}

func insertPlaylistVideoIds(ctx context.Context, tx *sql.Tx, mVidRawIds map[string]int, plId int) {
	sqlStr := fmt.Sprintf("insert into main.playlistVideo(playlistId, videoId) values %v on conflict (playlistId, videoId) do nothing;", strings.TrimSuffix(strings.Repeat("(?,?),", len(mVidRawIds)), ","))
	stArtAlb, err := tx.PrepareContext(ctx, sqlStr)
	if err != nil {
		log.Println(err)
	}

	defer func(stArtAlb *sql.Stmt) {
		er := stArtAlb.Close()
		if er != nil {
			log.Println(er)
		}
	}(stArtAlb)

	var args []interface{}
	for _, v := range mVidRawIds {
		args = append(args, plId, v)
	}

	_, err = stArtAlb.ExecContext(ctx, args...)
	if err != nil {
		log.Println(err)
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

	stmt, err := db.PrepareContext(ctx, "select ch.ch_id, ch.channelId, ch.title, ch.thumbnail, COUNT(IIF(v.syncState > 0, 1, NULL)) as news from main.channel ch join main.channelPlaylist cp on ch.ch_id = cp.channelId inner join main.playlistVideo plv on plv.playlistId = cp.playlistId inner join main.video v on v.vid_id = plv.videoId where ch.siteId = ? group by ch.ch_id order by 3;")
	if err != nil {
		log.Println(err)
	}
	defer func(stmtArt *sql.Stmt) {
		err = stmtArt.Close()
		if err != nil {
			log.Println(err)
		}
	}(stmt)

	rows, err := stmt.QueryContext(ctx, siteId)
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

func GetChannelVideosFromDb(ctx context.Context, siteId uint32, channelId string) ([]*artist.Album, error) {
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

	stRows, err := db.PrepareContext(ctx, "select v.vid_id, v.title, v.videoId, v.duration, v.timestamp, v.likeCount, v.viewCount, v.thumbnail, v.syncState, v.listState, ifnull(v.quality,0) from main.video v join main.playlistVideo pV on v.vid_id = pV.videoId where pV.playlistId = (select pl_id from main.playlist join main.channelPlaylist cP on playlist.pl_id = cP.playlistId where cP.channelId = (select ch_id from main.channel where channelId = ? and siteId = ? limit 1) and playlistType = 0 limit 1) order by 5 desc;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx, channelId, siteId)
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
		var (
			alb        artist.Album
			visibility int
		)

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.SubTitle, &alb.ReleaseDate, &alb.LikeCount, &alb.ViewCount, &alb.Thumbnail, &alb.SyncState, &visibility, &alb.Quality); err != nil {
			log.Println(err)
		} else {
			date, er := time.Parse(time.DateTime, alb.ReleaseDate)
			if er != nil {
				log.Println(er)
			} else {
				alb.SubTitle = fmt.Sprintf("%s   %s   Views: %d   Likes: %d", alb.GetSubTitle(), TimeAgo(date), alb.GetViewCount(), alb.GetLikeCount())
				if visibility == 1 {
					alb.SubTitle += "   👀"
				}
			}
			alb.ReleaseType = 3
			alb.ArtistIds = []string{channelId}
			albs = append(albs, &alb)
		}
	}

	return albs, err
}

func GetChannelVideosIdFromDb(ctx context.Context, siteId uint32, channelId string, newOnly bool) ([]string, error) {
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

	var str string

	if newOnly {
		str = "select v.videoId, v.title from main.video v inner join main.playlistVideo pV on v.vid_id = pV.videoId inner join main.playlist p on p.pl_id = pV.playlistId inner join main.channelPlaylist cP on p.pl_id = cP.playlistId inner join main.channel c on c.ch_id = cP.channelId where v.syncState = 1 and siteId = ?;"
	} else {
		str = "select v.videoId, v.title from main.video v inner join main.playlistVideo pV on v.vid_id = pV.videoId inner join main.playlist p on p.pl_id = pV.playlistId inner join main.channelPlaylist cP on p.pl_id = cP.playlistId inner join main.channel c on c.ch_id = cP.channelId where c.channelId = ? and siteId = ?;"
	}
	stRows, err := db.PrepareContext(ctx, str)
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	var rows *sql.Rows
	if newOnly {
		rows, err = stRows.QueryContext(ctx, siteId)
	} else {
		rows, err = stRows.QueryContext(ctx, channelId, siteId)
	}

	if err != nil {
		log.Println(err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	var albIds []string

	for rows.Next() {
		var (
			vidId    string
			vidTitle string
		)
		if err = rows.Scan(&vidId, &vidTitle); err != nil {
			log.Println(err)
		} else {
			albIds = append(albIds, channelId+";"+vidId+";"+vidTitle)
		}
	}

	return albIds, err
}

func GetNewVideosFromDb(ctx context.Context, siteId uint32) ([]*artist.Album, error) {
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

	stRows, err := db.PrepareContext(ctx, "select v.vid_id, v.title, v.videoId, v.duration, v.timestamp, v.likeCount, v.viewCount, v.thumbnail, v.syncState, v.listState, ifnull(v.quality,0), c.channelId from main.video v inner join main.playlistVideo pV on v.vid_id = pV.videoId inner join main.playlist p on p.pl_id = pV.playlistId inner join main.channelPlaylist cP on p.pl_id = cP.playlistId inner join main.channel c on c.ch_id = cP.channelId where v.syncState = 1 and p.playlistType = 0 and c.siteId = ? order by 5 desc;")
	if err != nil {
		log.Println(err)
	}
	defer func(stRows *sql.Stmt) {
		err = stRows.Close()
		if err != nil {
			log.Println(err)
		}
	}(stRows)

	rows, err := stRows.QueryContext(ctx, siteId)
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
		var (
			alb        artist.Album
			parentId   string
			visibility int
		)

		if err = rows.Scan(&alb.Id, &alb.Title, &alb.AlbumId, &alb.SubTitle, &alb.ReleaseDate, &alb.LikeCount, &alb.ViewCount, &alb.Thumbnail, &alb.SyncState, &visibility, &alb.Quality, &parentId); err != nil {
			log.Println(err)
		} else {
			date, er := time.Parse(time.DateTime, alb.ReleaseDate)
			if er != nil {
				log.Println(er)
			} else {
				alb.SubTitle = fmt.Sprintf("%s   %s   Views: %d   Likes: %d", alb.GetSubTitle(), TimeAgo(date), alb.GetViewCount(), alb.GetLikeCount())
				if visibility == 1 {
					alb.SubTitle += "   👀"
				}
			}
			alb.ReleaseType = 3
			alb.ArtistIds = []string{parentId}
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

func DownloadVideos(ctx context.Context, vidIds []string, quality string) (map[string]string, error) {
	mDownloaded := make(map[string]string)

	for _, id := range vidIds {
		res := strings.Split(id, ";")
		if len(res) != 3 {
			log.Println("Invalid ui param:", id)
			continue
		}
		chId := res[0]
		videoId := res[1]
		title := res[2]

		mChannel := make(map[string]string)
		absChannelName, exist := mChannel[chId]
		if !exist {
			absChannelName = filepath.Join(YouDir, chId)
			err := os.MkdirAll(absChannelName, 0o755)
			if err != nil {
				log.Println(chId+" can't create folder.", err)
				continue
			}
			mChannel[chId] = absChannelName
		}

		/*ext := ".mp4"
		if quality == "audio" {
			ext = ".mp3"
		}

		filename := path.Clean(filepath.Join(absChannelName, title+ext))
		exists, err := FileExists(filename)
		if err != nil {
			log.Println(filename + " can't check if video already exists locally, skipped..")
			continue
		}
		if exists {
			fmt.Println(filename + " exists locally, skipped..")
			continue
		}*/

		resDown, err := DownloadVideo(ctx, absChannelName, videoId, quality)
		if err != nil {
			log.Println(title+" can't download.", err)
			continue
		} else {
			mDownloaded[id] = resDown
		}
	}

	return mDownloaded, nil
}
