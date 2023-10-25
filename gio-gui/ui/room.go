package ui

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"

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
	RowTracker *RowTracker
	// ListState dynamically manages list state.
	// This lets us surf across a vast ocean of infinite messages, only ever
	// rendering what is actually viewable.
	// The widget.List consumes this during layout.
	ListState *list.Manager
	// List implements the raw scrolling, adding scrollbars and responding
	// to mousewheel / touch fling gestures.
	List widget.List
	// Editor contains the edit buffer for composing messages.
	Editor widget.Editor
	sync.Mutex
	// searchCurSeq    int
	SearchResponses chan []list.Serial
}

// SetComposing sets the composing status for a user in this room.
// Note: doesn't actually verify the user pertains to this room.
/*func (r *Room) SetComposing(user string, isComposing bool) {
	r.Room.SetComposing(user, isComposing)
}*/

// Send attempts to send arbitrary content as a message from the specified user.
/*func (r *Room) Send(user, content string) {
	row := r.RowTracker.Send(user, content)
	r.Lock()
	r.Room.Latest = &row
	r.Unlock()
	go r.ListState.Modify([]list.Element{row}, nil, nil)
}*/

// SendLocal attempts to send the contents of the edit buffer as a
// to the model.
// All the work of this method is dispatched in a new goroutine
// so that it can safely be called from layout code without blocking.
/*func (r *Room) SendLocal(msg string) {
	go func() {
		r.Lock()
		row := r.RowTracker.Send("TEST", msg)
		r.Room.Latest = &row
		r.Unlock()
		r.ListState.Modify([]list.Element{row}, nil, nil)
	}()
}*/

func AddAlbums(rooms *Rooms, albs []model.Message) {
	artMap := make(map[string][]model.Message)
	for _, alb := range albs {
		artMap["-1"] = append(artMap["-1"], alb)
		for _, artId := range alb.ParentId {
			/*_, ok := artMap[artId]
			if !ok {
				artMap[artId] = append(artMap[artId], alb)
			}*/
			artMap[artId] = append(artMap[artId], alb)
		}
	}
	go func() {
		for artId, albums := range artMap {
			ch := rooms.GetChannelById(artId)
			if ch != nil {
				ch.Lock()
				if ch.IsBase {
					curCount, _ := strconv.Atoi(ch.Room.Count)
					ch.Room.Count = strconv.Itoa(curCount + len(albs))
				} else {
					ch.Room.Count = strconv.Itoa(len(albums))
				}

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
	}()
}

func (r *Room) RunSearch(searchText string) {
	var resp []list.Serial
	go func() {
		defer func() {
			r.SearchResponses <- resp
		}()
		input := strings.ToLower(searchText)
		if input == "" {
			return
		}
		resp = make([]list.Serial, 0, len(r.RowTracker.Rows)/3)
		for i := range r.RowTracker.Rows {
			e := r.RowTracker.Rows[i].(model.Message)
			if strings.Contains(e.Title, input) || strings.Contains(strings.ToLower(e.Title), input) {
				// log.Println(e.SerialID)
				resp = append(resp, e.Serial())
			} // else {
			//resp.indices = append(resp.indices, e.Serial())
			// r.RowTracker.Delete(e.Serial())
			// r.ListState.Modify(nil, nil, []list.Serial{e.Serial()})
			//}

		}
	}()

	pending := <-r.SearchResponses
	for _, ind := range pending {
		// r.RowTracker.Delete(ind)
		fmt.Println(ind)
	}
	// go r.ListState.Modify(nil, nil, pending)
}

// NewRow generates a new row in the Room's RowTracker and inserts it
// into the list manager for the room.
/*func (r *Room) NewRow() {
	row := r.RowTracker.NewRow()
	go r.ListState.Modify([]list.Element{row}, nil, nil)
}*/

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

// Latest returns a copy of the latest message for the room.
/*func (r *Room) Latest() model.Message {
	r.Lock()
	defer r.Unlock()
	if r.Room.Latest == nil {
		return model.Message{}
	}
	return *r.Room.Latest
}*/

func (r *Room) DownloadAlbum(siteId uint32, albumId []string, trackQuality string) {
	r.Lock()
	defer r.Unlock()
	go r.RowTracker.Generator.DownloadAlbum(siteId, albumId, trackQuality)
}

func (r *Room) DownloadArtist(siteId uint32, artistId string, trackQuality string) {
	r.Lock()
	defer r.Unlock()
	go r.RowTracker.Generator.DownloadArtist(siteId, artistId, trackQuality)
}

func (r *Room) SyncArtist(rooms *Rooms, siteId uint32) {
	r.Lock()
	defer r.Unlock()
	albs := make(chan []model.Message, 1)
	go r.RowTracker.Generator.SyncArtist(siteId, r.Id, albs)
	res := <-albs
	AddAlbums(rooms, res)
}

// Select the room at the given index.
// Index is bounded by [0, len(rooms)).
/*func (r *Rooms) Select(index int) {
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
}*/

func (r *Rooms) SelectAndFill(siteId uint32, index int, albs []model.Message, invalidator func(), presentChatRow func(data list.Element, state interface{}) layout.Widget) {
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

	if r.List[r.active].RowTracker.Rows == nil && !r.List[r.active].Loaded {
		channel := r.List[r.active]
		// var albs []model.Message
		if albs == nil {
			if channel.Room.IsBase {
				albs = channel.RowTracker.Generator.GetNewAlbums(siteId)
				count := len(albs)
				if count > 0 {
					channel.Count = strconv.Itoa(count)
				}
			} else {
				albs = channel.RowTracker.Generator.GetArtistAlbums(siteId, r.List[r.active].Room.Id)
			}
		}
		for _, alb := range albs {
			channel.RowTracker.Add(alb)
		}
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
				Presenter: presentChatRow,
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
		channel.Room.Loaded = true
	}
}

// Changed if the active room has changed since last call.
func (r *Rooms) Changed() bool {
	r.Lock()
	defer r.Unlock()
	defer func() { r.changed = false }()
	return r.changed
}

// Index returns a pointer to a Room at the given index.
// Index is bounded by [0, len(rooms)).
func (r *Rooms) Index(index int) *Room {
	r.Lock()
	defer r.Unlock()
	if index < 0 {
		index = 0
	}
	if index > len(r.List) {
		index = len(r.List) - 1
	}
	return r.List[index]
}

func (r *Rooms) GetChannelById(artistId string) *Room {
	r.Lock()
	defer r.Unlock()
	for _, alb := range r.List {
		if alb.Id == artistId {
			return alb
		}
	}
	return nil
}

func (r *Rooms) Random() *Room {
	r.Lock()
	defer r.Unlock()
	return r.List[rand.Intn(len(r.List)-1)]
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
	y, m, d := asMessage.SentAt.Local().Date()
	yy, mm, dd := lastMessage.SentAt.Local().Date()
	if y == yy && m == mm && d == dd {
		out = append(out, row)
		return out
	}
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
