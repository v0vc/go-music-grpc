package icon

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var MenuIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationMenu)
	return icon
}()

var PlusIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentAdd)
	return icon
}()

var PasteIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentContentPaste)
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

var AudioTrackIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ImageAudiotrack)
	return icon
}()

var HighQualityIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.AVHighQuality)
	return icon
}()

var NavBack = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationArrowBack)
	return icon
}()

var MusicIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ImageMusicNote)
	return icon
}()

var SyncIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NotificationSync)
	return icon
}()

var CopyIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentContentCopy)
	return icon
}()

var SaveIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentSave)
	return icon
}()

var YoutubeIcon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.AVVideoLibrary)
	return icon
}()
