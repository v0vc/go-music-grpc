package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/machinebox/graphql"
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

func getTokenSber(ctx context.Context, dbFile string, siteId uint32) (string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return "", err
	}
	defer db.Close()

	stmt, err := db.PrepareContext(ctx, "SELECT token FROM site WHERE id=? LIMIT 1;")
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var token string
	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)
	if err != nil {
		return "", err
	}
	return token, nil
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

func insertArtistReleases(ctx context.Context, siteUid uint32, artistId string, item *artistReleases) (string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	stArtist, err := tx.PrepareContext(ctx, "INSERT INTO artist(siteId, artistId, title, thumbnail, userAdded) VALUES(?, ?, ?, ?, ?) ON CONFLICT(siteId, artistId) DO UPDATE SET userAdded=1 RETURNING id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtist.Close()

	stRelease, err := tx.PrepareContext(ctx, "INSERT INTO album(albumId, title, releaseDate, releaseType, thumbnail) VALUES(?, ?, ?, ?, ?) ON CONFLICT(albumId, title) DO NOTHING RETURNING id;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRelease.Close()

	stArtistAlbum, err := tx.PrepareContext(ctx, "INSERT INTO artistAlbum(artistId, albumId) VALUES(?, ?) ON CONFLICT(artistId, albumId) DO NOTHING;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistAlbum.Close()

	mArtist := make(map[string]int)
	var artistName string

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}

			url := strings.ReplaceAll(release.Image.Src, "{size}", thumbSize)
			var albId int
			_ = stRelease.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, getThumb(url)).Scan(&albId)

			for _, artist := range release.Artists {
				var artId int
				artId, ok := mArtist[artist.ID]
				if !ok {
					thUrl := strings.ReplaceAll(artist.Image.Src, "{size}", thumbSize)
					title := strings.TrimSpace(artist.Title)
					var userAdded = 0
					if artist.ID == artistId {
						artistName = title
						userAdded = 1
					}
					_ = stArtist.QueryRowContext(ctx, siteUid, artist.ID, title, getThumb(thUrl), userAdded).Scan(&artId)
					mArtist[artist.ID] = artId
				}
				if artId != 0 && albId != 0 {
					_, _ = stArtistAlbum.ExecContext(ctx, artId, albId)
				}
			}
		}
	}

	return artistName, tx.Commit()
}

func GetArtistFromSber(ctx context.Context, item *artistItem) (string, error) {
	token, err := getTokenSber(ctx, dbFile, item.SiteId)
	if err != nil {
		log.Fatal(err)
	}
	// check empty token and expiration

	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { __typename artists { __typename id title image { __typename ...ImageInfoGqlFragment } } date id image { __typename ...ImageInfoGqlFragment } title type } fragment ImageInfoGqlFragment on ImageInfo { __typename src }`)
	graphqlRequest.Var("id", item.ArtistId)
	graphqlRequest.Var("limit", releaseChunk)
	graphqlRequest.Var("offset", 0)
	graphqlRequest.Header.Add(authHeader, token)

	var graphqlResponse interface{}
	if err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse); err != nil {
		log.Fatal(err)
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	var obj artistReleases
	json.Unmarshal(jsonString, &obj)

	return insertArtistReleases(ctx, item.SiteId, item.ArtistId, &obj)
}
