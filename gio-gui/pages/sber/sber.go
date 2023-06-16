package sber

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"regexp"
	"time"

	"gioui.org/gesture"
	"gioui.org/op"
	"gioui.org/op/clip"
	"golang.org/x/image/draw"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/artist"
	client "github.com/v0vc/go-music-grpc/gio-gui/gen"
	"github.com/v0vc/go-music-grpc/gio-gui/icon"
	lay "github.com/v0vc/go-music-grpc/gio-gui/layout"
	page "github.com/v0vc/go-music-grpc/gio-gui/pages"
)

const (
	artistRegexString = `^https://zvuk.com/artist/(\d+)$`
)

type Page struct {
	// artistList        layout.List
	artistInput component.TextField
	// inputAlignment    text.Alignment
	addBtn, addButton widget.Clickable

	users      []*user
	usersList  *widget.List
	userHovers []gesture.Hover
	// userClicks []gesture.Click
	// selectedUser *user

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

type rect struct {
	Color color.NRGBA
	Size  image.Point
	Radii int
}

func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

func (p *Page) Actions() []component.AppBarAction {
	// return []component.AppBarAction{}
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
				// btn := component.SimpleIconButton(bg, fg, &p.addBtn, icon.PlusIcon)
				// btn.Background = bg
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
		Name: "СберЗвук",
		Icon: icon.HeartIcon,
	}
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical
	p.usersList = &widget.List{
		List: layout.List{
			Axis: layout.Vertical,
		},
	}
	p.updateUsers = make(chan []*user)
	p.listChanErr = make(chan error)
	p.ProgressIncrementer = make(chan float32)

	if p.users == nil {
		go fetchArtists(p)
		select {
		case err := <-p.listChanErr:
			fmt.Println(err.Error())
		case p.users = <-p.updateUsers:
			defer close(p.updateUsers)
			p.userHovers = make([]gesture.Hover, len(p.users))
			// p.userClicks = make([]gesture.Click, len(p.users))
		}
	}

	return layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Vertical,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			// gtx.Constraints.Max.Y = 700
			return material.List(th, p.usersList).Layout(gtx, len(p.users), func(gtx layout.Context, i int) layout.Dimensions {
				return userLayout(gtx, i, p, th)
			})
		}),
		layout.Rigid(
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Left:  unit.Dp(10),
					Right: unit.Dp(10),
				}.Layout(gtx, material.ProgressBar(th, p.Progress).Layout)
				// return material.ProgressBar(theme, 50).Layout(gtx)
			},
		),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return lay.DetailRow{
				PrimaryWidth: .8,
			}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					p.artistInput.SingleLine = true
					p.artistInput.MaxLen = 128
					return p.artistInput.Layout(gtx, th, "zvuk.com artist link")
				},
				func(gtx layout.Context) layout.Dimensions {
					if p.addButton.Clicked() {
						/*clipboard.WriteOp{
							Text: "SS",
						}.Add(gtx.Ops)*/
						go incProgress(p)
						artistUrl := p.artistInput.Text()
						if artistUrl != "" {
							artistId := findArtistId(artistUrl)
							if artistId != "" {
								go addArtist(p, artistId)
							}
						}
					}
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &p.addButton, "Add").Layout(gtx)
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
			thumb = client.GetNoAvatarInstance()
		}
		im, _, _ := image.Decode(bytes.NewReader(thumb))
		u := &user{
			name:     artist.GetTitle(),
			avatar:   im,
			avatarOp: paint.ImageOp{},
		}
		// p.users = append(p.users, u)
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
	client, err := client.GetClientInstance()
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
	client, err := client.GetClientInstance()
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

func userLayout(gtx layout.Context, index int, p *Page, th *material.Theme) layout.Dimensions {
	u := p.users[index]
	in := layout.UniformInset(unit.Dp(8))
	dims := in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return centerRowOpts().Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layoutRect(gtx, func(gtx layout.Context) layout.Dimensions {
						dim := gtx.Dp(unit.Dp(48))
						sz := image.Point{X: dim, Y: dim}
						gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(sz))
						return layoutAvatar(gtx, p.users[index])
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return column().Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return baseline().Layout(gtx,
							layout.Rigid(material.Body1(th, u.name).Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								// gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(2)}.Layout(gtx,
										material.Caption(th, "3 hours ago").Layout)
								})
							}),
						)
					}),
				)
			}),
		)
	})

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()

	hover := &p.userHovers[index]
	if hover.Hovered(gtx) {
		rr := gtx.Dp(unit.Dp(4))
		defer clip.RRect{
			Rect: image.Rectangle{
				Max: dims.Size,
			},
			NE: rr,
			SE: rr,
			NW: rr,
			SW: rr,
		}.Push(gtx.Ops).Pop()
		paintRect(gtx, gtx.Constraints.Max, WithAlpha(th.Palette.ContrastBg, 48))
	}
	defer hover.Add(gtx.Ops)

	/*click := &p.userClicks[index]
	for _, e := range click.Events(gtx) {
		if e.Type == gesture.TypeClick {
			p.selectedUser = p.users[index]
		}
	}*/

	/*	click := &p.userClicks[index]
		if click.Pressed() {
			rr := gtx.Dp(unit.Dp(4))
			defer clip.RRect{
				Rect: image.Rectangle{
					Max: dims.Size,
				},
				NE: rr,
				SE: rr,
				NW: rr,
				SW: rr,
			}.Push(gtx.Ops).Pop()
			paintRect(gtx, gtx.Constraints.Max, WithAlpha(th.Palette.ContrastBg, 48))
		}
		click.Add(gtx.Ops)*/

	return dims
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
	defer call.Add(gtx.Ops)
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

func centerRowOpts() layout.Flex {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
}

func column() layout.Flex {
	return layout.Flex{Axis: layout.Vertical}
}

func baseline() layout.Flex {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
}

func WithAlpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: a,
	}
}

func paintRect(gtx layout.Context, size image.Point, fill color.NRGBA) {
	rect{
		Color: fill,
		Size:  size,
	}.Layout(gtx)
}

func (r rect) Layout(gtx layout.Context) layout.Dimensions {
	paint.FillShape(
		gtx.Ops,
		r.Color,
		clip.UniformRRect(
			image.Rectangle{
				Max: r.Size,
			},
			r.Radii,
		).Op(gtx.Ops))
	return layout.Dimensions{Size: r.Size}
}
