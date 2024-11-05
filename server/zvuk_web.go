package main

import (
	"context"
	"encoding/json"
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

	"github.com/dustin/go-humanize"

	"github.com/machinebox/graphql"
)

const (
	apiBase               = "https://zvuk.com/"
	apiRelease            = "api/tiny/releases"
	apiStream             = "api/tiny/track/stream"
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

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(do.Body)
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

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(do.Body)

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
	case http.StatusForbidden:
		log.Printf("Try to renew access token...")
		token, err = getTokenFromSite(ctx, email, password)
		if err == nil && token != "" {
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

func downloadAlbumCover(ctx context.Context, url, path string) error {
	url = strings.Replace(url, "{size}", coverSize, 1)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}

	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Println(err)
		}
	}(f)
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

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)
	if response.StatusCode != http.StatusOK {
		return err
	}
	_, err = io.Copy(f, response.Body)
	return err
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

	fmt.Printf("Downloading track %s of %s: %s - %s\n", albInfo.TrackNum, albInfo.TrackTotal, albInfo.TrackTitle, curQuality.Specs)
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

func downloadTrack(ctx context.Context, trackPath, url string) (string, error) {
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}

	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Println(err)
		}
	}(f)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Range", "bytes=0-")
	do, err := client.Do(req)
	if err != nil || do == nil {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(do.Body)
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
			err = do.Body.Close()
			if err != nil {
				return "", err
			}
			log.Printf("Got a HTTP 418, %d attempt(s) remaining.\n", 4-i)

			continue
		}
		if do.StatusCode != http.StatusOK {
			err = do.Body.Close()
			if err != nil {
				return "", err
			}
			return "", err
		}

		break
	}
	if do == nil {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(do.Body)

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
