package sber

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/artist"
	alo "github.com/v0vc/go-music-grpc/ui/applayout"
	"github.com/v0vc/go-music-grpc/ui/icon"
	page "github.com/v0vc/go-music-grpc/ui/pages"
	"golang.org/x/image/draw"
	"image"
	"image/color"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	artistRegexString = `^https://zvuk.com/artist/(\d+)$`
)

type Page struct {
	artistList        layout.List
	artistInput       component.TextField
	inputAlignment    text.Alignment
	addBtn, addButton widget.Clickable

	users               []*user
	updateUsers         chan []*user
	listChanErr         chan error
	Progress            float32
	ProgressIncrementer chan float32

	widget.List
	*page.Router
}

type user struct {
	name     string
	avatar   image.Image
	avatarOp paint.ImageOp
}

var singleInstanceNoAvatar []byte

var lock = &sync.Mutex{}

func GetNoAvatarInstance() []byte {
	if singleInstanceNoAvatar == nil {
		lock.Lock()
		defer lock.Unlock()

		emptyAva := "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"
		reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(emptyAva))
		buff := bytes.Buffer{}
		buff.ReadFrom(reader)
		singleInstanceNoAvatar = buff.Bytes()
	}
	return singleInstanceNoAvatar
}

func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

func (p *Page) Actions() []component.AppBarAction {
	//return []component.AppBarAction{}
	return []component.AppBarAction{
		{
			OverflowAction: component.OverflowAction{
				Name: "Add",
				Tag:  &p.addBtn,
			},
			Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
				if p.addBtn.Clicked() {
					p.Router.AppBar.SetContextualActions(
						[]component.AppBarAction{},
						[]component.OverflowAction{},
					)
					p.Router.AppBar.ToggleContextual(gtx.Now, "Add artist")
				}
				//btn := component.SimpleIconButton(bg, fg, &p.addBtn, icon.PlusIcon)
				//btn.Background = bg
				return component.SimpleIconButton(bg, fg, &p.addBtn, icon.PlusIcon).Layout(gtx)
			},
		},
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
	/*	return []component.OverflowAction{
		{
			Name: "Add",
			Tag:  &p.addState,
		},
		{
			Name: "Delete",
			Tag:  &p.removeState,
		},
	}*/
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "SberZvuk",
		Icon: icon.HeartIcon,
	}
}

func (p *Page) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical

	p.updateUsers = make(chan []*user)
	p.listChanErr = make(chan error, 0)
	p.ProgressIncrementer = make(chan float32)

	if p.users == nil {
		go fetchArtists(p)
		select {
		case err := <-p.listChanErr:
			fmt.Println(err.Error())
		case p.users = <-p.updateUsers:
			defer close(p.updateUsers)
		}
	}

	return layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Vertical,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			//gtx.Constraints.Max.Y = 700
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				p.artistList.Axis = layout.Vertical
				return p.artistList.Layout(gtx, len(p.users), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layoutRect(gtx, func(gtx layout.Context) layout.Dimensions {
									dim := gtx.Dp(unit.Dp(48))
									sz := image.Point{X: dim, Y: dim}
									gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(sz))
									return layoutAvatar(gtx, p.users[index])
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
										layout.Rigid(material.Body1(theme, p.users[index].name).Layout),
									)
								}),
							)
						}),
					)
				})
			})
		}),
		layout.Rigid(
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Left:  unit.Dp(10),
					Right: unit.Dp(10),
				}.Layout(gtx, material.ProgressBar(theme, p.Progress).Layout)
				//return material.ProgressBar(theme, 50).Layout(gtx)
			},
		),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return alo.DetailRow{
				PrimaryWidth: .8,
			}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					p.artistInput.SingleLine = true
					p.artistInput.MaxLen = 128
					return p.artistInput.Layout(gtx, theme, "zvuk.com artist link")
				},
				func(gtx layout.Context) layout.Dimensions {
					if p.addButton.Clicked() {
						/*clipboard.WriteOp{
							Text: "SS",
						}.Add(gtx.Ops)*/
						go incProgress(p)
						/*artistUrl := p.artistInput.Text()
						if artistUrl != "" {
							artistId := findArtistId(artistUrl)
							if artistId != "" {
								go addArtist(p, artistId)
							}
						}*/
					}
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(theme, &p.addButton, "Add").Layout(gtx)
					})
				})
		}),
	)
}

func findArtistId(url string) string {
	matchArtist := regexp.MustCompile(artistRegexString).FindStringSubmatch(url)
	if matchArtist == nil {
		return ""
	}
	return matchArtist[1]
}

func addToList(p *Page, artists []*artist.Artist) {
	var users []*user
	for _, artist := range artists {
		/*		if artist.UserAdded == false {
				continue
			}*/
		thumb := artist.GetThumbnail()
		if thumb == nil {
			thumb = GetNoAvatarInstance()
		}
		im, _, _ := image.Decode(bytes.NewReader(thumb))
		u := &user{
			name:     artist.GetTitle(),
			avatar:   im,
			avatarOp: paint.ImageOp{},
		}
		//p.users = append(p.users, u)
		users = append(users, u)
	}
	p.updateUsers <- users
}

func incProgress(p *Page) {
	for {
		time.Sleep(time.Second / 25)
		p.ProgressIncrementer <- 0.005
	}
}

func addArtist(p *Page, artistId string) {
	client, err := page.GetClientInstance()
	if err != nil {
		p.listChanErr <- err
		return
	}

	res, err := client.SyncArtist(context.Background(), &artist.SyncArtistRequest{
		SiteId:   1,
		ArtistId: artistId,
	})

	if err != nil {
		p.listChanErr <- err
		p.Progress = 0
		return
	}
	p.Progress = 1
	p.artistInput.SetText("")
	addToList(p, res.Artists)
}

func fetchArtists(p *Page) {
	client, err := page.GetClientInstance()
	if err != nil {
		p.listChanErr <- err
		return
	}

	res, err := client.ListArtist(context.Background(), &artist.ListArtistRequest{SiteId: 1})
	if err != nil {
		p.listChanErr <- err
		return
	}

	addToList(p, res.Artists)
}

func layoutRect(gtx layout.Context, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	max := dims.Size.X
	if dy := dims.Size.Y; dy > max {
		max = dy
	}
	rr := max / 2
	defer clip.RRect{
		Rect: image.Rectangle{Max: image.Point{X: max, Y: max}},
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}.Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

func layoutAvatar(gtx layout.Context, u *user) layout.Dimensions {
	sz := gtx.Constraints.Min.X
	if u.avatarOp.Size().X != sz {
		img := image.NewRGBA(image.Rectangle{Max: image.Point{X: sz, Y: sz}})
		draw.ApproxBiLinear.Scale(img, img.Bounds(), u.avatar, u.avatar.Bounds(), draw.Src, nil)
		u.avatarOp = paint.NewImageOp(img)
	}
	img := widget.Image{Src: u.avatarOp}
	img.Scale = float32(sz) / float32(gtx.Dp(unit.Dp(float32(sz))))
	return img.Layout(gtx)
}
