package sber

import (
	"bytes"
	"context"
	"fmt"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/v0vc/go-music-grpc/artist"
	"github.com/v0vc/go-music-grpc/ui/icon"
	page "github.com/v0vc/go-music-grpc/ui/pages"
	"golang.org/x/image/draw"
	"image"
)

type Page struct {
	/*redButton, greenButton, blueButton       widget.Clickable
	balanceButton, accountButton, cartButton widget.Clickable
	leftFillColor                            color.NRGBA
	leftContextArea                          component.ContextArea
	leftMenu, rightMenu                      component.MenuState
	menuInit                                 bool*/

	usersList layout.List
	users     []*user
	//updateUsers chan []*user
	/*menuDemoList layout.List*/
	//menuDemoListStates []component.ContextArea
	widget.List
	*page.Router
}

type user struct {
	name     string
	avatar   image.Image
	avatarOp paint.ImageOp
}

// New constructs a Page with the provided router.
func New(router *page.Router) *Page {
	return &Page{
		Router: router,
	}
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "SberZvuk",
		Icon: icon.HeartIcon,
	}
}

func fetchArtists(p *Page) {
	res, err := page.GetClientInstance().ListArtist(context.Background(), &artist.ListArtistRequest{SiteId: 1})
	if err != nil {
		fmt.Printf("error while reading: %v \n", err)
	}

	for _, artist := range res.Artists {
		im, _, _ := image.Decode(bytes.NewReader(artist.GetThumbnail()))
		u := &user{
			name:     artist.GetTitle(),
			avatar:   im,
			avatarOp: paint.ImageOp{},
		}
		p.users = append(p.users, u)
	}
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical

	//p.updateUsers = make(chan []*user, 1)

	if p.users == nil {
		fetchArtists(p)
	}

	//p.users = <-p.updateUsers

	return material.List(th, &p.List).Layout(gtx, len(p.users), func(gtx layout.Context, i int) layout.Dimensions {
		return p.layoutArtist(gtx, i, th)
	})

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

func (p *Page) layoutArtist(gtx layout.Context, index int, theme *material.Theme) layout.Dimensions {
	user := p.users[index]
	in := layout.UniformInset(unit.Dp(8))
	dims := in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				in := layout.Inset{Right: unit.Dp(8)}
				return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layoutRect(gtx, func(gtx layout.Context) layout.Dimensions {
						dim := gtx.Dp(unit.Dp(48))
						sz := image.Point{X: dim, Y: dim}
						gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(sz))
						return layoutAvatar(gtx, user)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
							layout.Rigid(material.Body1(theme, user.name).Layout),
						)
					}),
				)
			}),
		)
	})
	return dims
}
