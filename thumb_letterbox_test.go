package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
)

// frame draws a w x h JPEG: black everywhere, with a mid-grey band from row
// top to row bottom (exclusive), as a letterboxed or black-backed thumb would be.
func frame(t *testing.T, w, h, top, bottom int) []byte {
	t.Helper()
	return box(t, w, h, image.Rect(0, top, w, bottom), false)
}

// box draws a w x h JPEG: black, with a mid-grey picture in pic. noisyEdge adds
// a bright last column, the JPEG ringing real YouTube thumbs have.
func box(t *testing.T, w, h int, pic image.Rectangle, noisyEdge bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{0, 0, 0, 255}
			if image.Pt(x, y).In(pic) || (noisyEdge && x == w-1) {
				c = color.RGBA{128, 140, 150, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func size(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestCropLetterbox(t *testing.T) {
	cases := []struct {
		name         string
		data         []byte
		crop         bool
		wantW, wantH int
	}{
		{"16:9 video in a 4:3 hqdefault", frame(t, 480, 360, 45, 315), true, 480, 270},
		{"16:9 video in a 4:3 sddefault", frame(t, 640, 480, 60, 420), true, 640, 360},
		{"uneven bars are trimmed to the smaller", frame(t, 480, 360, 45, 300), true, 480, 270},
		{"4:3 video with no bars", frame(t, 480, 360, 0, 360), false, 480, 360},
		{"subject on black is not a letterbox", frame(t, 480, 360, 115, 270), false, 480, 360},
		{"thin dark edge is not a letterbox", frame(t, 480, 360, 10, 350), false, 480, 360},
		{"widescreen frame with only top and bottom dark is left alone", frame(t, 1280, 720, 80, 640), false, 1280, 720},
		{"4:3 webcam pillarboxed in a 16:9 frame", box(t, 1280, 720, image.Rect(160, 0, 1120, 720), false), true, 960, 720},
		{"4:3 picture in a 16:9 band in a 4:3 frame", box(t, 640, 480, image.Rect(80, 60, 560, 420), false), true, 480, 360},
		{"bright edge column is treated as noise", box(t, 640, 480, image.Rect(80, 60, 560, 420), true), true, 480, 360},
		{"subject on black in the middle is not a pillarbox", box(t, 480, 360, image.Rect(170, 115, 270, 270), false), false, 480, 360},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, did := cropLetterbox(tc.data)
			if did != tc.crop {
				t.Fatalf("cropped = %v, want %v", did, tc.crop)
			}
			if w, h := size(t, out); w != tc.wantW || h != tc.wantH {
				t.Errorf("result is %dx%d, want %dx%d", w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

// The real trap case, committed in the cache: a green hand and a cube on black,
// with 100+ dark rows above and below. It must come through untouched.
func TestCropLetterboxLeavesBlackBackedVideoAlone(t *testing.T) {
	t.Chdir("..")
	data, err := os.ReadFile("images/yt/QFdPw2Yo8-0.jpg")
	if err != nil {
		t.Skip("cached thumb not present:", err)
	}
	if _, did := cropLetterbox(data); did {
		t.Error("cropped the hand animation thumbnail, which has no letterbox")
	}
}

func TestYouTubeIDs(t *testing.T) {
	body := `<iframe src="https://www.youtube.com/embed/rIP7djqpeK0"></iframe>
		<a href="https://youtu.be/S5Np-clhWNA">again</a>
		<iframe src="https://www.youtube.com/embed/rIP7djqpeK0"></iframe>
		<a href="https://www.youtube.com/channel/UCbpbGczUYcD1MOxgj8urocg">channel</a>`

	got := youTubeIDs(body)
	want := []string{"rIP7djqpeK0", "S5Np-clhWNA"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("youTubeIDs = %v, want %v", got, want)
	}
}

func TestYouTubeThumbLadderEndsOnMqdefault(t *testing.T) {
	// mqdefault is the only size every video has at 320x180 or better; it has to
	// stay last so it never beats a real HD or cropped SD frame.
	if last := ytThumbNames[len(ytThumbNames)-1]; last != "mqdefault.jpg" {
		t.Errorf("last rung is %q, want mqdefault.jpg", last)
	}
	if ytThumbNames[0] != "maxresdefault.jpg" {
		t.Errorf("first rung is %q, want maxresdefault.jpg", ytThumbNames[0])
	}
}
