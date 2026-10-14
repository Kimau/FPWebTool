package main

import (
	"bytes"
	"image"
	"image/jpeg"
)

// Black border removal for YouTube thumbnails.
//
// Old widescreen uploads have no maxresdefault or hq720, so the best YouTube
// offers is sddefault (640x480) or hqdefault (480x360): a 4:3 frame with the
// 16:9 picture sitting between two black bars. As a share card or a listing
// thumbnail that reads as a broken image (ccWrbGEFgI8, 1jdz-BHmkgE). The reverse
// happens too: a 4:3 webcam video uploaded in a 16:9 frame gets black pillars
// down both sides (hF_97IklJLM), and mCPO3_bZ1T4 manages both at once.
//
// The trap is a video whose subject sits on black - QFdPw2Yo8-0 is a green hand
// over a black void, with 100+ dark rows and columns all round. Borders are only
// cropped when what is left is shaped like video, which that is not.
const (
	borderDarkLuma  = 40 // brightest a border pixel may be; JPEG noise needs the headroom
	borderMinFrac   = 12 // each border at least 1/12 of the side it trims
	borderEdgeSlack = 2  // noisy lines allowed at the very edge (JPEG ringing)

	// What is left after trimming rows must look like widescreen (16:10 to scope),
	// and after trimming columns like 4:3 to 16:9.
	letterboxMinAspect, letterboxMaxAspect = 1.6, 2.45
	pillarboxMinAspect, pillarboxMaxAspect = 1.2, 1.9
)

// borderRun counts dark lines in from one edge, letting the first couple of
// lines be noise. isDark(i) tests line i counted from that edge.
func borderRun(n int, isDark func(int) bool) int {
	s := 0
	for s < borderEdgeSlack && s < n && !isDark(s) {
		s++
	}
	if s >= n || !isDark(s) {
		return 0
	}
	k := s
	for k < n/2 && isDark(k) {
		k++
	}
	return k
}

func luma8(img image.Image, x, y int) uint32 {
	r, g, b, _ := img.At(x, y).RGBA()
	// Rec. 601 luma on the 16-bit channels, scaled back to 8 bits.
	return (299*r + 587*g + 114*b) / 1000 >> 8
}

// blackBorders returns the picture area inside any letterbox and pillarbox
// bars, or img.Bounds() when there is nothing to trim. Each pair of bars is
// trimmed symmetrically to the smaller of the two, because a dark scene can make
// one side look deeper than the bar really is.
func blackBorders(img image.Image) image.Rectangle {
	r := img.Bounds()

	rowDark := func(y int) bool {
		for x := r.Min.X; x < r.Max.X; x += 4 {
			if luma8(img, x, y) >= borderDarkLuma {
				return false
			}
		}
		return true
	}
	if w, h := r.Dx(), r.Dy(); h > 0 && float64(w) < float64(h)*1.5 {
		top := borderRun(h, func(i int) bool { return rowDark(r.Min.Y + i) })
		bottom := borderRun(h, func(i int) bool { return rowDark(r.Max.Y - 1 - i) })
		if bar := min(top, bottom); bar*borderMinFrac >= h {
			if a := float64(w) / float64(h-2*bar); a >= letterboxMinAspect && a <= letterboxMaxAspect {
				r = image.Rect(r.Min.X, r.Min.Y+bar, r.Max.X, r.Max.Y-bar)
			}
		}
	}

	colDark := func(x int) bool {
		for y := r.Min.Y; y < r.Max.Y; y += 4 {
			if luma8(img, x, y) >= borderDarkLuma {
				return false
			}
		}
		return true
	}
	if w, h := r.Dx(), r.Dy(); w > 0 && h > 0 {
		left := borderRun(w, func(i int) bool { return colDark(r.Min.X + i) })
		right := borderRun(w, func(i int) bool { return colDark(r.Max.X - 1 - i) })
		if bar := min(left, right); bar*borderMinFrac >= w {
			if a := float64(w-2*bar) / float64(h); a >= pillarboxMinAspect && a <= pillarboxMaxAspect {
				r = image.Rect(r.Min.X+bar, r.Min.Y, r.Max.X-bar, r.Max.Y)
			}
		}
	}

	return r
}

// cropLetterbox returns the JPEG with its black bars removed, or the input
// unchanged (and false) when there is nothing to crop or it does not decode.
func cropLetterbox(data []byte) ([]byte, bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, false
	}

	crop := blackBorders(img)
	if crop == img.Bounds() {
		return data, false
	}

	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return data, false
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, sub.SubImage(crop), &jpeg.Options{Quality: 90}); err != nil {
		return data, false
	}
	return out.Bytes(), true
}
