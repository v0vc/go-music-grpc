package icon

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var MenuIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationMenu)
	return icon
}()

var RestaurantMenuIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.MapsRestaurantMenu)
	return icon
}()

var AccountBalanceIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionAccountBalance)
	return icon
}()

var AccountBoxIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionAccountBox)
	return icon
}()

var CartIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionAddShoppingCart)
	return icon
}()

var HomeIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionHome)
	return icon
}()

var SettingsIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionSettings)
	return icon
}()

var OtherIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionHelp)
	return icon
}()

var HeartIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionFavorite)
	return icon
}()

var PlusIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentAdd)
	return icon
}()

var EditIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentCreate)
	return icon
}()

var VisibilityIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionVisibility)
	return icon
}()

var DeleteIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionDelete)
	return icon
}()

var DownloadIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.FileFileDownload)
	return icon
}()

var NavBack = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationArrowBack)
	return icon
}()

var Send = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentSend)
	return icon
}()

var Search = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionSearch)
	return icon
}()

var MusicIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ImageMusicNote)
	return icon
}()
