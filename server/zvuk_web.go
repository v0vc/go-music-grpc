package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/v0vc/graphql"
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
	uaHeader              = "user-agent"
	thumbSize             = "64x64"
	coverSize             = "600x600"
	ua                    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14.3; rv:115.0) Gecko/20100101 Firefox/115.0"
)

type Transport struct{ auth string }

var (
	jar, _          = cookiejar.New(nil)
	trackQualityMap = map[string]TrackQuality{
		"/stream?":   {"128 Kbps MP3", ".mp3", false},
		"/streamhq?": {"320 Kbps MP3", ".mp3", false},
		"/streamfl?": {"900 Kbps FLAC", ".flac", true},
	}
)

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add(uaHeader, ua)
	if t.auth != "" {
		req.Header.Add("cookie", fmt.Sprintf("auth=%s", t.auth))
	}
	return http.DefaultTransport.RoundTrip(req)
}

/*func getTokenFromSite(ctx context.Context, email, password string) (string, error) {
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"api/tiny/login/email", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Jar: jar, Transport: &Transport{}}
	defer client.CloseIdleConnections()
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
}*/

func getAlbumTracks(ctx context.Context, albumId, token string) (*ReleaseInfo, error, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+apiRelease, nil)
	if err != nil {
		log.Println(err)
		return nil, err, false
	}

	query := url.Values{}
	query.Set("ids", albumId)
	query.Set("include", "track")
	req.URL.RawQuery = query.Encode()
	client := &http.Client{Jar: jar, Transport: &Transport{auth: token}}
	defer client.CloseIdleConnections()
	do, err := client.Do(req)
	if err != nil || do == nil {
		log.Println(err)
		return nil, err, false
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(do.Body)

	switch do.StatusCode {
	case http.StatusTeapot:
		log.Println("Got status: Teapot, too many requests, throttling started..")
		return nil, nil, true
	case http.StatusUnauthorized:
		log.Println("Try to renew access token...")
		return nil, nil, false
	case http.StatusForbidden:
		log.Println("Something was changed in api, please report. Exit...")
		return nil, nil, false
	case http.StatusOK:
		var obj ReleaseInfo

		err = json.NewDecoder(do.Body).Decode(&obj)
		if err != nil {
			log.Println("Can't decode response from api: ", err)
		}
		return &obj, err, true
	default:
		return nil, err, false
	}
}

func setGraphqlHeaders(req *graphql.Request, token string) {
	req.Header.Add(authHeader, token)
	req.Header.Add(uaHeader, ua)
	req.Header.Add("origin", apiBase)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("apollographql-client-version", "1.3")
	req.Header.Add("apollographql-client-name", "SberZvuk")
	req.Header.Add("accept-language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Add("accept", "*/*")
}

func getArtistReleases(ctx context.Context, artistId, token string) (*ArtistAlbums, error) {
	var (
		res         ArtistAlbums
		hasNextPage = true
		cursor      string
		err         error
	)

	for hasNextPage {
		graphqlRequest := graphql.NewRequest(`query artistReleases($ids: [ID!]!, $limit: Int = 100, $cursor: String = null) { getArtists(ids: $ids) { id title image { src } discography { all(limit: $limit, cursor: $cursor) { releases { id title type image { src } artists { id title } date } page_info { endCursor hasNextPage } } } } }`)
		graphqlRequest.Var("limit", releaseChunk)
		if cursor != "" {
			graphqlRequest.Var("cursor", cursor)
		} else {
			graphqlRequest.Var("cursor", nil)
		}
		graphqlRequest.Var("ids", []string{artistId})
		setGraphqlHeaders(graphqlRequest, token)

		graphqlClient := graphql.NewClient(apiBase+"api/v1/graphql", graphql.WithHTTPClient(&http.Client{Jar: jar, Transport: &Transport{}}))

		var graphqlResponse interface{}
		err = graphqlClient.Run(ctx, graphqlRequest, &graphqlResponse)
		if err != nil {
			return nil, err
		}

		jsonString, er := json.Marshal(graphqlResponse)
		if er != nil {
			return nil, er
		}

		var obj ArtistAlbums
		if res.GetArtists == nil {
			err = json.Unmarshal(jsonString, &res)
			if err != nil {
				return nil, err
			}
			if len(res.GetArtists) == 1 {
				hasNextPage = res.GetArtists[0].Discography.All.PageInfo.HasNextPage
				cursor = res.GetArtists[0].Discography.All.PageInfo.EndCursor
			} else {
				return nil, fmt.Errorf("bad api response for artist: %s", artistId)
			}

		} else {
			err = json.Unmarshal(jsonString, &obj)
			if err != nil {
				return nil, err
			}
			if len(res.GetArtists) == 1 {
				res.GetArtists[0].Discography.All.Releases = append(res.GetArtists[0].Discography.All.Releases, obj.GetArtists[0].Discography.All.Releases...)
				hasNextPage = obj.GetArtists[0].Discography.All.PageInfo.HasNextPage
				cursor = obj.GetArtists[0].Discography.All.PageInfo.EndCursor
			} else {
				return nil, fmt.Errorf("bad api response for artist: %s", artistId)
			}
		}
	}

	return &res, err
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
	if err != nil || cdnUrl == "" {
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
			absAlbName = filepath.Join(ZvukDir, strings.ReplaceAll(albName, ":", "_"))
		} else {
			absAlbName = filepath.Join(ZvukDir, strings.ReplaceAll(albInfo.ArtistTitle, ":", "_"), strings.ReplaceAll(albName, ":", "_"))
		}

		err = os.MkdirAll(absAlbName, 0o755)
		if err != nil {
			log.Println(trackName+" can't create folder.", err)
			return
		}

		coverPath = filepath.Join(absAlbName, "cover.jpg")
		err = downloadAlbumCover(ctx, albInfo.AlbumCover, coverPath)
		if err != nil {
			log.Println(trackName+" can't download cover.", err)
			coverPath = ""
		}
		mAlbum[albInfo.AlbumId] = absAlbName
	}

	trackPath := filepath.Join(absAlbName, trackName+curQuality.Extension)
	exists, err := FileExists(trackPath)
	if err != nil {
		fmt.Println(trackName + " can't check if track already exists locally, skipped..")
		return
	}

	if exists {
		fmt.Println(trackName + " exists locally, skipped..")
		return
	}

	fmt.Printf("Downloading track %s of %s: %s - %s\n", albInfo.TrackNum, albInfo.TrackTotal, albInfo.TrackTitle, curQuality.Specs)
	resDown, err := downloadTrack(ctx, trackPath, cdnUrl)
	if err != nil {
		log.Println(trackName+" can't download.", err)
		return
	}
	mDownloaded[trackId] = resDown

	err = WriteTags(trackPath, coverPath, curQuality.IsFlac, mTrack)
	if err != nil {
		log.Println(trackName+" can't write tags.", err)
		return
	}

	if trTotal == 1 && coverPath != "" {
		err = os.Remove(coverPath)
		if err != nil {
			log.Println(trackName+" can't delete cover.", err)
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
	client := &http.Client{Jar: jar, Transport: &Transport{}}
	defer client.CloseIdleConnections()
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

	fmt.Println()
	return humanize.Bytes(uint64(res)), err
}

func getTrackStreamUrl(ctx context.Context, trackId, trackQuality, token string) (string, error) {
	var do *http.Response
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+apiStream, nil)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	query.Set("id", trackId)
	query.Set("quality", trackQuality)
	req.URL.RawQuery = query.Encode()
	client := &http.Client{Jar: jar, Transport: &Transport{auth: token}}
	defer client.CloseIdleConnections()
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
			return "", fmt.Errorf("status code:  %d", do.StatusCode)
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
