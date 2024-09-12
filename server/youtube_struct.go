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
			Title       string    `json:"title,omitempty"`
			ResourceID  struct {
				VideoID string `json:"videoId,omitempty"`
			} `json:"resourceId,omitempty"`
		} `json:"snippet,omitempty"`
	} `json:"items,omitempty"`
}

type Statistics struct {
	Items []struct {
		ID             string `json:"id,omitempty"`
		ContentDetails struct {
			Duration string `json:"duration,omitempty"`
		} `json:"contentDetails,omitempty"`
		Statistics struct {
			ViewCount    string `json:"viewCount,omitempty"`
			LikeCount    string `json:"likeCount,omitempty"`
			CommentCount string `json:"commentCount,omitempty"`
		} `json:"statistics,omitempty"`
	} `json:"items,omitempty"`
}
