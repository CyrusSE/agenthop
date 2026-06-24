package tui

import _ "embed"

//go:embed banner.txt
var bannerASCII string

func renderBanner() string {
	return accentStyle.Render(bannerASCII)
}
