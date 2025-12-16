package cli

import (
	"fmt"
	"strings"
)

const bannerDefaultWidth = 60

// PrintBanner renders a box-drawing banner around a title using the default width.
func PrintBanner(title string) {
	PrintBannerWidth(title, bannerDefaultWidth)
}

// PrintBannerWidth renders a box-drawing banner around a title using the provided width.
// If the title is wider than the inner width, the banner grows to fit it.
func PrintBannerWidth(title string, width int) {
	if width < 10 {
		width = bannerDefaultWidth
	}

	inner := width - 2
	if len(title)+2 > inner {
		inner = len(title) + 2
	}

	topBottom := strings.Repeat("═", inner)
	middle := padCenter(title, inner)

	fmt.Printf("╔%s╗\n", topBottom)
	fmt.Printf("║%s║\n", middle)
	fmt.Printf("╚%s╝\n", topBottom)
}

func padCenter(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	padTotal := width - len(text)
	left := padTotal / 2
	right := padTotal - left
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}
