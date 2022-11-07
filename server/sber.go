package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/machinebox/graphql"
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
			ID       string `json:"id"`
		} `json:"releases"`
	} `json:"getArtists"`
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
	var obj artistReleases
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
	ids := getArtistAllAlbumId(ctx, item.ArtistId, token)
	for _, id := range ids {
		fmt.Printf(id)
	}
}
