package app

import (
	"image/color"
	"testing"
)

func TestViewUsesFixedTerminalColors(t *testing.T) {
	t.Parallel()

	view := New(resizeTestSearcher{}, resizeTestTracker{}, nil, nil, nil).View()

	if got, want := color.RGBAModel.Convert(view.BackgroundColor), (color.RGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff}); got != want {
		t.Errorf("background color = %v, want %v", got, want)
	}
	if got, want := color.RGBAModel.Convert(view.ForegroundColor), (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}); got != want {
		t.Errorf("foreground color = %v, want %v", got, want)
	}
}
