package page

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

var (
	Light = Palette{
		Error:         rgb(0xB00020),
		Surface:       rgb(0xFFFFFF),
		Bg:            rgb(0xDCDCDC),
		BgSecondary:   rgb(0xEBEBEB),
		OnError:       rgb(0xFFFFFF),
		OnSurface:     rgb(0x000000),
		OnBg:          rgb(0x000000),
		OnBgSecondary: rgb(0x000000),
	}
	Dark = Palette{
		Error:         rgb(0xB00020),
		Surface:       rgb(0x222222),
		Bg:            rgb(0x000000),
		BgSecondary:   rgb(0x444444),
		OnError:       rgb(0xFFFFFF),
		OnSurface:     rgb(0xFFFFFF),
		OnBg:          rgb(0xEEEEEE),
		OnBgSecondary: rgb(0xFFFFFF),
	}
)

// Theme wraps the material.Theme with useful application-specific
// theme information.
type Theme struct {
	*material.Theme
	// Palette specifies semantic colors.
	Palette Palette
}

// Palette defines non-brand semantic colors.
//
// `On` colors define a color that is appropriate to display atop its
// counterpart.
type Palette struct {
	// Error used to indicate errors.
	Error   color.NRGBA
	OnError color.NRGBA
	// Surface affect surfaces of components, such as cards, sheets and menus.
	Surface   color.NRGBA
	OnSurface color.NRGBA
	// Bg appears behind scrollable content.
	Bg   color.NRGBA
	OnBg color.NRGBA
	// BgSecondary appears behind scrollable content.
	BgSecondary   color.NRGBA
	OnBgSecondary color.NRGBA
}

// UserColorData tracks both a color and its luminance.
type UserColorData struct {
	color.NRGBA
	Luminance float64
}

// NewTheme instantiates a theme using the provided fonts.
func NewTheme(conf *Config) *Theme {
	base := material.NewTheme()
	theme := Theme{
		Theme: base,
	}
	theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	switch conf.Theme {
	case "light":
		theme.UsePalette(Light)
	case "dark":
		theme.UsePalette(Dark)
	}

	return &theme
}

// UsePalette changes to the specified palette.
func (t *Theme) UsePalette(p Palette) {
	t.Palette = p
	t.Bg = t.Palette.Bg
	t.Fg = t.Palette.OnBg
}

func rgb(c uint32) color.NRGBA {
	return argb(0xff000000 | c)
}

func argb(c uint32) color.NRGBA {
	return color.NRGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
}
