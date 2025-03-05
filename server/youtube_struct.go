package main

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
			PublishedAt string `json:"publishedAt,omitempty"`
			Title       string `json:"title,omitempty"`
			Thumbnails  struct {
				Default struct {
					URL string `json:"url,omitempty"`
				} `json:"default,omitempty"`
			}
			ResourceID struct {
				VideoID string `json:"videoId,omitempty"`
			} `json:"resourceId,omitempty"`
		} `json:"snippet,omitempty"`
	} `json:"items,omitempty"`
}

type UploadIds struct {
	NextPageToken string `json:"nextPageToken,omitempty"`
	Items         []struct {
		Snippet struct {
			ResourceID struct {
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

type ChannelId struct {
	Items []struct {
		Snippet struct {
			ChannelID string `json:"channelId,omitempty"`
		} `json:"snippet,omitempty"`
	} `json:"items,omitempty"`
}

type ChannelIdByVid struct {
	Items []struct {
		ID      string `json:"id,omitempty"`
		Snippet struct {
			ChannelID string `json:"channelId,omitempty"`
		} `json:"snippet,omitempty"`
	} `json:"items,omitempty"`
}

type ChannelIdHandle struct {
	Items []struct {
		ID string `json:"id,omitempty"`
	} `json:"items,omitempty"`
}

type VideoById struct {
	Items []struct {
		ID      string `json:"id,omitempty"`
		Snippet struct {
			PublishedAt string `json:"publishedAt,omitempty"`
			Title       string `json:"title,omitempty"`
			Thumbnails  struct {
				Default struct {
					URL string `json:"url,omitempty"`
				} `json:"default,omitempty"`
			} `json:"thumbnails,omitempty"`
		} `json:"snippet,omitempty"`
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

type PlaylistByChannel struct {
	NextPageToken string `json:"nextPageToken,omitempty"`
	Items         []struct {
		ID      string `json:"id,omitempty"`
		Snippet struct {
			Title      string `json:"title,omitempty"`
			Thumbnails struct {
				Default struct {
					URL string `json:"url,omitempty"`
				} `json:"default,omitempty"`
			} `json:"thumbnails,omitempty"`
		} `json:"snippet,omitempty"`
	} `json:"items,omitempty"`
}

type vidItem struct {
	id            string
	title         string
	published     string
	duration      string
	likeCount     string
	viewCount     string
	commentCount  string
	thumbnailLink string
}

type plItem struct {
	id        string
	title     string
	thumbnail []byte
	typePl    int
	rawId     int
}
