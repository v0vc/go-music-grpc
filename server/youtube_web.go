package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	youtubeApi      = "https://www.googleapis.com/youtube/v3/"
	chanelString    = "channels?id=[ID]&key=[KEY]&part=contentDetails,snippet,statistics&fields=items(contentDetails(relatedPlaylists(uploads)),snippet(title,thumbnails(default(url))),statistics(viewCount,subscriberCount))&prettyPrint=false"
	uploadString    = "playlistItems?key=[KEY]&playlistId=[ID]&part=snippet,contentDetails&order=date&fields=nextPageToken,items(snippet(publishedAt,title,resourceId(videoId),thumbnails(default(url))),contentDetails(videoPublishedAt))&maxResults=50&prettyPrint=false"
	statisticString = "videos?id=[VID]&key=[KEY]&part=contentDetails,statistics&fields=items(id,contentDetails(duration),statistics(viewCount,commentCount,likeCount))&prettyPrint=false"
)

func getChannel(ctx context.Context, channelId string, apiKey string) (*Channel, error) {
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

	var uploads *Uploads
	err = json.NewDecoder(response.Body).Decode(&uploads)
	if err != nil || uploads == nil {
		return new(Uploads), err
	}
	return uploads, nil
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

	var stat *Statistics
	err = json.NewDecoder(response.Body).Decode(&stat)
	if err != nil || stat == nil {
		return new(Statistics), err
	}
	return stat, nil
}
