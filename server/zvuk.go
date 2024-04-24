package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	slices2 "golang.org/x/exp/slices"

	"github.com/dustin/go-humanize"
	"github.com/machinebox/graphql"
	"github.com/v0vc/go-music-grpc/artist"
)

const (
	apiBase               = "https://zvuk.com/"
	apiRelease            = "api/tiny/releases"
	apiStream             = "api/tiny/track/stream"
	albumRegexString      = `^https://zvuk.com/release/(\d+)$`
	playlistRegexString   = `^https://zvuk.com/playlist/(\d+)$`
	artistRegexString     = `^https://zvuk.com/artist/(\d+)$`
	trackTemplateAlbum    = "{{.trackPad}}-{{.title}}"
	trackTemplatePlaylist = "{{.artist}} - {{.title}}"
	albumTemplate         = "{{.year}} - {{.album}}"
	releaseChunk          = 100
	authHeader            = "x-auth-token"
	thumbSize             = "64x64"
	coverSize             = "600x600"
)

type Transport struct{}

var (
	jar, _     = cookiejar.New(nil)
	client     = &http.Client{Jar: jar, Transport: &Transport{}}
	userAgents = []string{
		"OpenPlay|4.9.4|Android|7.1|HTC One X10",
		"OpenPlay|4.10.1|Android|7.1.2|Sony Xperia Z5",
		"OpenPlay|4.10.2|Android|7.1|Sony Xperia XZ",
		"OpenPlay|4.10.3|Android|7.1.2|Asus ASUS_Z01QD",
		"OpenPlay|4.11.2|Android|8|Nexus 6P",
		"OpenPlay|4.11.4|Android|8.1|Samsung Galaxy S6",
		"OpenPlay|4.11.5|Android|9|Samsung Galaxy S7",
		"OpenPlay|4.12.3|Android|10|Samsung Galaxy S8",
		"OpenPlay|4.13|Android|11|Samsung Galaxy S9",
		"OpenPlay|4.14|Android|12|Google Pixel 4 XL",
	}
	trackQualityMap = map[string]TrackQuality{
		"/stream?":   {"128 Kbps MP3", ".mp3", false},
		"/streamhq?": {"320 Kbps MP3", ".mp3", false},
		"/streamfl?": {"900 Kbps FLAC", ".flac", true},
	}
)

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", userAgents[rand.Int()%len(userAgents)])
	req.Header.Add("Referer", apiBase)
	return http.DefaultTransport.RoundTrip(req)
}

func getThumb(ctx context.Context, url string) []byte {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return []byte{}
	}

	defer response.Body.Close()
	res, err := io.ReadAll(response.Body)
	if err != nil {
		return []byte{}
	}
	return res
}

func downloadAlbumCover(ctx context.Context, url, path string) error {
	url = strings.Replace(url, "{size}", coverSize, 1)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}

	defer f.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if err != nil || response == nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return err
	}
	_, err = io.Copy(f, response.Body)
	return err
}

func downloadTrack(ctx context.Context, trackPath, url string) (string, error) {
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}

	defer f.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Range", "bytes=0-")
	do, err := client.Do(req)
	if err != nil || do == nil {
		return "", err
	}

	defer do.Body.Close()
	if do.StatusCode != http.StatusOK && do.StatusCode != http.StatusPartialContent {
		log.Println(do.Status)
		return "", err
	}
	totalBytes := do.ContentLength
	counter := &WriteCounter{
		Total:     totalBytes,
		TotalStr:  humanize.Bytes(uint64(totalBytes)),
		StartTime: time.Now().UnixMilli(),
	}
	res, err := io.Copy(f, io.TeeReader(do.Body, counter))

	log.Println("")

	return humanize.Bytes(uint64(res)), err
}

/*func runExec(tx *sql.Tx, ctx context.Context, ids []string, command string) {
	if ids != nil {
		stDelete, err := tx.PrepareContext(ctx, command)
		if err != nil {
			log.Fatal(err)
		}
		defer stDelete.Close()

		for _, id := range ids {
			_, _ = stDelete.ExecContext(ctx, id)
		}
	}
}*/

func getAlbumIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, albumId string) int {
	stmtAlb, err := tx.PrepareContext(ctx, "select aa.albumId from main.artistAlbum aa join album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aa.artistId where a.albumId = ? and ar.siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer stmtAlb.Close()

	var albId int
	err = stmtAlb.QueryRowContext(ctx, albumId, siteId).Scan(&albId)
	if err != nil {
		log.Println(err)
	}
	return albId
}

func getArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId string) int {
	stmtAlb, err := tx.PrepareContext(ctx, "select art_id from main.artist  where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer stmtAlb.Close()

	var artId int
	err = stmtAlb.QueryRowContext(ctx, artistId, siteId).Scan(&artId)
	if err != nil {
		log.Println(err)
	}
	return artId
}

func getExistIds(tx *sql.Tx, ctx context.Context, artId int) ([]string, []string) {
	var (
		existAlbumIds  []string
		existArtistIds []string
	)

	if artId != 0 {
		rows, err := tx.QueryContext(ctx, "select al.albumId, a.artistId res from main.artistAlbum aa join main.artist a on a.art_id = aa.artistId join album al on al.alb_id = aa.albumId where aa.albumId in (select albumId from main.artistAlbum where artistId = ?);", artId)
		if err != nil {
			log.Println(err)
		}

		defer rows.Close()

		for rows.Next() {
			var (
				albId   string
				artisId string
			)

			if er := rows.Scan(&albId, &artisId); er != nil {
				log.Println(er)
			}

			if albId != "" && !slices2.Contains(existAlbumIds, albId) {
				existAlbumIds = append(existAlbumIds, albId)
			}

			if artisId != "" && !slices2.Contains(existArtistIds, artisId) {
				existArtistIds = append(existArtistIds, artisId)
			}
		}
	}
	return existAlbumIds, existArtistIds
}

func getTrackFromDb(tx *sql.Tx, ctx context.Context, siteId uint32, ids []string, isAlbum bool) (map[string]*AlbumInfo, []string) {
	var sqlStr string

	if len(ids) == 1 {
		if isAlbum {
			sqlStr = "select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.albumId in (select alb_id from album where albumId = ? limit 1) and ar.siteId = ? group by at.trackId;"
		} else {
			sqlStr = "select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.trackId in (select trk_id from track where trackId = ? limit 1) and ar.siteId = ? group by at.trackId;"
		}
	} else {
		if isAlbum {
			sqlStr = fmt.Sprintf("select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.albumId in (select alb_id from album where albumId in (? %v)) and ar.siteId = ? group by at.trackId;", strings.Repeat(",?", len(ids)-1))
		} else {
			sqlStr = fmt.Sprintf("select group_concat(ar.title, ', '), a.title, a.albumId, a.releaseDate, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from main.albumTrack at join main.artistAlbum aa on at.albumId = aa.albumId join main.album a on a.alb_id = aa.albumId join main.artist ar on ar.art_id = aA.artistId join main.track t on t.trk_id = at.trackId where at.trackId in (select trk_id from track where trackId in (? %v)) and ar.siteId = ? group by at.trackId;", strings.Repeat(",?", len(ids)-1))
		}
	}

	stRows, err := tx.PrepareContext(ctx, sqlStr)
	if err != nil {
		log.Println(err)
	}
	defer stRows.Close()

	args := make([]interface{}, len(ids))
	for i, trackId := range ids {
		args[i] = trackId
	}

	args = append(args, siteId)
	rows, err := stRows.QueryContext(ctx, args...)
	if err != nil {
		log.Println(err)
	}

	defer rows.Close()
	mTracks := make(map[string]*AlbumInfo)

	var mAlbum []string

	for rows.Next() {
		var (
			trackId string
			alb     AlbumInfo
		)
		if er := rows.Scan(&alb.ArtistTitle, &alb.AlbumTitle, &alb.AlbumId, &alb.AlbumYear, &alb.AlbumCover, &trackId, &alb.TrackNum, &alb.TrackTotal, &alb.TrackTitle, &alb.TrackGenre); er != nil {
			log.Println(er)
		}
		_, ok := mTracks[trackId]
		if !ok {
			mTracks[trackId] = &alb
		}
		if isAlbum && !slices2.Contains(mAlbum, alb.AlbumId) {
			mAlbum = append(mAlbum, alb.AlbumId)
		}
	}

	return mTracks, mAlbum
}

func getTokenDb(tx *sql.Tx, ctx context.Context, siteId uint32) (string, string, string) {
	stmt, err := tx.PrepareContext(ctx, "select login, pass, token from main.site where site_id = ? limit 1;")
	if err != nil {
		log.Println(err)
	}
	defer stmt.Close()

	var (
		token string
		login string
		pass  string
	)
	err = stmt.QueryRowContext(ctx, siteId).Scan(&login, &pass, &token)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		log.Printf("no token for sourceId: %d", siteId)
	case err != nil:
		log.Println(err)
	}

	return login, pass, token
}

func updateTokenDb(tx *sql.Tx, ctx context.Context, token string, siteId uint32) {
	stmtUpdToken, err := tx.PrepareContext(ctx, "update main.site set token = ? where site_id = ?;")
	if err != nil {
		log.Println(err)
	}

	defer stmtUpdToken.Close()
	_, _ = stmtUpdToken.ExecContext(ctx, token, siteId)
}

func getTokenFromSite(ctx context.Context, email, password string) (string, error) {
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"api/tiny/login/email", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	do, err := client.Do(req)
	if err != nil || do == nil {
		return "", err
	}

	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", err
	}

	var obj *Auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil || obj == nil {
		return "", err
	}
	return obj.Result.Token, nil
}

func getTrackStreamUrl(ctx context.Context, trackId, trackQuality, token string) (string, error) {
	var do *http.Response
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+apiStream, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add(authHeader, token)
	query := url.Values{}
	query.Set("id", trackId)
	query.Set("quality", trackQuality)
	req.URL.RawQuery = query.Encode()

	for i := 0; i < 5; i++ {
		do, err = client.Do(req)
		if err != nil || do == nil {
			return "", err
		}
		if do.StatusCode == http.StatusTeapot && i != 4 {
			do.Body.Close()
			log.Printf("Got a HTTP 418, %d attempt(s) remaining.\n", 4-i)

			continue
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return "", err
		}

		break
	}
	if do == nil {
		return "", err
	}

	defer do.Body.Close()

	var obj *TrackStreamInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil || obj == nil {
		return "", err
	}
	return obj.Result.Stream, nil
}

func getCurrentTrackQuality(streamUrl string, qualityMap *map[string]TrackQuality) *TrackQuality {
	for k, v := range *qualityMap {
		if strings.Contains(streamUrl, k) {
			return &v
		}
	}
	return nil
}

func getAlbumTracks(ctx context.Context, albumId, token, email, password string) (*ReleaseInfo, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+apiRelease, nil)
	if err != nil {
		log.Println(err)
		return nil, "", false, err
	}

	req.Header.Add(authHeader, token)
	query := url.Values{}
	query.Set("ids", albumId)
	query.Set("include", "track")
	req.URL.RawQuery = query.Encode()
	do, err := client.Do(req)
	if err != nil || do == nil {
		log.Println(err)
		return nil, "", false, err
	}

	defer do.Body.Close()
	needTokenUpd := false

	switch do.StatusCode {
	case http.StatusTeapot:
		return nil, "", false, nil
	case http.StatusUnauthorized:
		log.Printf("Try to renew access token...")
		token, err = getTokenFromSite(ctx, email, password)
		if err == nil {
			log.Printf("Token was updated successfully.")
			needTokenUpd = true
		} else {
			log.Println("Can't get new token", err)
		}
		return nil, token, needTokenUpd, nil
	case http.StatusOK:
		var obj ReleaseInfo

		err = json.NewDecoder(do.Body).Decode(&obj)
		if err != nil {
			log.Println("Can't decode response from api: ", err)
		}
		return &obj, token, needTokenUpd, err
	default:
		return nil, "", false, err
	}
}

func getArtistReleases(ctx context.Context, artistId, token, email, password string) (*ArtistReleases, string, bool, error) {
	var obj ArtistReleases
	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { __typename artists { __typename id title image { __typename ...ImageInfoGqlFragment } } date id image { __typename ...ImageInfoGqlFragment } title type } fragment ImageInfoGqlFragment on ImageInfo { __typename src }`)
	graphqlRequest.Var("id", artistId)
	graphqlRequest.Var("limit", releaseChunk)
	graphqlRequest.Var("offset", 0)
	graphqlRequest.Header.Add(authHeader, token)

	var (
		needTokenUpd    = false
		graphqlResponse interface{}
	)
	err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse)
	if err != nil {
		log.Printf("try to renew access token...")
		token, err = getTokenFromSite(ctx, email, password)
		if err == nil {
			graphqlRequest.Header.Set(authHeader, token)
			err = graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse)
			if err != nil {
				log.Printf("can't get artist data from api: %v\n", err)
			} else {
				log.Printf("token was updated successfully")
				needTokenUpd = true
			}
		} else {
			log.Printf("can't get new token: %v\n", err)
		}
	}
	if err != nil {
		return nil, "", false, err
	} else {
		jsonString, er := json.Marshal(graphqlResponse)
		err = json.Unmarshal(jsonString, &obj)
		if err != nil {
			return nil, "", false, err
		}
		return &obj, token, needTokenUpd, er
	}
}

/*func SyncArtistSb(ctx context.Context, siteId uint32, artistId string, isAdd bool) ([]*artist.Artist, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	login, pass, token := getTokenDb(tx, ctx, siteId)
	item, token, needTokenUpd, err := getArtistReleases(ctx, artistId, token, login, pass)
	if item == nil || err != nil {
		log.Fatal(err)
	}
	if needTokenUpd {
		updateTokenDb(tx, ctx, token, siteId)
	}

	artRawId, userAddedDb := GetArtistIdDb(tx, ctx, siteId, artistId)
	existAlbumIds, existArtistIds := getExistIds(tx, ctx, artRawId)

	stArtistMaster, err := tx.PrepareContext(ctx, "insert into main.artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do update set userAdded = 1, thumbnail = ? returning art_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistMaster.Close()

	stArtistSlave, err := tx.PrepareContext(ctx, "insert into main.artist(siteId, artistId, title, thumbnail) values (?, ?, ?, ?) on conflict (siteId, artistId) do update set syncState = 0 returning art_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistSlave.Close()
	var syncState uint8
	if isAdd {
		syncState = 0
	} else {
		syncState = 1
	}

	stAlbum, err := tx.PrepareContext(ctx, "insert into main.album(albumId, title, releaseDate, releaseType, thumbnail, syncState) values (?, ?, ?, ?, ?, ?) on conflict (albumId, title) do update set syncState = 0 returning alb_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stAlbum.Close()

	stArtistAlbum, err := tx.PrepareContext(ctx, "insert into main.artistAlbum(artistId, albumId) values (?, ?) on conflict (artistId, albumId) do nothing;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistAlbum.Close()

	var (
		authors   []*artist.Artist
		albumIds  []string
		artistIds []string
	)
	mArtist := make(map[string]int)
	for _, data := range item.GetArtists {
		var (
			author *artist.Artist
			albums []*artist.Album
		)

		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			if !Contains(albumIds, release.ID) {
				albumIds = append(albumIds, release.ID)
			}

			thumbAlb := getThumb(strings.Replace(release.Image.Src, "{size}", thumbSize, 1))

			var albId int
			err = stAlbum.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, thumbAlb, syncState).Scan(&albId)
			if err != nil {
				log.Fatal(err)
			} else {
				fmt.Printf("processed album: %v, id: %v \n", release.Title, albId)
			}
			alb := &artist.Album{}
			if artRawId == 0 || userAddedDb == 0 || !Contains(existAlbumIds, release.ID) {
				alb.Id = int64(albId)
				alb.AlbumId = release.ID
				alb.Title = release.Title
				alb.ReleaseType = release.Type
				alb.ReleaseDate = release.Date
				alb.Thumbnail = thumbAlb

				var sb []string
				for _, artistData := range release.Artists {
					alb.ArtistIds = append(alb.ArtistIds, artistData.ID)
					sb = append(sb, artistData.Title)
					if !Contains(artistIds, artistData.ID) {
						artistIds = append(artistIds, artistData.ID)
					}
					artId, ok := mArtist[artistData.ID]
					if !ok {
						thumbArtUrl := strings.Replace(artistData.Image.Src, "{size}", thumbSize, 1)
						thumbArt := getThumb(thumbArtUrl)
						artistTitle := strings.TrimSpace(artistData.Title)
						userAdded := false
						if artistData.ID == artistId {
							err = stArtistMaster.QueryRowContext(ctx, siteId, artistData.ID, artistTitle, thumbArt, 1, thumbArt).Scan(&artId)
							userAdded = true
						} else {
							err = stArtistSlave.QueryRowContext(ctx, siteId, artistData.ID, artistTitle, nil).Scan(&artId)
						}
						if err != nil {
							log.Fatal(err)
						} else {
							fmt.Printf("processed artist: %v, id: %v \n", artistData.Title, artId)
						}
						// if artRawId == 0 || !Contains(existArtistIds, artistData.ID) {
						if userAdded {
							author = &artist.Artist{
								Id:        int64(artId),
								SiteId:    siteId,
								ArtistId:  artistId,
								Title:     artistTitle,
								Thumbnail: thumbArt,
								UserAdded: userAdded,
							}
						}
						mArtist[artistData.ID] = artId
					}

					if artId != 0 {
						_, _ = stArtistAlbum.ExecContext(ctx, artId, albId)
					}
				}
				alb.SubTitle = strings.Join(sb, ", ")
				albums = append(albums, alb)
			}
		}
		if author != nil && albums != nil {
			author.Albums = albums
			// author.NewAlbs = int32(len(albums))
			authors = append(authors, author)
		}
	}

	var (
		deletedAlbumIds  []string
		deletedArtistIds []string
	)
	if artRawId != 0 {
		deletedAlbumIds = FindDifference(existAlbumIds, albumIds)
		runExec(tx, ctx, deletedAlbumIds, "delete from main.album where albumId = ?;")
		deletedArtistIds = FindDifference(existArtistIds, artistIds)
		runExec(tx, ctx, deletedArtistIds, "delete from main.artist where artistId = ?;")
	}

	return authors, tx.Commit()
}*/

func SyncArtistSb(ctx context.Context, siteId uint32, artistId ArtistRawId, isAdd bool) (*artist.Artist, error) {
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

	login, pass, token := getTokenDb(tx, ctx, siteId)
	item, token, needTokenUpd, err := getArtistReleases(ctx, artistId.Id, token, login, pass)
	if item == nil || err != nil {
		log.Println(err)
		return resArtist, tx.Rollback()
	}
	if needTokenUpd {
		updateTokenDb(tx, ctx, token, siteId)
	}

	var (
		artRawId int
		uAdd     int
	)

	if artistId.RawId == 0 {
		artRawId, uAdd = GetArtistIdDb(tx, ctx, siteId, artistId.Id)
	} else {
		artRawId = artistId.RawId
		uAdd = 1
	}
	if uAdd == 1 && isAdd {
		// пытались добавить существующего, сделаем просто синк
		isAdd = false
	}

	var existArtistIds, existAlbumIds, netAlbumIds, netArtistIds []string
	mArtist := make(map[string]int)

	if artRawId != 0 {
		existAlbumIds, existArtistIds = getExistIds(tx, ctx, artRawId)
		mArtist[artistId.Id] = artRawId
	}

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			if !slices2.Contains(netAlbumIds, release.ID) {
				netAlbumIds = append(netAlbumIds, release.ID)
			}

			for _, author := range release.Artists {
				if author.ID == "" {
					continue
				}
				if !slices2.Contains(netArtistIds, author.ID) {
					netArtistIds = append(netArtistIds, author.ID)
				}
			}
		}
	}

	deletedArtistIds := FindDifference(existArtistIds, netArtistIds)
	log.Printf("siteId: %v, artistId: %d, deleted: %d\n", siteId, artRawId, len(deletedArtistIds))

	newAlbumIds := FindDifference(netAlbumIds, existAlbumIds)
	log.Printf("siteId: %v, artistId: %d, new albums: %d\n", siteId, artRawId, len(newAlbumIds))

	newArtistIds := FindDifference(netArtistIds, existArtistIds)
	log.Printf("siteId: %v, artistId: %d, new artists: %d\n", siteId, artRawId, len(newArtistIds))

	type artAlb struct {
		art int
		alb int
	}

	var (
		processedAlbumIds  []string
		processedArtistIds []string
		thumb              []byte
		artists            []*artist.Artist
		albums             []*artist.Album
	)

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			alb := &artist.Album{}
			if slices2.Contains(newAlbumIds, release.ID) && !slices2.Contains(processedAlbumIds, release.ID) {
				alb.AlbumId = release.ID
				alb.Title = strings.TrimSpace(release.Title)
				alb.ReleaseDate = release.Date
				alb.ReleaseType = MapReleaseType(release.Type)
				alb.Thumbnail = getThumb(ctx, strings.Replace(release.Image.Src, "{size}", thumbSize, 1))
				if !isAdd {
					alb.SyncState = 1
				}
				processedAlbumIds = append(processedAlbumIds, release.ID)
			} else {
				if isAdd {
					alb.Title = strings.TrimSpace(release.Title)
					alb.ReleaseDate = release.Date
					alb.ReleaseType = MapReleaseType(release.Type)
					alb.Thumbnail = getThumb(ctx, strings.Replace(release.Image.Src, "{size}", thumbSize, 1))
				}
			}

			if isAdd || alb.AlbumId != "" {
				sb := make([]string, len(release.Artists))
				for i, author := range release.Artists {
					if author.ID == "" {
						continue
					}
					sb[i] = author.Title
					alb.ArtistIds = append(alb.ArtistIds, author.ID)

					if slices2.Contains(newArtistIds, author.ID) && !slices2.Contains(processedArtistIds, author.ID) {
						art := &artist.Artist{
							ArtistId: author.ID,
							Title:    strings.TrimSpace(author.Title),
						}
						if isAdd && art.GetArtistId() == artistId.Id && art.Thumbnail == nil {
							art.Thumbnail = getThumb(ctx, strings.Replace(author.Image.Src, "{size}", thumbSize, 1))
							art.UserAdded = true
							resArtist = art
						}
						artists = append(artists, art)
						processedArtistIds = append(processedArtistIds, author.ID)
					} else if artRawId != 0 && author.ID == artistId.Id && thumb == nil && resArtist == nil {
						thumb = getThumb(ctx, strings.Replace(author.Image.Src, "{size}", thumbSize, 1))
						resArtist = &artist.Artist{
							SiteId:    siteId,
							ArtistId:  artistId.Id,
							Title:     author.Title,
							Thumbnail: thumb,
							UserAdded: true,
						}
					}
				}

				alb.SubTitle = strings.Join(sb, ", ")
				if !slices2.Contains(alb.ArtistIds, artistId.Id) {
					// api bug
					alb.ArtistIds = append(alb.ArtistIds, artistId.Id)
					if resArtist != nil {
						alb.SubTitle = fmt.Sprintf("%v, %v", alb.SubTitle, resArtist.Title)
					}
				}
				albums = append(albums, alb)
			}
		}
	}

	var (
		artAlbs []*artAlb
		albId   int
	)
	if artists != nil {
		stArtist, _ := tx.PrepareContext(ctx, "insert into main.artist(siteId, artistId, title) values (?,?,?) on conflict (siteId, artistId) do update set syncState = 1 returning art_id;")
		defer stArtist.Close()

		stArtistUser, _ := tx.PrepareContext(ctx, "insert into main.artist(siteId, artistId, title, userAdded, thumbnail) values (?,?,?,?,?) on conflict (siteId, artistId) do update set userAdded = 1 returning art_id;")
		defer stArtistUser.Close()

		for _, art := range artists {
			artId, ok := mArtist[art.GetArtistId()]
			if !ok {
				var insErr error
				if art.GetUserAdded() {
					insErr = stArtistUser.QueryRowContext(ctx, siteId, art.GetArtistId(), art.GetTitle(), 1, art.GetThumbnail()).Scan(&artId)
				} else {
					insErr = stArtist.QueryRowContext(ctx, siteId, art.GetArtistId(), art.GetTitle()).Scan(&artId)
				}
				if insErr != nil {
					log.Println(insErr)
				} else {
					log.Printf("processed artist: %v, id: %v \n", art.GetTitle(), artId)
				}
				mArtist[art.GetArtistId()] = artId
			}
		}
	}

	if albums != nil {
		stAlbum, _ := tx.PrepareContext(ctx, "insert into main.album(albumId, title, releaseDate, releaseType, thumbnail, syncState) values (?,?,?,?,?,?) on conflict (albumId, title) do update set syncState = 0 returning alb_id;")
		defer stAlbum.Close()

		for _, album := range albums {
			if album.GetAlbumId() != "" {
				err = stAlbum.QueryRowContext(ctx, album.GetAlbumId(), album.GetTitle(), album.GetReleaseDate(), album.GetReleaseType(), album.GetThumbnail(), album.GetSyncState()).Scan(&albId)
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("processed album: %v, id: %v \n", album.GetTitle(), albId)
				}

				for _, arId := range album.GetArtistIds() {
					artId, ok := mArtist[arId]
					if ok {
						artAlbs = append(artAlbs, &artAlb{
							art: artId,
							alb: albId,
						})
					} else {
						artId = getArtistIdDb(tx, ctx, siteId, arId)
						if artId != 0 {
							mArtist[arId] = artId
							artAlbs = append(artAlbs, &artAlb{
								art: artId,
								alb: albId,
							})
						}
					}
				}
			}
			if resArtist != nil {
				resArtist.Albums = append(resArtist.Albums, album)
			}
		}
	} else if artists != nil {
		log.Printf("siteId: %v, artistId: %d, new relations found, processing..\n", siteId, artRawId)
		mAlbum := make(map[string]int)

		for _, data := range item.GetArtists {
			for _, release := range data.Releases {
				for _, ar := range release.Artists {
					if release.ID == "" {
						continue
					}

					for _, art := range artists {
						if ar.ID == art.GetArtistId() {
							alId, ok := mAlbum[release.ID]
							if !ok {
								alId = getAlbumIdDb(tx, ctx, siteId, release.ID)
								mAlbum[release.ID] = alId
								artId, exist := mArtist[art.GetArtistId()]
								if exist {
									artAlbs = append(artAlbs, &artAlb{
										art: artId,
										alb: alId,
									})
								}
							} else {
								artId, exist := mArtist[art.GetArtistId()]
								if exist {
									artAlbs = append(artAlbs, &artAlb{
										art: artId,
										alb: alId,
									})
								}
							}
						}
					}
				}
			}
		}
	}

	if isAdd && artRawId != 0 && uAdd != 1 && thumb != nil {
		log.Printf("siteId: %v, artistId: %d, avatar has been updated\n", siteId, artRawId)
		stArtistUpd, _ := tx.PrepareContext(ctx, "update main.artist set userAdded = 1, thumbnail = ? where art_id = ?;")

		defer stArtistUpd.Close()
		_, err = stArtistUpd.ExecContext(ctx, thumb, artRawId)
		if err != nil {
			log.Println(err)
		}
	}

	if artAlbs != nil {
		log.Printf("siteId: %v, artistId: %d, relations: %d\n", siteId, artRawId, len(artAlbs))
		sqlStr := fmt.Sprintf("insert into main.artistAlbum(artistId, albumId) values %v on conflict (artistId, albumId) do nothing;", strings.TrimSuffix(strings.Repeat("(?,?),", len(artAlbs)), ","))
		stArtAlb, _ := tx.PrepareContext(ctx, sqlStr)

		defer stArtAlb.Close()

		var args []interface{}
		for _, artAl := range artAlbs {
			args = append(args, &artAl.art, &artAl.alb)
		}

		_, err = stArtAlb.ExecContext(ctx, args...)
		if err != nil {
			log.Println(err)
		}
	}

	for _, aid := range deletedArtistIds {
		aff, er := DeleteBase(ctx, tx, aid, siteId, false)
		if er != nil {
			log.Println(er)
		} else {
			log.Printf("deleted artist: %v, rows: %v \n", aid, aff)
		}
	}

	return resArtist, tx.Commit()
}

func SyncAlbumSb(ctx context.Context, siteId uint32, albumId string) ([]*artist.Track, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	login, pass, token := getTokenDb(tx, ctx, siteId)
	item, token, needTokenUpd, er := getAlbumTracks(ctx, albumId, token, login, pass)
	if er != nil {
		return nil, er
	}
	if needTokenUpd {
		updateTokenDb(tx, ctx, token, siteId)
	}
	if item == nil {
		return nil, nil
	}

	stTrack, err := tx.PrepareContext(ctx, "insert into main.track (trackId, trackNum, title, hasFlac, hasLyric, quality, condition, genre, duration) values (?, ?, ?, ?, ?, ?, ?, ?, ?) on conflict (trackId, title) do nothing returning trk_id;")
	if err != nil {
		log.Println(err)
	}
	defer stTrack.Close()

	var (
		tracks     []*artist.Track
		trackTotal = len(item.Result.Tracks)
	)

	if trackTotal > 0 {
		albId := getAlbumIdDb(tx, ctx, siteId, albumId)
		stAlbumUpd, er := tx.PrepareContext(ctx, "update main.album set trackTotal = ? where alb_id = ?;")
		if err != nil {
			log.Println(er)
		}

		defer stAlbumUpd.Close()
		_, _ = stAlbumUpd.ExecContext(ctx, trackTotal, albId)

		stAlbumTrackRem, er := tx.PrepareContext(ctx, "delete from main.track where trk_id in (select trackId from albumTrack where albumId = ?)")
		if er != nil {
			log.Println(er)
		}

		defer stAlbumTrackRem.Close()
		_, _ = stAlbumTrackRem.ExecContext(ctx, albId)

		stAlbumTrack, er := tx.PrepareContext(ctx, "insert into main.albumTrack(albumId, trackId) values (?, ?) on conflict (albumId, trackId) do nothing;")
		if er != nil {
			log.Println(er)
		}

		defer stAlbumTrack.Close()
		stTrackArtist, er := tx.PrepareContext(ctx, "insert into main.trackArtist(trackId, artistId) values (?, ?) on conflict (trackId, artistId) do nothing;")
		if er != nil {
			log.Println(er)
		}

		defer stTrackArtist.Close()
		mArtist := make(map[int]int)

		for trId, track := range item.Result.Tracks {
			if trId != "" {
				var trackId int
				err = stTrack.QueryRowContext(ctx, trId, track.Position, track.Title, track.HasFlac, track.Lyrics, track.HighestQuality, track.Condition, strings.Join(track.Genres, ", "), track.Duration).Scan(&trackId)
				if err != nil {
					log.Println(err)
				}
				if trackId != 0 {
					_, _ = stAlbumTrack.ExecContext(ctx, albId, trackId)

					for _, artistId := range track.ArtistIds {
						artId, ok := mArtist[artistId]
						if !ok {
							artRawId, _ := GetArtistIdDb(tx, ctx, siteId, artistId)
							if artRawId > 0 {
								mArtist[artistId] = artRawId
								artId = artRawId
							}
						}
						if artId != 0 {
							_, _ = stTrackArtist.ExecContext(ctx, trackId, artId)
						}
					}

					tracks = append(tracks, &artist.Track{
						Id:        int64(trackId),
						TrackId:   trId,
						Title:     track.Title,
						HasFlac:   track.HasFlac,
						HasLyric:  track.Lyrics,
						Quality:   track.HighestQuality,
						Condition: track.Condition,
						TrackNum:  int32(track.Position),
						Duration:  int32(track.Duration),
					})
				}
			}
		}
	}

	return tracks, tx.Commit()
}

func downloadFiles(ctx context.Context, trackId, token, trackQuality string, albInfo *AlbumInfo, mDownloaded map[string]string) {
	var coverPath string

	cdnUrl, err := getTrackStreamUrl(ctx, trackId, trackQuality, token)
	if err != nil {
		log.Println("Failed to get track info from api.", err)
		return
	}
	curQuality := getCurrentTrackQuality(cdnUrl, &trackQualityMap)
	if curQuality == nil {
		log.Println("The API returned an unsupported format.")
		return
	}
	mTrack := CreateTagsFromDb(albInfo)
	trTotal, _ := strconv.Atoi(albInfo.TrackTotal)
	albTemplate := albumTemplate
	trackTemplate := trackTemplateAlbum
	if trTotal == 1 {
		trackTemplate = trackTemplatePlaylist
		albTemplate = "{{.artist}}"
	}

	trackName := ParseTemplate(mTrack, trackTemplate)
	mAlbum := make(map[string]string)
	absAlbName, exist := mAlbum[albInfo.AlbumId]
	if !exist {
		albName := ParseTemplate(mTrack, albTemplate)
		if len(albName) > 120 {
			log.Println("Album folder was chopped as it exceeds 120 characters.")
			albName = albName[:120]
		}
		if trTotal == 1 {
			absAlbName = filepath.Join(DownloadDir, albName)
		} else {
			absAlbName = filepath.Join(DownloadDir, albInfo.ArtistTitle, albName)
		}

		err = os.MkdirAll(absAlbName, 0o755)
		if err != nil {
			log.Println("Failed to create folder.", err)
			return
		}

		coverPath = filepath.Join(absAlbName, "cover.jpg")
		err = downloadAlbumCover(ctx, albInfo.AlbumCover, coverPath)
		if err != nil {
			log.Println("Failed to download cover.", err)
			coverPath = ""
		}
		mAlbum[albInfo.AlbumId] = absAlbName
	}

	trackPath := filepath.Join(absAlbName, trackName+curQuality.Extension)
	exists, err := FileExists(trackPath)
	if err != nil {
		log.Println("Failed to check if track already exists locally.")
		return
	}

	if exists {
		log.Println("Track already exists locally.")
		return
	}

	log.Printf("Downloading track %s of %s: %s - %s\n", albInfo.TrackNum, albInfo.TrackTotal, albInfo.TrackTitle, curQuality.Specs)
	resDown, err := downloadTrack(ctx, trackPath, cdnUrl)
	if err != nil {
		log.Println("Failed to download track.", err)
		return
	}
	mDownloaded[trackId] = resDown

	err = WriteTags(trackPath, coverPath, curQuality.IsFlac, mTrack)
	if err != nil {
		log.Println("Failed to write tags.", err)
		return
	}

	if trTotal == 1 && coverPath != "" {
		err = os.Remove(coverPath)
		if err != nil {
			log.Println("Failed to delete cover.", err)
		}
	}
}

func DownloadTracksSb(ctx context.Context, siteId uint32, trackIds []string, trackQuality string) (map[string]string, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=false&cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	mTracks, _ := getTrackFromDb(tx, ctx, siteId, trackIds, false)
	_, _, token := getTokenDb(tx, ctx, siteId)
	err = tx.Rollback()
	if err != nil {
		log.Println(err)
	}
	mDownloaded := make(map[string]string)

	for _, trackId := range trackIds {
		albInfo, dbExist := mTracks[trackId]
		if dbExist {
			downloadFiles(ctx, trackId, token, trackQuality, albInfo, mDownloaded)
		} else {
			log.Println("Track not found in database, please sync")
			// нет в базе, можно продумать как формировать пути скачивания без данных в базе, типа лить в базовую папку без прохода по темплейтам альбома, хз
		}

		RandomPause(3, 7)
	}

	return mDownloaded, err
}

func DownloadAlbumSb(ctx context.Context, siteId uint32, albIds []string, trackQuality string) (map[string]string, error) {
	db, err := sql.Open(sqlite3, fmt.Sprintf("file:%v?_foreign_keys=false&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println(err)
	}

	mTracks, dbAlbums := getTrackFromDb(tx, ctx, siteId, albIds, true)
	login, pass, token := getTokenDb(tx, ctx, siteId)

	mDownloaded := make(map[string]string)

	notDbAlbumIds := FindDifference(albIds, dbAlbums)
	for _, albumId := range notDbAlbumIds {
		var tryCount int
	L1:
		item, tokenNew, needTokenUpd, er := getAlbumTracks(ctx, albumId, token, login, pass)
		if er != nil {
			return mDownloaded, er
		}
		if needTokenUpd {
			updateTokenDb(tx, ctx, tokenNew, siteId)
		}

		if item == nil {
			tryCount += 1
			if tryCount == 4 {
				continue
			}

			RandomPause(3, 7)

			goto L1
		}

		for trId, track := range item.Result.Tracks {
			if trId != "" {
				_, ok := mTracks[trId]
				if !ok {
					var alb AlbumInfo
					alb.AlbumId = strconv.Itoa(track.ReleaseID)
					alb.ArtistTitle = strings.Join(item.Result.Releases[alb.AlbumId].ArtistNames, ", ")
					alb.AlbumTitle = item.Result.Releases[alb.AlbumId].Title
					alb.AlbumYear = strconv.Itoa(item.Result.Releases[alb.AlbumId].Date)[:4]
					alb.AlbumCover = item.Result.Releases[alb.AlbumId].Image.Src
					alb.TrackNum = strconv.Itoa(track.Position)
					alb.TrackTotal = strconv.Itoa(len(item.Result.Tracks))
					alb.TrackTitle = track.Title
					alb.TrackGenre = strings.Join(track.Genres, ", ")
					mTracks[trId] = &alb
				}
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Println(err)
	}

	for trackId, albInfo := range mTracks {
		downloadFiles(ctx, trackId, token, trackQuality, albInfo, mDownloaded)
		RandomPause(3, 7)
	}
	return mDownloaded, err
}
