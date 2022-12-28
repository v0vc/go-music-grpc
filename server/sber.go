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

func getToken(ctx context.Context, siteId uint32) (string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return "", err
	}
	defer db.Close()

	stmt, err := db.PrepareContext(ctx, "select token from site where site_id = ?;")
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var token string
	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)
	switch {
	case err == sql.ErrNoRows:
		log.Fatalf("no token for sourceId: %d", siteId)
		return "", err
	case err != nil:
		log.Fatal(err)
		return "", err
	default:
		return token, nil
	}
}

/*func getTokenWithArtisId(ctx context.Context, siteId uint32, artistId string) (string, int, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return "", -1, err
	}
	defer db.Close()

	stmt, err := db.PrepareContext(ctx, "select token from site where site_id = ? limit 1;")
	if err != nil {
		return "", -1, err
	}
	defer stmt.Close()

	var token string
	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)
	if err != nil {
		log.Fatal(err)
	}

	stmtArt, err := db.PrepareContext(ctx, "select art_id from artist where artistId = ? and siteId = ? limit 1;")
	if err != nil {
		return token, -1, err
	}
	defer stmtArt.Close()

	var artId int
	err = stmtArt.QueryRowContext(ctx, artistId, siteId).Scan(&artId)
	switch {
	case err == sql.ErrNoRows:
		// новый артист
		return token, artId, nil
	case err != nil:
		return token, artId, err
	default:
		return token, artId, nil
	}
}*/

func getArtistReleasesDbIds(ctx context.Context, id int) ([]string, []string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "select al.albumId, a.artistId res from artistAlbum aa join artist a on a.art_id = aa.artistId join album al on al.alb_id = aa.albumId where aa.albumId in (select albumId from artistAlbum where artistId = ?);", id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var albIds []string
	var artIds []string
	for rows.Next() {
		var albId string
		var artId string
		if err := rows.Scan(&albId, &artId); err != nil {
			return nil, nil, err
		}
		if !Contains(albIds, albId) {
			albIds = append(albIds, albId)
		}
		if !Contains(artIds, artId) {
			artIds = append(artIds, artId)
		}
	}

	return albIds, artIds, nil
}

func insertArtistReleases(ctx context.Context, siteUid uint32, artistId string, item *artistReleases) ([]*artist.Artist, []*artist.Album, []string, []string, string, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	/*stArtist, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do update set userAdded=1 returning art_id;")*/
	stArtist, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do nothing returning art_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtist.Close()

	stRelease, err := tx.PrepareContext(ctx, "insert into album(albumId, title, releaseDate, releaseType, thumbnail) values (?, ?, ?, ?, ?) on conflict (albumId, title) do nothing returning alb_id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRelease.Close()

	stArtistAlbum, err := tx.PrepareContext(ctx, "insert into artistAlbum(artistId, albumId) values (?, ?) on conflict (artistId, albumId) do nothing;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistAlbum.Close()

	var artistName string
	var newArtists []*artist.Artist
	var newAlbums []*artist.Album
	var existAlbumIds []string
	var existArtistIds []string
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
			err = stRelease.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, nil).Scan(&albId)
			switch {
			case err == sql.ErrNoRows:
				existAlbumIds = append(existAlbumIds, release.ID)
			default:
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
				var artId int
				artId, ok := mArtist[artistData.ID]
				if !ok {
					//thUrl := strings.ReplaceAll(artist.Image.Src, "{size}", thumbSize)
					title := strings.TrimSpace(artistData.Title)
					var userAdded = 0
					if artistData.ID == artistId {
						artistName = title
						userAdded = 1
					}
					err = stArtist.QueryRowContext(ctx, siteUid, artistData.ID, title, nil, userAdded).Scan(&artId)
					switch {
					case err == sql.ErrNoRows:
						existArtistIds = append(existArtistIds, artistData.ID)
					default:
						newArtists = append(newArtists, &artist.Artist{
							Id:        int64(artId),
							SiteId:    siteUid,
							ArtistId:  artistData.ID,
							Title:     artistData.Title,
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
	deletedAlbumIds := FindDifference(existAlbumIds, albumIds)
	deletedArtistIds := FindDifference(existArtistIds, artistIds)
	return newArtists, newAlbums, deletedAlbumIds, deletedArtistIds, artistName, tx.Commit()
}

func SyncArtistSb(ctx context.Context, siteId uint32, artistId string) ([]*artist.Artist, []*artist.Album, []string, []string, string, error) {
	/*token, artId, err := getTokenWithArtisId(ctx, siteId, artistId)*/
	token, err := getToken(ctx, siteId)
	if err != nil {
		log.Fatal(err)
	}

	item := getArtistReleases(ctx, artistId, token)

	return insertArtistReleases(ctx, siteId, artistId, &item)
}
