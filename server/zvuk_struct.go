package main

type Auth struct {
	Result struct {
		Token string `json:"token,omitempty"`
	} `json:"result,omitempty"`
}

type ArtistReleases struct {
	GetArtists []struct {
		Typename string `json:"__typename,omitempty"`
		Releases []struct {
			Typename string `json:"__typename,omitempty"`
			Artists  []struct {
				Typename string `json:"__typename,omitempty"`
				ID       string `json:"id,omitempty"`
				Title    string `json:"title,omitempty"`
				Image    struct {
					Typename string `json:"__typename,omitempty"`
					Src      string `json:"src,omitempty"`
				} `json:"image,omitempty"`
			} `json:"artists,omitempty"`
			Date  string `json:"date,omitempty"`
			ID    string `json:"id,omitempty"`
			Image struct {
				Typename string `json:"__typename,omitempty"`
				Src      string `json:"src,omitempty"`
			} `json:"image,omitempty"`
			Title string `json:"title,omitempty"`
			Type  string `json:"type,omitempty"`
		} `json:"releases,omitempty"`
	} `json:"getArtists,omitempty"`
}

type Release struct {
	Image struct {
		Src           string `json:"src,omitempty"`
		Palette       string `json:"palette,omitempty"`
		PaletteBottom string `json:"palette_bottom,omitempty"`
	} `json:"image,omitempty"`
	SearchCredits string   `json:"search_credits,omitempty"`
	TrackIds      []int    `json:"track_ids,omitempty"`
	Credits       string   `json:"credits,omitempty"`
	Date          int      `json:"date,omitempty"`
	ID            int      `json:"id,omitempty"`
	GenreIds      []int    `json:"genre_ids,omitempty"`
	ArtistIds     []int    `json:"artist_ids,omitempty"`
	Title         string   `json:"title,omitempty"`
	SearchTitle   string   `json:"search_title,omitempty"`
	Explicit      bool     `json:"explicit,omitempty"`
	Availability  int      `json:"availability,omitempty"`
	ArtistNames   []string `json:"artist_names,omitempty"`
	LabelID       int      `json:"label_id,omitempty"`
	Template      string   `json:"template,omitempty"`
	HasImage      bool     `json:"has_image,omitempty"`
	Type          string   `json:"type,omitempty"`
	Price         int      `json:"price,omitempty"`
}

type Track struct {
	HasFlac        bool     `json:"has_flac,omitempty"`
	ReleaseID      int      `json:"release_id,omitempty"`
	Lyrics         bool     `json:"lyrics,omitempty"`
	Price          int      `json:"price,omitempty"`
	SearchCredits  string   `json:"search_credits,omitempty"`
	Credits        string   `json:"credits,omitempty"`
	Duration       int      `json:"duration,omitempty"`
	HighestQuality string   `json:"highest_quality,omitempty"`
	ID             int      `json:"id,omitempty"`
	Condition      string   `json:"condition,omitempty"`
	ArtistIds      []int    `json:"artist_ids,omitempty"`
	Genres         []string `json:"genres,omitempty"`
	Title          string   `json:"title,omitempty"`
	SearchTitle    string   `json:"search_title,omitempty"`
	Explicit       bool     `json:"explicit,omitempty"`
	ReleaseTitle   string   `json:"release_title,omitempty"`
	Availability   int      `json:"availability,omitempty"`
	ArtistNames    []string `json:"artist_names,omitempty"`
	Template       string   `json:"template,omitempty"`
	Position       int      `json:"position,omitempty"`
	Image          struct {
		Src           string `json:"src,omitempty"`
		Palette       string `json:"palette,omitempty"`
		PaletteBottom string `json:"palette_bottom,omitempty"`
	} `json:"image,omitempty"`
}

type Playlist struct {
	ImageUrl    string `json:"image_url,omitempty"`
	ImageUrlBig string `json:"image_url_big,omitempty"`
	Title       string `json:"title,omitempty"`
	TrackIds    []int  `json:"track_ids,omitempty"`
}

type ReleaseInfo struct {
	Result struct {
		Tracks    map[string]Track    `json:"tracks,omitempty"`
		Playlists map[string]Playlist `json:"playlists,omitempty"`
		Releases  map[string]Release  `json:"releases,omitempty"`
	} `json:"result,omitempty"`
}
