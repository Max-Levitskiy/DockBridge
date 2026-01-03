package gui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// resources.go contains embedded icons and static assets for the GUI.
// Icons and images should be embedded here using embed directives.

//go:embed assets/icon.png
var iconData []byte

// ResourceIcon is the application icon resource.
var ResourceIcon = &fyne.StaticResource{
	StaticName:    "icon.png",
	StaticContent: iconData,
}

//go:embed assets/logo.png
var logoData []byte

// ResourceLogo is the application logo resource.
var ResourceLogo = &fyne.StaticResource{
	StaticName:    "logo.png",
	StaticContent: logoData,
}
