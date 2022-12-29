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

func getArtistReleases(ctx context.Context, artistId string, token string) artistReleases {
	var obj artistReleases
	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { __typename artists { __typename id title image { __typename ...ImageInfoGqlFragment } } date id image { __typename ...ImageInfoGqlFragment } title type } fragment ImageInfoGqlFragment on ImageInfo { __typename src }`)
	graphqlRequest.Var("id", artistId)
	graphqlRequest.Var("limit", releaseChunk)
	graphqlRequest.Var("offset", 0)
	graphqlRequest.Header.Add(authHeader, token)

	var graphqlResponse interface{}
	if err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse); err != nil {
		log.Fatal(err)
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	json.Unmarshal(jsonString, &obj)
	return obj
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
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := tx.PrepareContext(ctx, "select token from site where site_id = ?;")
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

	item := getArtistReleases(ctx, artistId, token)

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
		log.Printf("artist db id is %d\n", artRawId)
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

	stArtist, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do update set lastDate=datetime(CURRENT_TIMESTAMP, 'localtime') returning art_id;")
	/*stArtist, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do nothing returning art_id;")*/
	if err != nil {
		log.Fatal(err)
	}
	defer stArtist.Close()

	stAlbum, err := tx.PrepareContext(ctx, "insert into album(albumId, title, releaseDate, releaseType, thumbnail) values (?, ?, ?, ?, ?) on conflict (albumId, title) do update set lastDate=datetime(CURRENT_TIMESTAMP, 'localtime') returning alb_id;")
	/*stAlbum, err := tx.PrepareContext(ctx, "insert into album(albumId, title, releaseDate, releaseType, thumbnail) values (?, ?, ?, ?, ?) on conflict (albumId, title) do nothing returning alb_id;")*/
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
			_ = stAlbum.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, nil).Scan(&albId)

			if artRawId == 0 {
				newAlbums = append(newAlbums, &artist.Album{
					Id:          int64(albId),
					AlbumId:     release.ID,
					Title:       release.Title,
					ReleaseType: release.Type,
					ReleaseDate: release.Date,
					Thumbnail:   nil,
				})
			} else if !Contains(existAlbumIds, release.ID) {
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

					var userAdded = 0
					artistTitle := strings.TrimSpace(artistData.Title)
					if artistData.ID == artistId {
						artistName = artistTitle
						userAdded = 1
					}
					_ = stArtist.QueryRowContext(ctx, siteId, artistData.ID, artistTitle, nil, userAdded).Scan(&artId)
					if userAdded == 1 {
						artistRawId = artId
					}
					if artRawId == 0 {
						newArtists = append(newArtists, &artist.Artist{
							Id:        int64(artId),
							SiteId:    siteId,
							ArtistId:  artistData.ID,
							Title:     artistTitle,
							Counter:   0,
							Thumbnail: nil,
						})
					} else if !Contains(existArtistIds, artistData.ID) {
						newArtists = append(newArtists, &artist.Artist{
							Id:        int64(artId),
							SiteId:    siteId,
							ArtistId:  artistData.ID,
							Title:     artistTitle,
							Counter:   0,
							Thumbnail: nil,
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
