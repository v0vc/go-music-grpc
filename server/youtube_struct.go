package main

import "time"

type Channel struct {
	Items []struct {
		Snippet struct {
			Title      string `json:"title,omitempty"`
			Thumbnails struct {
				Default struct {
					URL string `json:"url,omitempty"`
				} `json:"default,omitempty"`
			} `json:"thumbnails,omitempty"`
		} `json:"snippet,omitempty"`
		ContentDetails struct {
			RelatedPlaylists struct {
				Likes   string `json:"likes,omitempty"`
				Uploads string `json:"uploads,omitempty"`
			} `json:"relatedPlaylists,omitempty"`
		} `json:"contentDetails,omitempty"`
		Statistics struct {
			ViewCount       string `json:"viewCount,omitempty"`
			SubscriberCount string `json:"subscriberCount,omitempty"`
		} `json:"statistics,omitempty"`
	} `json:"items,omitempty"`
}

type Uploads struct {
	NextPageToken string `json:"nextPageToken,omitempty"`
	Items         []struct {
		Snippet struct {
			PublishedAt time.Time `json:"publishedAt,omitempty"`
			ChannelID   string    `json:"channelId,omitempty"`
			Title       string    `json:"title,omitempty"`
			ResourceID  struct {
				VideoID string `json:"videoId,omitempty"`
			} `json:"resourceId,omitempty"`
		} `json:"snippet,omitempty"`
		ContentDetails struct {
			VideoPublishedAt time.Time `json:"videoPublishedAt,omitempty"`
		} `json:"contentDetails,omitempty"`
	} `json:"items,omitempty"`
}
