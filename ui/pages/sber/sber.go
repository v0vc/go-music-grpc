package sber

import (
	"bytes"
	"context"
	"fmt"
	"gioui.org/io/clipboard"
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
)

type Page struct {
	usersList         layout.List
	numberInput       component.TextField
	inputAlignment    text.Alignment
	addBtn, addButton widget.Clickable

	users       []*user
	updateUsers chan []*user
	listChanErr chan error

	widget.List
	*page.Router
}

type user struct {
	name     string
	avatar   image.Image
	avatarOp paint.ImageOp
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
				p.usersList.Axis = layout.Vertical
				return p.usersList.Layout(gtx, len(p.users), func(gtx layout.Context, index int) layout.Dimensions {
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
				}.Layout(gtx, material.ProgressBar(theme, 50).Layout)
				//return material.ProgressBar(theme, 50).Layout(gtx)
			},
		),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return alo.DetailRow{
				PrimaryWidth: .8,
			}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					p.numberInput.SingleLine = true
					p.numberInput.MaxLen = 128
					return p.numberInput.Layout(gtx, theme, "zvuk.com artist link")
				},
				func(gtx layout.Context) layout.Dimensions {
					if p.addButton.Clicked() {
						clipboard.WriteOp{
							Text: "SS",
						}.Add(gtx.Ops)
					}
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(theme, &p.addButton, "Add").Layout(gtx)
					})
					//return material.Button(theme, &p.addButton, "Add").Layout(gtx)
				})
		}),
	)

	/*	return material.List(theme, &p.List).Layout(gtx, len(p.users), func(gtx layout.Context, i int) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutRect(gtx, func(gtx layout.Context) layout.Dimensions {
							dim := gtx.Dp(unit.Dp(48))
							sz := image.Point{X: dim, Y: dim}
							gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(sz))
							return layoutAvatar(gtx, p.users[i])
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
								layout.Rigid(material.Body1(theme, p.users[i].name).Layout),
							)
						}),
					)
				}),
			)
		})
	})*/
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

	var users []*user
	for _, artist := range res.Artists {
		im, _, _ := image.Decode(bytes.NewReader(artist.GetThumbnail()))
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
