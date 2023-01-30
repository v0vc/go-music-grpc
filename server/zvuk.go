package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/machinebox/graphql"
	"github.com/v0vc/go-music-grpc/artist"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	megabyte              = 1000000
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
	thumbSize             = "10x10"
)

type Transport struct{}

var (
	jar, _     = cookiejar.New(nil)
	client     = &http.Client{Jar: jar, Transport: &Transport{}}
	qualityMap = map[int]string{
		1: "mid",
		2: "high",
		3: "flac",
	}
)

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", "OpenPlay|4.14|Android|12|Google Pixel 4 XL")
	req.Header.Add("Referer", apiBase)
	return http.DefaultTransport.RoundTrip(req)
}

func getThumb(url string) []byte {
	response, err := http.Get(url)
	if err != nil || response.StatusCode != http.StatusOK {
		return []byte{}
	}
	defer response.Body.Close()
	res, err := io.ReadAll(response.Body)
	if err != nil {
		return []byte{}
	}
	return res
}

func downloadAlbumCover(url, path string) error {
	url = strings.Replace(url, "&size={size}", "", 1)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	req, err := client.Get(url)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return err
	}
	_, err = io.Copy(f, req.Body)
	return err
}

func downloadTrack(trackPath, url string) (string, error) {
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Range", "bytes=0-")
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK && do.StatusCode != http.StatusPartialContent {
		fmt.Println(do.Status)
		return "", err
	}
	totalBytes := do.ContentLength
	counter := &WriteCounter{
		Total:     totalBytes,
		TotalStr:  humanize.Bytes(uint64(totalBytes)),
		StartTime: time.Now().UnixMilli(),
	}
	res, err := io.Copy(f, io.TeeReader(do.Body, counter))
	fmt.Println("")
	return humanize.Bytes(uint64(res)), err
}

func runExec(tx *sql.Tx, ctx context.Context, ids []string, command string) {
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
}

func getAlbumIdDb(tx *sql.Tx, ctx context.Context, albId int64) string {
	stmtAlb, err := tx.PrepareContext(ctx, "select albumId from album where alb_id = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtAlb.Close()

	var albumId string
	err = stmtAlb.QueryRowContext(ctx, albId).Scan(&albumId)
	if err != nil {
		log.Fatal(err)
	}
	return albumId
}

func getArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId interface{}) int {
	stmtArt, err := tx.PrepareContext(ctx, "select art_id from artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtArt.Close()

	var artRawId int
	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId)
	switch {
	case err == sql.ErrNoRows:
		fmt.Printf("no artist with id %v", artistId)
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Printf("artist db id is %d \n", artRawId)
	}
	return artRawId
}

func getExistIds(tx *sql.Tx, ctx context.Context, artId int) ([]string, []string) {
	var existAlbumIds []string
	var existArtistIds []string

	if artId != 0 {
		rows, err := tx.QueryContext(ctx, "select al.albumId, a.artistId res from artistAlbum aa join artist a on a.art_id = aa.artistId join album al on al.alb_id = aa.albumId where aa.albumId in (select albumId from artistAlbum where artistId = ?);", artId)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var albId string
			var artisId string
			if err := rows.Scan(&albId, &artisId); err != nil {
				log.Fatal(err)
			}
			if albId != "" && !Contains(existAlbumIds, albId) {
				existAlbumIds = append(existAlbumIds, albId)
			}
			if artisId != "" && !Contains(existArtistIds, artisId) {
				existArtistIds = append(existArtistIds, artisId)
			}
		}
	}
	return existAlbumIds, existArtistIds
}

func getTokenWithTrackFromDb(ctx context.Context, siteId uint32, trackIds []string) (map[string]*AlbumInfo, string) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?cache=shared&mode=ro", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStr := fmt.Sprintf("select group_concat(ar.title), a.title, a.albumId, a.releaseDate, a.thumbnailUrl, t.trackId, t.trackNum, a.trackTotal, t.title, t.genre from albumTrack at join artistAlbum aa on at.albumId = aa.albumId join album a on a.alb_id = aa.albumId join artist ar on ar.art_id = aA.artistId join track t on t.trk_id = at.trackId where at.trackId in (select trk_id from track where trackId in (? %v)) and ar.siteId = ? group by at.trackId;", strings.Repeat(",?", len(trackIds)-1))
	stRows, err := db.PrepareContext(ctx, sqlStr)
	if err != nil {
		log.Fatal(err)
	}
	defer stRows.Close()

	var args []interface{}
	for _, trackId := range trackIds {
		args = append(args, trackId)
	}
	args = append(args, siteId)
	rows, err := stRows.QueryContext(ctx, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	mTracks := make(map[string]*AlbumInfo)
	for rows.Next() {
		var trackId string
		var alb AlbumInfo
		if err := rows.Scan(&alb.ArtistTitle, &alb.AlbumTitle, &alb.AlbumId, &alb.AlbumYear, &alb.AlbumCover, &trackId, &alb.TrackNum, &alb.TrackTotal, &alb.TrackTitle, &alb.TrackGenre); err != nil {
			log.Fatal(err)
		}
		_, ok := mTracks[trackId]
		if !ok {
			mTracks[trackId] = &alb
		}
	}

	stmt, err := db.PrepareContext(ctx, "select token from site where site_id = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var token string

	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)
	switch {
	case err == sql.ErrNoRows:
		log.Fatalf("no token for sourceId: %d", siteId)
	case err != nil:
		log.Fatal(err)
	}

	return mTracks, token
}

func getTokenDb(tx *sql.Tx, ctx context.Context, siteId uint32) (string, string, string) {
	stmt, err := tx.PrepareContext(ctx, "select login, pass, token from site where site_id = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var token string
	var login string
	var pass string
	err = stmt.QueryRowContext(ctx, siteId).Scan(&login, &pass, &token)
	switch {
	case err == sql.ErrNoRows:
		log.Fatalf("no token for sourceId: %d", siteId)
	case err != nil:
		log.Fatal(err)
	}
	return login, pass, token
}

func updateTokenDb(tx *sql.Tx, ctx context.Context, token string, siteId uint32) {
	stmtUpdToken, err := tx.PrepareContext(ctx, "update site set token = ? where site_id = ?;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtUpdToken.Close()
	_, _ = stmtUpdToken.ExecContext(ctx, token, siteId)
}

func getTokenFromSite(email, password string) (string, error) {
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)
	req, err := http.NewRequest(http.MethodPost, apiBase+"api/tiny/login/email", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", err
	}
	var obj *Auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.Result.Token, nil
}

func getTrackStreamUrl(trackId, trackQuality, token string) (string, error) {
	var do *http.Response
	req, err := http.NewRequest(http.MethodGet, apiBase+apiStream, nil)
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
		if err != nil {
			return "", err
		}
		if do.StatusCode == http.StatusTeapot && i != 4 {
			do.Body.Close()
			fmt.Printf("Got a HTTP 418, %d attempt(s) remaining.\n", 4-i)
			continue
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return "", err
		}
		break
	}
	var obj *TrackStreamInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	do.Body.Close()
	if err != nil {
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

func sanitize(filename string, isFolder bool) string {
	var regexStr string
	if isFolder {
		regexStr = `[:*?"><|]`
	} else {
		regexStr = `[\/:*?"><|]`
	}
	str := regexp.MustCompile(regexStr).ReplaceAllString(filename, "_")
	return strings.TrimRightFunc(str, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

func parseTemplate(tags map[string]string, defTemplate string) string {
	var buffer bytes.Buffer

	for {
		err := template.Must(template.New("").Parse(defTemplate)).Execute(&buffer, tags)
		if err == nil {
			break
		}
		buffer.Reset()
	}
	resPath := html.UnescapeString(buffer.String())
	return sanitize(resPath, false)
}

func getAlbumTracks(albumId, token, email, password string) (*ReleaseInfo, string, bool) {
	req, err := http.NewRequest(http.MethodGet, apiBase+apiRelease, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add(authHeader, token)
	query := url.Values{}
	query.Set("ids", albumId)
	query.Set("include", "track")
	req.URL.RawQuery = query.Encode()
	do, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer do.Body.Close()
	var needTokenUpd = false
	if do.StatusCode != http.StatusOK {
		log.Printf("try to renew access token...")
		token, err = getTokenFromSite(email, password)
		if err == nil {
			req.Header.Set(authHeader, token)
			do, err := client.Do(req)
			if err != nil {
				log.Fatalln("can't get album data from api: " + err.Error())
			} else {
				log.Printf("token was updated successfully")
				needTokenUpd = true
			}
			defer do.Body.Close()
		} else {
			log.Fatalln("can't get new token: " + err.Error())
		}
	}
	var obj ReleaseInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		log.Fatal(err)
	}
	return &obj, token, needTokenUpd
}

func getArtistReleases(ctx context.Context, artistId, token, email, password string) (*ArtistReleases, string, bool) {
	var obj ArtistReleases
	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { __typename artists { __typename id title image { __typename ...ImageInfoGqlFragment } } date id image { __typename ...ImageInfoGqlFragment } title type } fragment ImageInfoGqlFragment on ImageInfo { __typename src }`)
	graphqlRequest.Var("id", artistId)
	graphqlRequest.Var("limit", releaseChunk)
	graphqlRequest.Var("offset", 0)
	graphqlRequest.Header.Add(authHeader, token)

	var needTokenUpd = false
	var graphqlResponse interface{}
	err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse)
	if err != nil {
		log.Printf("try to renew access token...")
		token, err = getTokenFromSite(email, password)
		if err == nil {
			graphqlRequest.Header.Set(authHeader, token)
			err = graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse)
			if err != nil {
				log.Fatalln("can't get artist data from api: " + err.Error())
			} else {
				log.Printf("token was updated successfully")
				needTokenUpd = true
			}
		} else {
			log.Fatalln("can't get new token: " + err.Error())
		}
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	json.Unmarshal(jsonString, &obj)
	return &obj, token, needTokenUpd
}

func SyncArtistSb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Artist, []*artist.Album, []string, []string, string, int, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	login, pass, token := getTokenDb(tx, ctx, siteId)
	item, token, needTokenUpd := getArtistReleases(ctx, artistId, token, login, pass)
	if needTokenUpd {
		updateTokenDb(tx, ctx, token, siteId)
	}
	artRawId := getArtistIdDb(tx, ctx, siteId, artistId)
	existAlbumIds, existArtistIds := getExistIds(tx, ctx, artRawId)

	stArtistMaster, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do update set userAdded = 1 returning art_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistMaster.Close()

	stArtistSlave, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail) values (?, ?, ?, ?) on conflict (siteId, artistId) do update set syncState = 0 returning art_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistSlave.Close()

	stAlbum, err := tx.PrepareContext(ctx, "insert into album(albumId, title, releaseDate, releaseType, thumbnail, thumbnailUrl) values (?, ?, ?, ?, ?, ?) on conflict (albumId, title) do update set syncState = 0 returning alb_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stAlbum.Close()

	stArtistAlbum, err := tx.PrepareContext(ctx, "insert into artistAlbum(artistId, albumId) values (?, ?) on conflict (artistId, albumId) do nothing;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistAlbum.Close()

	var artistName string
	var artistRawId int
	var newArtists []*artist.Artist
	var newAlbums []*artist.Album
	var albumIds []string
	var artistIds []string

	mArtist := make(map[string]int)
	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			if !Contains(albumIds, release.ID) {
				albumIds = append(albumIds, release.ID)
			}

			//url := strings.ReplaceAll(release.Image.Src, "{size}", thumbSize)
			var albId int
			err = stAlbum.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, nil, release.Image.Src).Scan(&albId)
			if err != nil {
				log.Fatal(err)
			} else {
				fmt.Sprintf("upsert: %v, id: %v)", release.Title, albId)
			}
			if artRawId == 0 || !Contains(existAlbumIds, release.ID) {
				newAlbums = append(newAlbums, &artist.Album{
					Id:          int64(albId),
					AlbumId:     release.ID,
					Title:       release.Title,
					ReleaseType: release.Type,
					ReleaseDate: release.Date,
					Thumbnail:   nil,
				})
			}

			for _, artistData := range release.Artists {
				if !Contains(artistIds, artistData.ID) {
					artistIds = append(artistIds, artistData.ID)
				}
				artId, ok := mArtist[artistData.ID]
				if !ok {
					//thUrl := strings.ReplaceAll(artist.Image.Src, "{size}", thumbSize)

					artistTitle := strings.TrimSpace(artistData.Title)
					var userAdded = false
					if artistData.ID == artistId {
						err = stArtistMaster.QueryRowContext(ctx, siteId, artistData.ID, artistTitle, nil, 1).Scan(&artId)
						artistName = artistTitle
						artistRawId = artId
						userAdded = true
					} else {
						err = stArtistSlave.QueryRowContext(ctx, siteId, artistData.ID, artistTitle, nil).Scan(&artId)
					}
					if err != nil {
						log.Fatal(err)
					} else {
						fmt.Sprintf("upsert: %v, id: %v)", artistData.Title, artId)
					}
					if artRawId == 0 || !Contains(existArtistIds, artistData.ID) || userAdded {
						newArtists = append(newArtists, &artist.Artist{
							Id:        int64(artId),
							SiteId:    siteId,
							ArtistId:  artistId,
							Title:     artistTitle,
							Thumbnail: nil,
							UserAdded: userAdded,
						})
					}
					mArtist[artistData.ID] = artId
				}

				if artId != 0 && albId != 0 {
					_, _ = stArtistAlbum.ExecContext(ctx, artId, albId)
				}
			}
		}
	}

	var deletedAlbumIds []string
	var deletedArtistIds []string
	if artRawId != 0 {
		deletedAlbumIds = FindDifference(existAlbumIds, albumIds)
		runExec(tx, ctx, deletedAlbumIds, "delete from album where albumId = ?;")
		deletedArtistIds = FindDifference(existArtistIds, artistIds)
		runExec(tx, ctx, deletedArtistIds, "delete from artist where artistId = ?;")
	}
	return newArtists, newAlbums, deletedAlbumIds, deletedArtistIds, artistName, artistRawId, tx.Commit()
}

func SyncAlbumSb(ctx context.Context, siteId uint32, albId int64) ([]*artist.Track, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true&cache=shared&mode=rw", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	login, pass, token := getTokenDb(tx, ctx, siteId)
	albumId := getAlbumIdDb(tx, ctx, albId)
	item, token, needTokenUpd := getAlbumTracks(albumId, token, login, pass)
	if needTokenUpd {
		updateTokenDb(tx, ctx, token, siteId)
	}

	stTrack, err := tx.PrepareContext(ctx, "insert into track (trackId, trackNum, title, hasFlac, hasLyric, quality, condition, genre, duration) values (?, ?, ?, ?, ?, ?, ?, ?, ?) on conflict (trackId, title) do nothing returning trk_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stTrack.Close()

	var tracks []*artist.Track
	var trackTotal = len(item.Result.Tracks)
	if trackTotal > 0 {
		stAlbumUpd, err := tx.PrepareContext(ctx, "update album set trackTotal = ? where alb_id = ?;")
		if err != nil {
			log.Fatal(err)
		}
		defer stAlbumUpd.Close()
		_, _ = stAlbumUpd.ExecContext(ctx, trackTotal, albId)

		stAlbumTrackRem, err := tx.PrepareContext(ctx, "delete from track where trk_id in (select trackId from albumTrack where albumId = ?)")
		if err != nil {
			log.Fatal(err)
		}
		defer stAlbumTrackRem.Close()
		_, _ = stAlbumTrackRem.ExecContext(ctx, albId)

		stAlbumTrack, err := tx.PrepareContext(ctx, "insert into albumTrack(albumId, trackId) values (?, ?) on conflict (albumId, trackId) do nothing;")
		if err != nil {
			log.Fatal(err)
		}
		defer stAlbumTrack.Close()

		stTrackArtist, err := tx.PrepareContext(ctx, "insert into trackArtist(trackId, artistId) values (?, ?) on conflict (trackId, artistId) do nothing;")
		if err != nil {
			log.Fatal(err)
		}
		defer stTrackArtist.Close()

		mArtist := make(map[int]int)
		for trId, track := range item.Result.Tracks {
			if trId != "" {
				var trackId int
				err = stTrack.QueryRowContext(ctx, trId, track.Position, track.Title, track.HasFlac, track.Lyrics, track.HighestQuality, track.Condition, track.Genres[0], track.Duration).Scan(&trackId)
				if err != nil {
					log.Fatal(err)
				}
				if trackId != 0 {
					_, _ = stAlbumTrack.ExecContext(ctx, albId, trackId)

					for _, artistId := range track.ArtistIds {
						artId, ok := mArtist[artistId]
						if !ok {
							artRawId := getArtistIdDb(tx, ctx, siteId, artistId)
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

func DownloadTracksSb(ctx context.Context, siteId uint32, trackIds []string, trackQuality string) (map[string]string, error) {
	/*token := getOnlyTokenDb(ctx, siteId)*/
	mTracks, token := getTokenWithTrackFromDb(ctx, siteId, trackIds)

	qualityMap := map[string]TrackQuality{
		//mid, high, flac
		"/stream?":   {"128 Kbps MP3", ".mp3", false},
		"/streamhq?": {"320 Kbps MP3", ".mp3", false},
		"/streamfl?": {"900 Kbps FLAC", ".flac", true},
	}

	mDownloaded := make(map[string]string)
	mAlbum := make(map[string]string)
	for trackNum, trackId := range trackIds {
		cdnUrl, err := getTrackStreamUrl(trackId, trackQuality, token)
		if err != nil {
			fmt.Errorf(err.Error())
			continue
		}
		curQuality := getCurrentTrackQuality(cdnUrl, &qualityMap)
		if curQuality == nil {
			fmt.Println("The API returned an unsupported format.")
			continue
		}
		albInfo, ok := mTracks[trackId]
		if !ok {
			// нет в базе, можно продумать как формировать пути скачивания без данных в базе
		} else {
			trTotal, _ := strconv.Atoi(albInfo.TrackTotal)
			trNum, _ := strconv.Atoi(albInfo.TrackNum)
			mTrack := make(map[string]string)
			mTrack["artist"] = albInfo.ArtistTitle
			mTrack["year"] = albInfo.AlbumYear[:4]
			mTrack["album"] = albInfo.AlbumTitle
			mTrack["genre"] = albInfo.TrackGenre
			mTrack["title"] = albInfo.TrackTitle
			mTrack["track"] = albInfo.TrackNum
			mTrack["trackPad"] = fmt.Sprintf("%02d", trNum)
			mTrack["trackTotal"] = albInfo.TrackTotal

			albTemplate := albumTemplate
			trackTemplate := trackTemplateAlbum
			if trTotal == 1 {
				trackTemplate = trackTemplatePlaylist
				albTemplate = "{{.artist}}"
			}
			trackName := parseTemplate(mTrack, trackTemplate)
			var coverPath string

			absAlbName, exist := mAlbum[albInfo.AlbumId]
			if !exist {
				albName := parseTemplate(mTrack, albTemplate)
				if len(albName) > 120 {
					fmt.Println("Album folder was chopped as it exceeds 120 characters.")
					albName = albName[:120]
				}
				if trTotal == 1 {
					absAlbName = filepath.Join(DownloadDir, albName)
				} else {
					absAlbName = filepath.Join(DownloadDir, albInfo.ArtistTitle, albName)
				}

				err = os.MkdirAll(absAlbName, 0755)
				if err != nil {
					fmt.Println(err)
					continue
				}

				coverPath = filepath.Join(absAlbName, "cover.jpg")
				err = downloadAlbumCover(albInfo.AlbumCover, coverPath)
				if err != nil {
					fmt.Println(err)
					coverPath = ""
				}
				mAlbum[albInfo.AlbumId] = absAlbName
			}

			trackPath := filepath.Join(absAlbName, trackName+curQuality.Extension)
			exists, err := FileExists(trackPath)
			if err != nil {
				fmt.Println("Failed to check if track already exists locally.")
				continue
			}
			if exists {
				fmt.Println("Track already exists locally.")
				continue
			}

			fmt.Printf("Downloading track %d of %d: %s - %s\n", trackNum, len(trackIds), albInfo.TrackTitle, curQuality.Specs)
			resDown, err := downloadTrack(trackPath, cdnUrl)
			if err != nil {
				fmt.Println("Failed to download track.")
				continue
			}
			mDownloaded[trackId] = resDown
			if trTotal == 1 && coverPath != "" {
				err := os.Remove(coverPath)
				if err != nil {
					fmt.Println("Failed to delete cover.")
				}
			}
		}

	}
	return mDownloaded, nil
}
