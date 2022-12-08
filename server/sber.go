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

type artistReleasesId struct {
	GetArtists []struct {
		Typename string `json:"__typename"`
		Releases []struct {
			Typename string `json:"__typename"`
			ID       string `json:"id"`
		} `json:"releases"`
	} `json:"getArtists"`
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

func getTokenWithArtisId(ctx context.Context, siteId uint32, id int64) (string, string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return "", "", err
	}
	defer db.Close()

	stmt, err := db.PrepareContext(ctx, "select token from site where site_id = ? limit 1;")
	if err != nil {
		return "", "", err
	}
	defer stmt.Close()

	var token string
	err = stmt.QueryRowContext(ctx, siteId).Scan(&token)
	if err != nil {
		log.Fatal(err)
	}

	stmtArt, err := db.PrepareContext(ctx, "select artistId from artist where art_id = ? limit 1;")
	if err != nil {
		return "", "", err
	}
	defer stmtArt.Close()

	var artId string
	err = stmtArt.QueryRowContext(ctx, id).Scan(&artId)
	if err != nil {
		log.Fatal(err)
	}
	return token, artId, nil
}

func getArtistReleasesIds(ctx context.Context, id int64) ([]string, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "select a.albumId from artistAlbum aa join album a on a.alb_id = aa.albumId where artistId = ?;", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var albId string
		if err := rows.Scan(&albId); err != nil {
			return nil, err
		}
		res = append(res, albId)
	}

	return res, nil
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

func insertArtistReleases(ctx context.Context, siteUid uint32, artistId string, item *artistReleases) (int64, string, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_foreign_keys=true", dbFile))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Fatal(err)
	}

	stArtist, err := tx.PrepareContext(ctx, "insert into artist(siteId, artistId, title, thumbnail, userAdded) values (?, ?, ?, ?, ?) on conflict (siteId, artistId) do update set userAdded=1 returning art_id;")
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

	mArtist := make(map[string]int)
	var artistName string
	var artRes int64

	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}

			//url := strings.ReplaceAll(release.Image.Src, "{size}", thumbSize)
			var albId int
			_ = stRelease.QueryRowContext(ctx, release.ID, strings.TrimSpace(release.Title), release.Date, release.Type, nil).Scan(&albId)

			for _, artist := range release.Artists {
				var artId int
				artId, ok := mArtist[artist.ID]
				if !ok {
					//thUrl := strings.ReplaceAll(artist.Image.Src, "{size}", thumbSize)
					title := strings.TrimSpace(artist.Title)
					var userAdded = 0
					if artist.ID == artistId {
						artistName = title
						userAdded = 1
						artRes = int64(artId)
					}
					_ = stArtist.QueryRowContext(ctx, siteUid, artist.ID, title, nil, userAdded).Scan(&artId)
					mArtist[artist.ID] = artId
				}
				if artId != 0 && albId != 0 {
					_, _ = stArtistAlbum.ExecContext(ctx, artId, albId)
				}
			}
		}
	}

	return artRes, artistName, tx.Commit()
}

func GetArtistSb(ctx context.Context, siteUid uint32, artistId string) (int64, string, error) {

	token, err := getToken(ctx, siteUid)
	if err != nil {
		log.Fatal(err)
	}
	// check empty token and expiration

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
	var obj artistReleases
	json.Unmarshal(jsonString, &obj)

	return insertArtistReleases(ctx, siteUid, artistId, &obj)
}

func SyncArtistSb(ctx context.Context, siteId uint32, artistId int64) ([]*artist.Artist, []*artist.Album, []string, error) {
	token, artId, err := getTokenWithArtisId(ctx, siteId, artistId)
	if err != nil {
		log.Fatal(err)
	}
	graphqlClient := graphql.NewClient(apiBase + "api/v1/graphql")
	graphqlRequest := graphql.NewRequest(`query getArtistReleases($id: ID!, $limit: Int!, $offset: Int!) { getArtists(ids: [$id]) { __typename releases(limit: $limit, offset: $offset) { __typename ...ReleaseGqlFragment } } } fragment ReleaseGqlFragment on Release { id }`)
	graphqlRequest.Var("id", artId)
	graphqlRequest.Var("limit", releaseChunk)
	graphqlRequest.Var("offset", 0)
	graphqlRequest.Header.Add(authHeader, token)
	var graphqlResponse interface{}
	if err := graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse); err != nil {
		log.Fatal(err)
	}
	jsonString, _ := json.Marshal(graphqlResponse)
	var item artistReleasesId
	json.Unmarshal(jsonString, &item)

	dbIds, err := getArtistReleasesIds(ctx, artistId)

	var inetIds []string
	for _, data := range item.GetArtists {
		for _, release := range data.Releases {
			if release.ID == "" {
				continue
			}
			inetIds = append(inetIds, release.ID)
		}
	}

	newIds := FindDifference(inetIds, dbIds)
	for i := range newIds {
		fmt.Println("new: " + newIds[i])
	}

	if len(newIds) > 0 {

	}

	deletedIds := FindDifference(dbIds, inetIds)
	for i := range deletedIds {
		fmt.Println("deleted: " + deletedIds[i])
	}

	return nil, nil, deletedIds, nil
}
