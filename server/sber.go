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

type artistReleases struct {
	GetArtists []struct {
		Typename string `json:"__typename"`
		Releases []struct {
			Typename string `json:"__typename"`
			Artists  []struct {
				Typename string `json:"__typename"`
				ID       string `json:"id"`
				Title    string `json:"title"`
				Image    struct {
					Typename string `json:"__typename"`
					Src      string `json:"src"`
				} `json:"image"`
			} `json:"artists"`
			Date  string `json:"date"`
			ID    string `json:"id"`
			Image struct {
				Typename string `json:"__typename"`
				Src      string `json:"src"`
			} `json:"image"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"releases"`
	} `json:"getArtists"`
}

type auth struct {
	Result struct {
		Token string `json:"token"`
	} `json:"result"`
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
	var obj auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.Result.Token, nil
}

func getArtistReleases(ctx context.Context, artistId, token, email, password string) (artistReleases, string, bool) {
	var obj artistReleases
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
			if err == nil {
				log.Printf("token was updated successfully")
				needTokenUpd = true
			} else {
				log.Fatal("can't get artist data from api: " + err.Error())
			}
		} else {
			log.Fatal("can't update token: " + err.Error())
		}
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	json.Unmarshal(jsonString, &obj)
	return obj, token, needTokenUpd
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

	item, token, needTokenUpd := getArtistReleases(ctx, artistId, token, login, pass)
	if needTokenUpd {
		stmtUpdToken, err := tx.PrepareContext(ctx, "update site set token = ? where site_id = ?;")
		if err != nil {
			log.Fatal(err)
		}
		defer stmtUpdToken.Close()
		_, _ = stmtUpdToken.ExecContext(ctx, token, siteId)
	}

	var existAlbumIds []string
	var existArtistIds []string

	stmtArt, err := db.PrepareContext(ctx, "select art_id from artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtArt.Close()

	var artRawId int
	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artRawId)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("no artist with id %v, inserting..", artistId)
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Printf("artist db id is %d \n", artRawId)
	}

	if artRawId != 0 {
		rows, err := db.QueryContext(ctx, "select al.albumId, a.artistId res from artistAlbum aa join artist a on a.art_id = aa.artistId join album al on al.alb_id = aa.albumId where aa.albumId in (select albumId from artistAlbum where artistId = ?);", artRawId)
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
				fmt.Println("upsert album: " + release.Title)
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
						fmt.Println("upsert artist: " + artistData.Title)
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
