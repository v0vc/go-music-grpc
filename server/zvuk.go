package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/machinebox/graphql"
	"github.com/v0vc/go-music-grpc/artist"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const (
	megabyte              = 1000000
	apiBase               = "https://zvuk.com/"
	apiRelease            = "api/tiny/releases"
	albumRegexString      = `^https://zvuk.com/release/(\d+)$`
	playlistRegexString   = `^https://zvuk.com/playlist/(\d+)$`
	artistRegexString     = `^https://zvuk.com/artist/(\d+)$`
	trackTemplateAlbum    = "{{.trackPad}}-{{.title}}"
	trackTemplatePlaylist = "{{.artist}} - {{.title}}"
	albumTemplate         = "{{.albumArtist}} - {{.album}}"
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

func getArtistIdDb(tx *sql.Tx, ctx context.Context, siteId uint32, artistId string) int {
	stmtArt, err := tx.PrepareContext(ctx, "select art_id from artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtArt.Close()

	var artRawId int
	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("no artist with id %v", artistId)
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

func getTokenDb(tx *sql.Tx, ctx context.Context, siteId uint32) (string, string, string) {
	stmt, err := tx.PrepareContext(ctx, "select login, pass, token from site where site_id = ?;")
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
	var obj Auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.Result.Token, nil
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

func getArtistReleases(ctx context.Context, artistId, token, email, password string) (ArtistReleases, string, bool) {
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
	return obj, token, needTokenUpd
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

	stAlbum, err := tx.PrepareContext(ctx, "insert into album(albumId, title, releaseDate, releaseType, thumbnail) values (?, ?, ?, ?, ?) on conflict (albumId, title) do update set syncState = 0 returning alb_id;")
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
			err = stAlbum.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, nil).Scan(&albId)
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

	stTrack, err := tx.PrepareContext(ctx, "insert into track (trackId, trackNum, title, hasFlac, hasLyric, quality, condition, duration) values (?, ?, ?, ?, ?, ?, ?, ?) on conflict (trackId, title) do nothing returning trk_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stTrack.Close()

	stAlbumTrack, err := tx.PrepareContext(ctx, "insert into albumTrack(albumId, trackId) values (?, ?) on conflict (albumId, trackId) do nothing;")
	if err != nil {
		log.Fatal(err)
	}
	defer stAlbumTrack.Close()

	var tracks []*artist.Track
	if len(item.Result.Tracks) > 0 {
		stAlbumTrackRem, err := tx.PrepareContext(ctx, "delete from track where trk_id in (select trackId from albumTrack where albumId = ?)")
		if err != nil {
			log.Fatal(err)
		}
		defer stAlbumTrackRem.Close()
		_, _ = stAlbumTrackRem.ExecContext(ctx, albId)

		for trId, track := range item.Result.Tracks {
			if trId != "" {
				var trackId int
				err = stTrack.QueryRowContext(ctx, trId, track.Position, track.Title, track.HasFlac, track.Lyrics, track.HighestQuality, track.Condition, track.Duration).Scan(&trackId)
				if err != nil {
					log.Fatal(err)
				}
				if trackId != 0 {
					_, _ = stAlbumTrack.ExecContext(ctx, albId, trackId)
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
