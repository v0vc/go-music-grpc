package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/v0vc/go-music-grpc/gio-gui/list"
	"github.com/v0vc/go-music-grpc/gio-gui/model"
)

// Rooms contains a selectable list of rooms.
type Rooms struct {
	active  int
	changed bool
	List    []*Room
	sync.Mutex
}

// Room is a unique conversation context.
// Note(jfm): Allocates model and interact, not sure about that.
// Avoids the UI needing to allocate two lists (interact/model) for the
// rooms.
type Room struct {
	// Room model defines the backend data describing a room.
	*model.Room
	// Interact defines the interactive state for a room widget.
	Interact Channel
	// RowTracker implements what would be a backend data model.
	// This would be the facade to your business api.
	// This is the source of truth.
	// This type gets asked to create messages and queried for message history.
	RowTracker, RowTrackerPl *RowTracker
	// ListState dynamically manages list state.
	// This lets us surf across a vast ocean of infinite messages, only ever
	// rendering what is actually viewable.
	// The widget.List consumes this during layout.
	ListState, ListStatePl *list.Manager
	// List implements the raw scrolling, adding scrollbars and responding
	// to mousewheel / touch fling gestures.
	List, ListPl widget.List
	// Editor contains the edit buffer for composing messages.
	Editor widget.Editor
	sync.Mutex
	// searchCurSeq    int
	// SearchResponses chan []list.Serial
}

func AddAlbumsToUi(rooms *Rooms, artMap map[string][]model.Message, channel *Room, start time.Time) {
	go func() {
		if channel.IsBase {
			for _, c := range rooms.List {
				if !c.IsBase {
					c.Loaded = false
				}
			}
		}
		for artId, albums := range artMap {
			ch := rooms.GetChannelById(artId)
			if ch != nil {
				ch.Lock()
				curCount, _ := strconv.Atoi(ch.Count)
				ch.Count = strconv.Itoa(curCount + len(albums))

				if ch.IsBase || ch.RowTracker.Rows != nil {
					el := make([]list.Element, 0, len(albums))
					for _, alb := range albums {
						el = append(el, alb)
						ch.RowTracker.Add(alb)
					}

					ch.ListState.Modify(el, nil, nil)
				}
				ch.Unlock()
			}
		}
		channel.Content = fmt.Sprintf("last sync %s", time.Since(start))
	}()
}

func (r *Room) RunSearch(searchText string) {
	input := strings.ToLower(searchText)
	resp := make([]list.Serial, 0)
	for _, i := range r.RowTracker.Rows {
		e := i.(model.Message)
		if input == "" {
			resp = append(resp, e.Serial())
		} else {
			if strings.Contains(strings.ToLower(e.Title), input) {
				resp = append(resp, e.Serial())
			}
		}
	}
	for _, i := range resp {
		r.RowTracker.Delete(i)
	}
	r.ListState.Modify(nil, nil, resp)
}

// DeleteRow removes the row with the provided serial from both the
// row tracker and the list manager for the room.
func (r *Room) DeleteRow(serial list.Serial) {
	r.RowTracker.Delete(serial)

	go r.ListState.Modify(nil, nil, []list.Serial{serial})
}

func (r *Rooms) DeleteChannel(index int, siteId uint32) []*Room {
	channel := r.Index(index)
	r.Lock()
	defer r.Unlock()

	if len(r.List) != 0 {
		go channel.RowTracker.Generator.DeleteArtist(siteId, channel.Id)
		return slices.Delete(r.List, index, index+1)
	}

	return make([]*Room, 0)
}

// Active returns the active room, empty if not rooms are available.
func (r *Rooms) Active() *Room {
	r.Lock()
	defer r.Unlock()

	if len(r.List) == 0 {
		return &Room{}
	}
	return r.List[r.active]
}

func (r *Room) DownloadAlbum(siteId uint32, albumId []string, trackQuality string, isPl bool) {
	r.Lock()
	defer r.Unlock()

	r.Content = "Downloading..."
	ids := r.RowTracker.Generator.DownloadAlbum(siteId, albumId, trackQuality, isPl)
	albs := len(ids)
	switch albs {
	case 0:
		r.Content = "All items exist locally"
	case 1:
		r.Content = fmt.Sprintf("%d item downloaded", albs)
	default:
		r.Content = fmt.Sprintf("%d items downloaded", albs)
	}
}

func (r *Room) DownloadArtist(siteId uint32, artistId string, trackQuality string) {
	r.Lock()
	defer r.Unlock()

	go r.RowTracker.Generator.DownloadArtist(siteId, artistId, trackQuality)
}

func (r *Room) ClearSync(siteId uint32) {
	r.Lock()
	defer r.Unlock()

	res := r.RowTracker.Generator.ClearSync(siteId)
	r.Content = fmt.Sprintf("New items cleared: %d", res)
	deleted := make([]list.Serial, 0, len(r.RowTracker.Rows))

	for _, i := range r.RowTracker.Rows {
		r.RowTracker.Delete(i.Serial())
		deleted = append(deleted, i.Serial())
	}
	r.RowTracker.Rows = nil

	go r.ListState.Modify(nil, nil, deleted)
}

func (r *Room) SyncArtist(rooms *Rooms, siteId uint32) {
	r.Lock()
	defer r.Unlock()
	r.Content = "In progress..."

	arts := make(chan map[string][]model.Message, 1)
	start := time.Now()

	go r.RowTracker.Generator.SyncArtist(siteId, r.Id, arts)

	res := <-arts
	AddAlbumsToUi(rooms, res, r, start)
}

func (r *Rooms) SelectAndFill(siteId uint32, index int, albs []model.Message, pls []model.Message, invalidator func(), presentRow func(data list.Element, state interface{}) layout.Widget, presentRowPl func(data list.Element, state interface{}) layout.Widget, isClean bool) {
	r.Lock()
	defer r.Unlock()

	if index < 0 {
		index = 0
	}
	if index > len(r.List) {
		index = len(r.List) - 1
	}

	r.changed = true
	r.List[r.active].Interact.Active = false
	r.active = index
	r.List[r.active].Interact.Active = true

	if r.List[r.active].Loaded {
		return
	}

	channel := r.List[r.active]

	if isClean {
		resp := make([]list.Serial, 0)
		for _, i := range channel.RowTracker.Rows {
			resp = append(resp, i.Serial())
		}
		channel.RowTracker.DeleteAll()
		channel.ListState.Modify(nil, nil, resp)
	}

	if albs == nil {
		if channel.IsBase {
			albs = channel.RowTracker.Generator.GetNewAlbums(siteId)
			count := len(albs)
			if count > 0 {
				channel.Count = strconv.Itoa(count)
			}
		} else {
			albs, pls = channel.RowTracker.Generator.GetArtistAlbums(siteId, r.List[r.active].Id)
		}
	}
	res := make([]list.Element, 0)
	for _, j := range albs {
		res = append(res, j)
	}
	channel.RowTracker.AddAll(res)
	lm := list.NewManager(len(albs),
		list.Hooks{
			// Define an allocator function that can instantiate the appropriate
			// state type for each kind of row data in our list.
			Allocator: func(data list.Element) interface{} {
				switch data.(type) {
				case model.Message:
					return &Row{}
				default:
					return nil
				}
			},
			// Define a presenter that can transform each kind of row data
			// and state into a widget.
			Presenter: presentRow,
			// NOTE(jfm): award coupling between message data and `list.Manager`.
			Loader:      channel.RowTracker.Load,
			Synthesizer: synth,
			Comparator:  rowLessThan,
			Invalidator: invalidator,
		},
	)
	// lm.Stickiness = list.After
	lm.Stickiness = list.Before
	channel.ListState = lm

	// таб с плейлистами, для NEW можно подумать насчет синтетических плейлистов (planned и тд)
	resPl := make([]list.Element, 0)
	for _, j := range pls {
		resPl = append(resPl, j)
	}
	channel.RowTrackerPl.AddAll(resPl)

	lmPl := list.NewManager(len(pls),
		list.Hooks{
			// Define an allocator function that can instantiate the appropriate
			// state type for each kind of row data in our list.
			Allocator: func(data list.Element) interface{} {
				switch data.(type) {
				case model.Message:
					return &Row{}
				default:
					return nil
				}
			},
			// Define a presenter that can transform each kind of row data
			// and state into a widget.
			Presenter: presentRowPl,
			// NOTE(jfm): award coupling between message data and `list.Manager`.
			Loader:      channel.RowTrackerPl.Load,
			Synthesizer: synth,
			Comparator:  rowLessThan,
			Invalidator: invalidator,
		},
	)
	lmPl.Stickiness = list.Before
	channel.ListStatePl = lmPl

	channel.Loaded = true
}

// Index returns a pointer to a Room at the given index.
// Index is bounded by [0, len(rooms)).
func (r *Rooms) Index(index int) *Room {
	r.Lock()
	defer r.Unlock()

	if index < 0 {
		index = 0
	}

	if index == len(r.List) {
		index = len(r.List) - 1
	}

	return r.List[index]
}

func (r *Rooms) GetChannelById(artistId string) *Room {
	r.Lock()
	defer r.Unlock()

	for _, channel := range r.List {
		if channel.Id == artistId {
			return channel
		}
	}
	return nil
}

func (r *Rooms) GetBaseChannel() *Room {
	r.Lock()
	defer r.Unlock()

	for _, channel := range r.List {
		if channel.IsBase {
			return channel
		}
	}
	return nil
}

// synth inserts date separators and unread separators
// between chat rows as a list.Synthesizer.
func synth(previous, row, _ list.Element) []list.Element {
	var out []list.Element
	asMessage, ok := row.(model.Message)
	if !ok {
		out = append(out, row)
		return out
	}

	if previous == nil {
		if !asMessage.Read {
			out = append(out, model.UnreadBoundary{})
		}

		out = append(out, row)
		return out
	}

	lastMessage, ok := previous.(model.Message)
	if !ok {
		out = append(out, row)
		return out
	}

	if !asMessage.Read && lastMessage.Read {
		out = append(out, model.UnreadBoundary{})
	}

	/*y, m, d := asMessage.SentAt.Local().Date()
	yy, mm, dd := lastMessage.SentAt.Local().Date()
	if y == yy && m == mm && d == dd {
		out = append(out, row)
		return out
	}*/

	out = append(out, model.DateBoundary{Date: asMessage.SentAt}, row)
	return out
}

// rowLessThan acts as a list.Comparator, returning whether a sorts before b.
func rowLessThan(a, b list.Element) bool {
	aID := string(a.Serial())
	bID := string(b.Serial())
	aAsInt, _ := strconv.Atoi(aID)
	bAsInt, _ := strconv.Atoi(bID)
	return aAsInt < bAsInt
}
