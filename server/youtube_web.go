package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/lrstanley/go-ytdlp"
)

const (
	youtubeVideo        = "https://www.youtube.com/watch?v="
	youtubeApi          = "https://www.googleapis.com/youtube/v3/"
	chanelString        = "channels?id=[ID]&key=[KEY]&part=contentDetails,snippet,statistics&fields=items(contentDetails(relatedPlaylists(uploads)),snippet(title,thumbnails(default(url))),statistics(viewCount,subscriberCount))&prettyPrint=false"
	uploadString        = "playlistItems?key=[KEY]&playlistId=[ID]&part=snippet,contentDetails&order=date&fields=nextPageToken,items(snippet(publishedAt,title,resourceId(videoId),thumbnails(default(url))),contentDetails(videoPublishedAt))&maxResults=50&prettyPrint=false"
	playlistIdsString   = "playlistItems?key=[KEY]&playlistId=[ID]&part=snippet&fields=nextPageToken,items(snippet(resourceId(videoId)))&maxResults=50&prettyPrint=false"
	statisticString     = "videos?id=[VID]&key=[KEY]&part=contentDetails,statistics&fields=items(id,contentDetails(duration),statistics(viewCount,commentCount,likeCount))&prettyPrint=false"
	channelIdByVideoId  = "videos?id=[ID]&key=[KEY]&part=snippet&fields=items(snippet(channelId))&prettyPrint=false"
	channelIdByVideoIds = "videos?id=[ID]&key=[KEY]&part=snippet&fields=items(id,snippet(channelId))&prettyPrint=false"
	channelIdByHandle   = "channels?forHandle=[ID]&key=[KEY]&part=snippet&fields=items(id)&{PrintType}&prettyPrint=false"
	vidByIdsString      = "videos?id=[VID]&key=[KEY]&part=snippet,statistics,contentDetails&fields=items(id,contentDetails(duration),snippet(publishedAt,title,thumbnails(default(url))),statistics(viewCount,commentCount,likeCount))&prettyPrint=false"
	playlistByChannelId = "playlists?channelId=[ID]&key=[KEY]&part=snippet&fields=nextPageToken,items(id,snippet(title,thumbnails(default(url))))&maxResults=50&prettyPrint=false"
)

func GetChannelId(ctx context.Context, token string, id string) (string, error) {
	var url string
	if strings.HasPrefix(id, "@") {
		url = strings.Replace(strings.Replace(channelIdByHandle, "[ID]", id, 1), "[KEY]", token, 1)
	} else {
		url = strings.Replace(strings.Replace(channelIdByVideoId, "[ID]", id, 1), "[KEY]", token, 1)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	if strings.HasPrefix(id, "@") {
		var chId *ChannelIdHandle
		err = json.NewDecoder(response.Body).Decode(&chId)
		if err != nil || chId == nil {
			return "", err
		}
		return chId.Items[0].ID, nil
	} else {
		var chId *ChannelId
		err = json.NewDecoder(response.Body).Decode(&chId)
		if err != nil || chId == nil {
			return "", err
		}
		return chId.Items[0].Snippet.ChannelID, nil
	}
}

func GetChannelIdsByVid(ctx context.Context, token string, ids string, channelId string) (string, error) {
	url := strings.Replace(strings.Replace(channelIdByVideoIds, "[ID]", ids, 1), "[KEY]", token, 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var res *ChannelIdByVid
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil || res == nil {
		return "", err
	}

	var result []string
	for _, item := range res.Items {
		chId := item.Snippet.ChannelID
		if channelId != chId {
			continue
		}
		result = append(result, item.ID)
	}

	var sb strings.Builder
	for _, vid := range result {
		sb.WriteString(vid + ",")
	}

	return strings.TrimRight(sb.String(), ","), nil
}

func GetChannel(ctx context.Context, channelId string, apiKey string) (*Channel, error) {
	url := strings.Replace(strings.Replace(chanelString, "[ID]", channelId, 1), "[KEY]", apiKey, 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(Channel), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(Channel), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var channel *Channel
	err = json.NewDecoder(response.Body).Decode(&channel)
	if err != nil || channel == nil {
		return new(Channel), err
	}
	return channel, nil
}

func GetUploadVid(ctx context.Context, uploadId string, token string) []*vidItem {
	var videos []*vidItem
	urlRaw := strings.Replace(strings.Replace(uploadString, "[ID]", uploadId, 1), "[KEY]", token, 1)
	url := urlRaw
	i := 0
	for {
		upl, e := geUpload(ctx, url)
		if e == nil && upl != nil {
			var sb strings.Builder
			for _, vid := range upl.Items {
				sb.WriteString(vid.Snippet.ResourceID.VideoID + ",")
				videos = append(videos, &vidItem{
					id:            vid.Snippet.ResourceID.VideoID,
					title:         strings.TrimSpace(strings.ReplaceAll(vid.Snippet.Title, ";", ".")),
					published:     strings.TrimRight(strings.Replace(vid.Snippet.PublishedAt, "T", " ", 1), "Z"),
					thumbnailLink: vid.Snippet.Thumbnails.Default.URL,
				})
			}
			urlStat := strings.Replace(strings.Replace(statisticString, "[VID]", strings.TrimRight(sb.String(), ","), 1), "[KEY]", token, 1)
			stat, ere := geStatistics(ctx, urlStat)
			if ere == nil && stat != nil {
				for _, vid := range stat.Items {
					idx := slices.IndexFunc(videos, func(c *vidItem) bool { return c.id == vid.ID })
					if idx != -1 {
						videos[idx].duration = vid.ContentDetails.Duration
						videos[idx].likeCount = vid.Statistics.LikeCount
						videos[idx].viewCount = vid.Statistics.ViewCount
						videos[idx].commentCount = vid.Statistics.CommentCount
					} else {
						log.Println("Failed to find video ", vid.ID)
					}
				}
			}
		}
		if upl == nil || upl.NextPageToken == "" {
			fmt.Println("got no nextPageToken, all done")
			break
		} else {
			fmt.Println("nextPageToken: ", upl.NextPageToken)
			url = fmt.Sprintf("%s&pageToken=%s", urlRaw, upl.NextPageToken)
		}
		i++
	}
	return videos
}

func GetPlaylistVidIds(ctx context.Context, uploadId string, token string) []string {
	var netIds []string
	urlRaw := strings.Replace(strings.Replace(playlistIdsString, "[ID]", uploadId, 1), "[KEY]", token, 1)
	url := urlRaw
	i := 0
	for {
		upl, e := geUploadIds(ctx, url)
		if e == nil && upl != nil {
			for _, vid := range upl.Items {
				netIds = append(netIds, vid.Snippet.ResourceID.VideoID)
			}
		}
		if upl == nil || upl.NextPageToken == "" {
			fmt.Println("got no nextPageToken, all done")
			break
		} else {
			fmt.Println("nextPageToken: ", upl.NextPageToken)
			url = fmt.Sprintf("%s&pageToken=%s", urlRaw, upl.NextPageToken)
		}
		i++
	}
	return netIds
}

func GetVidByIds(ctx context.Context, vidIds string, token string) []*vidItem {
	var videos []*vidItem
	url := strings.Replace(strings.Replace(vidByIdsString, "[VID]", vidIds, 1), "[KEY]", token, 1)
	vid, err := getVidById(ctx, url)
	if err == nil && vid != nil {
		for _, vi := range vid.Items {
			videos = append(videos, &vidItem{
				id:            vi.ID,
				title:         vi.Snippet.Title,
				published:     strings.TrimRight(strings.Replace(vi.Snippet.PublishedAt, "T", " ", 1), "Z"),
				duration:      vi.ContentDetails.Duration,
				likeCount:     vi.Statistics.LikeCount,
				viewCount:     vi.Statistics.ViewCount,
				commentCount:  vi.Statistics.CommentCount,
				thumbnailLink: vi.Snippet.Thumbnails.Default.URL,
			})
		}
	}
	return videos
}

func GetPlaylists(ctx context.Context, channelId string, token string) []*plItem {
	var res []*plItem
	urlRaw := strings.Replace(strings.Replace(playlistByChannelId, "[ID]", channelId, 1), "[KEY]", token, 1)
	url := urlRaw
	i := 0
	for {
		upl, e := getPlaylist(ctx, url)
		if e == nil && upl != nil {
			for _, pl := range upl.Items {
				thumb := GetThumb(ctx, pl.Snippet.Thumbnails.Default.URL)
				if thumb != nil {
					thumb = PrepareThumb(thumb, 15, 64, 64, 90)
				}
				res = append(res, &plItem{
					id:        pl.ID,
					title:     pl.Snippet.Title,
					typePl:    1,
					thumbnail: thumb,
				})
			}
		}
		if upl == nil || upl.NextPageToken == "" {
			fmt.Println("got no nextPageToken, all done")
			break
		} else {
			fmt.Println("nextPageToken: ", upl.NextPageToken)
			url = fmt.Sprintf("%s&pageToken=%s", urlRaw, upl.NextPageToken)
		}
		i++
	}
	return res
}

func DownloadVideo(ctx context.Context, videoPath, id, quality string) (string, error) {
	install, err := ytdlp.Install(ctx, &ytdlp.InstallOptions{AllowVersionMismatch: true})
	if err != nil {
		return "-1", err
	}
	fmt.Println(install.Executable + ":" + install.Version)
	fmt.Println(id + " selected quality: " + quality)

	dl := ytdlp.New().
		FormatSort("res,ext:mp4:m4a").
		Format(quality).
		Output(videoPath + string(os.PathSeparator) + "%(title)s.%(ext)s")

	res, err := dl.Run(ctx, youtubeVideo+id)
	if err != nil {
		log.Println(err)
		return "-1", err
	}
	fmt.Println(res.String())

	return id, nil
}

func geUpload(ctx context.Context, url string) (*Uploads, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(Uploads), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(Uploads), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var res *Uploads
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil || res == nil {
		return new(Uploads), err
	}
	return res, nil
}

func geUploadIds(ctx context.Context, url string) (*UploadIds, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(UploadIds), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(UploadIds), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var res *UploadIds
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil || res == nil {
		return new(UploadIds), err
	}
	return res, nil
}

func geStatistics(ctx context.Context, url string) (*Statistics, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(Statistics), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(Statistics), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var res *Statistics
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil || res == nil {
		return new(Statistics), err
	}
	return res, nil
}

func getVidById(ctx context.Context, url string) (*VideoById, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(VideoById), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(VideoById), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var res *VideoById
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil || res == nil {
		return new(VideoById), err
	}
	return res, nil
}

func getPlaylist(ctx context.Context, url string) (*PlaylistByChannel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeApi+url, nil)
	if err != nil {
		return new(PlaylistByChannel), err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return new(PlaylistByChannel), err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(response.Body)

	var res *PlaylistByChannel
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil || res == nil {
		return new(PlaylistByChannel), err
	}
	return res, nil
}
