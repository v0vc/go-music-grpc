/*
Package model provides the domain-specific data models for this list.
*/
package model

import (
	"image"
	"sync"
	"time"

	"github.com/v0vc/go-music-grpc/gio-gui/list"
)

// Message represents a chat message.
type Message struct {
	SerialID                      string
	TypeId, Views, Likes          int32
	Quality                       float32
	Title, Content, AlbumId, Type string
	ParentId                      []string
	SentAt                        time.Time
	Avatar                        image.Image
	Read                          bool
}

func (m Message) GetStringType() string {
	switch m.TypeId {
	case 0:
		return "Album"
	case 1:
		return "Single"
	default:
		return ""
	}
}

// Serial returns the unique identifier for this message.
func (m Message) Serial() list.Serial {
	return list.Serial(m.SerialID)
}

// DateBoundary represents a change in the date during a chat.
type DateBoundary struct {
	Date time.Time
}

// Serial returns the unique identifier of the message.
func (d DateBoundary) Serial() list.Serial {
	return list.NoSerial
}

// UnreadBoundary represents the boundary between the last read message
// in a chat and the next unread message.
type UnreadBoundary struct{}

// Serial returns the unique identifier for the boundary.
func (u UnreadBoundary) Serial() list.Serial {
	return list.NoSerial
}

// Room is a unique conversation context.
// Room can have any number of participants, and any number of messages.
// Any participant of a room should be able to view the room, send messages to
// and receive messages from the other participants.
type Room struct {
	// Image avatar for the room.
	Image image.Image
	// Name of the room.
	Name string
	// Channel id
	Id                   string
	Count                string
	Content              string
	Selected, SelectedPl []string
	IsBase               bool
	Loaded               bool
}

type Messages struct {
	list  []Message
	index map[string]*Message
	once  sync.Once
}

func (us *Messages) Add(u Message) {
	us.once.Do(func() {
		us.index = map[string]*Message{}
	})

	us.list = append(us.list, u)
	us.index[u.Title] = &us.list[len(us.list)-1]
}

func (us *Messages) List() (list []*Message) {
	list = make([]*Message, len(us.list))
	for ii := range us.list {
		list[ii] = &us.list[ii]
	}
	return list
}

func (us *Messages) GetList() (list []Message) {
	return us.list
}

// Rooms structure manages a collection of rooms.
type Rooms struct {
	list  []Room
	index map[string]*Room
	once  sync.Once
}

// Add room to collection.
func (r *Rooms) Add(room Room) {
	r.once.Do(func() {
		r.index = map[string]*Room{}
	})

	r.list = append(r.list, room)
	r.index[room.Name] = &r.list[len(r.list)-1]
}

// List returns an ordered list of room data.
func (r *Rooms) List() (list []*Room) {
	if r == nil {
		return nil
	}
	list = make([]*Room, len(r.list))

	for ii := range r.list {
		list[ii] = &r.list[ii]
	}
	return list
}
