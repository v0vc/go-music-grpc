package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/machinebox/graphql"
	"log"
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
	authHeader            = "x-auth-token"
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
			} `json:"artists"`
			Availability int    `json:"availability"`
			Date         string `json:"date"`
			Explicit     bool   `json:"explicit"`
			ID           string `json:"id"`
			Image        struct {
				Typename      string `json:"__typename"`
				Src           string `json:"src"`
				Palette       string `json:"palette"`
				PaletteBottom string `json:"paletteBottom"`
			} `json:"image"`
			Label struct {
				Typename string `json:"__typename"`
				ID       string `json:"id"`
			} `json:"label"`
			SearchTitle    string `json:"searchTitle"`
			ArtistTemplate string `json:"artistTemplate"`
			Title          string `json:"title"`
			Tracks         []struct {
				Typename string `json:"__typename"`
				ID       string `json:"id"`
			} `json:"tracks"`
			Type string `json:"type"`
		} `json:"releases"`
	} `json:"getArtists"`
}

type artistReleasesId struct {
	GetArtists []struct {
		Typename string `json:"__typename"`
		Releases []struct {
			Typename string `json:"__typename"`
			ID       string `json:"id"`
		} `json:"releases"`
	} `json:"getArtists"`
}

func getTokenSber(ctx context.Context, dbFile string, siteId uint32) (string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return "", err
	}
	defer db.Close()

	stmt, err := db.PrepareContext(ctx, "select token from site where id=? limit 1")
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

func insertArtistReleases(ctx context.Context, item *artistReleases) error {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	stArtist, err := tx.PrepareContext(ctx, "INSERT INTO artist(siteId, artistId, title) VALUES(?, ?, ?) ON CONFLICT(siteId, artistId) DO NOTHING;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtist.Close()

	stRelease, err := tx.PrepareContext(ctx, "INSERT INTO album(albumId, title, releaseDate, releaseType) VALUES(?, ?, ?, ?) ON CONFLICT(albumId, title) DO NOTHING;")
	if err != nil {
		log.Fatal(err)
	}
	defer stRelease.Close()

	stArtistAlbum, err := tx.PrepareContext(ctx, "INSERT INTO artistAlbum(artistId, albumId) VALUES(?, ?) ON CONFLICT(artistId, albumId) DO NOTHING;")
	if err != nil {
		log.Fatal(err)
	}
	defer stArtistAlbum.Close()

	mArtist := make(map[string]int64)

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			res, err := stRelease.ExecContext(ctx, release.ID, release.Title, release.Date, release.Type)
			if err != nil {
				log.Fatal(err)
			}

			for _, artist := range release.Artists {
				resArt, err := stArtist.ExecContext(ctx, 1, artist.ID, strings.TrimSpace(artist.Title))
				art, _ := resArt.LastInsertId()
				if err != nil {
					log.Fatal(err)
				} else {
					mArtist[artist.ID] = art
				}

				alb, _ := res.LastInsertId()
				_, err = stArtistAlbum.ExecContext(ctx, art, alb)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	return err
}

func uniqueStrings(stringSlices ...[]string) []string {
	uniqueMap := map[string]bool{}

	for _, intSlice := range stringSlices {
		for _, number := range intSlice {
			uniqueMap[number] = true
		}
	}

	result := make([]string, 0, len(uniqueMap))

	for key := range uniqueMap {
		result = append(result, key)
	}

	return result
}

func getArtistReleases(ctx context.Context, artistId string, token string) error {
	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { __typename artists { __typename id title } availability date explicit id image { __typename ...ImageInfoGqlFragment } label { __typename id } searchTitle artistTemplate title tracks { __typename id } type } fragment ImageInfoGqlFragment on ImageInfo { __typename src palette paletteBottom }`)
	graphqlRequest.Var("id", artistId)
	graphqlRequest.Var("limit", 100)
	graphqlRequest.Var("offset", 0)
	graphqlRequest.Header.Add(authHeader, token)
	var graphqlResponse interface{}
	if err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse); err != nil {
		panic(err)
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	var obj artistReleases
	json.Unmarshal(jsonString, &obj)
	return insertArtistReleases(ctx, &obj)
}

func GetArtistFromSber(ctx context.Context, item *artistItem) {
	token, err := getTokenSber(ctx, dbFile, item.SiteId)
	if err != nil {
		log.Fatal(err)
	}
	// check empty token and expiration

	getArtistReleases(ctx, item.ArtistId, token)
}

func getArtistAlbumId(ctx context.Context, artistId string, limit int, offset int, token string) []string {
	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`
				query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { id }
			`)
	graphqlRequest.Var("id", artistId)
	graphqlRequest.Var("limit", limit)
	graphqlRequest.Var("offset", offset)
	graphqlRequest.Header.Add(authHeader, token)
	var graphqlResponse interface{}
	if err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse); err != nil {
		panic(err)
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	var obj artistReleasesId
	json.Unmarshal(jsonString, &obj)

	var albumsIds = make([]string, 0)
	if len(obj.GetArtists) > 0 {
		for _, element := range obj.GetArtists[0].Releases {
			if element.ID != "" {
				albumsIds = append(albumsIds, element.ID)
			}
		}
	}
	return albumsIds
}

func getArtistAllAlbumId(ctx context.Context, artistId string, token string) []string {
	firstFifty := getArtistAlbumId(ctx, artistId, 50, 0, token)
	lastFifty := getArtistAlbumId(ctx, artistId, 50, 49, token)
	return uniqueStrings(firstFifty, lastFifty)
}

func GetArtist(ctx context.Context, item *artistItem, token string) {
	releaseIds := getArtistAllAlbumId(ctx, item.ArtistId, token)
	for _, id := range releaseIds {
		fmt.Printf(id)
	}
}
