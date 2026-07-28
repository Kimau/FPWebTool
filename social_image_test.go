package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Every positive case here is a string that actually appears in blogdata/ or
// microdata/. The negatives are the three YouTube URL shapes the corpus contains
// that are not videos - a channel link in a socials footer, a user page, and a
// search - any of which becoming a post's header image would be a visible bug.
func TestFirstYouTubeID(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"iframe embed",
			`<iframe width="560" height="315" src="https://www.youtube.com/embed/UHlleCi7j_c" allowfullscreen></iframe>`,
			"UHlleCi7j_c",
		},
		{
			"iframe embed with si tracking param",
			`<iframe src="https://www.youtube.com/embed/Jd3-eiid-Uw?si=2ACMqjZV1jyABmRG"></iframe>`,
			"Jd3-eiid-Uw",
		},
		{
			// blogdata/post/2008/progress-7-file-savings-and-loading.html
			"malformed double slash embed",
			`<iframe src="http://www.youtube.com/embed//S5Np-clhWNA"></iframe>`,
			"S5Np-clhWNA",
		},
		{
			// blogdata/post/2007/the-best-pick-me-up-ever.html
			"2006-era flash embed",
			`<embed src="http://www.youtube.com/v/QGO2hVA3P58" type="application/x-shockwave-flash">`,
			"QGO2hVA3P58",
		},
		{
			// microdata/2025/split_render.md - markdown link, trailing paren
			"youtu.be short link",
			`<a href="https://youtu.be/ZhcpvWUyfp0">VR Standards Dilemma</a>)`,
			"ZhcpvWUyfp0",
		},
		{
			// microdata/2026/contingency.md
			"watch link",
			`<a href="https://www.youtube.com/watch?v=WYV7lKL9QUU">Audio Recording</a>`,
			"WYV7lKL9QUU",
		},
		{
			"watch link with leading id starting in underscore",
			`<a href="https://www.youtube.com/watch?v=_tcoz48zrtc">talk</a>)`,
			"_tcoz48zrtc",
		},
		{
			"watch link with params before v",
			`http://www.youtube.com/watch?feature=player_embedded&v=2k8fHR9jKVM`,
			"2k8fHR9jKVM",
		},
		{
			"watch link with html-escaped ampersand",
			`http://www.youtube.com/watch?feature=x&amp;v=2k8fHR9jKVM`,
			"2k8fHR9jKVM",
		},
		{
			"watch link with trailing playlist",
			`https://www.youtube.com/watch?v=0GpSLTq-h0E&list=PLbQabcd`,
			"0GpSLTq-h0E",
		},
		{
			"protocol relative",
			`<iframe src="//www.youtube.com/embed/D0y5K120Vvw"></iframe>`,
			"D0y5K120Vvw",
		},
		{
			"nocookie host",
			`<iframe src="https://www.youtube-nocookie.com/embed/D0y5K120Vvw"></iframe>`,
			"D0y5K120Vvw",
		},
		{
			"id containing a hyphen",
			`<iframe src="https://www.youtube.com/embed/QFdPw2Yo8-0"></iframe>`,
			"QFdPw2Yo8-0",
		},

		// Negatives.
		{
			// microdata/2025/personal_journey.md:69 - a socials footer link
			"channel link is not a video",
			`<a href="https://www.youtube.com/channel/UCbpbGczUYcD1MOxgj8urocg">YouTube</a>`,
			"",
		},
		{
			// blogdata/post/2011/lets-play-design-goldmine-on-youtube.html:1
			"user page is not a video",
			`<a href="http://www.youtube.com/user/Kikoskia">Kikoskia</a>`,
			"",
		},
		{
			// same post, line 2
			"search results page is not a video",
			`<a href="http://www.youtube.com/results?search_query=Lets+Play&aq=f">search</a>`,
			"",
		},
		{
			"bare mention",
			`I posted it to YouTube last week.`,
			"",
		},
		{
			"body with no video at all",
			`<p>Some text and an <img src="/images/blog/ball.gif" /> image.</p>`,
			"",
		},

		// Ordering: the socials footer sits at the bottom of most micro posts, so
		// a real embed above it must win rather than the footer being reached first.
		{
			"real embed wins over a later channel link",
			`<iframe src="https://www.youtube.com/embed/v5jx1HCfBGY"></iframe>
			 <a href="https://www.youtube.com/channel/UCbpbGczUYcD1MOxgj8urocg">YouTube</a>`,
			"v5jx1HCfBGY",
		},
		{
			"channel link before a real embed is skipped, not matched",
			`<a href="https://www.youtube.com/channel/UCbpbGczUYcD1MOxgj8urocg">YouTube</a>
			 <iframe src="https://www.youtube.com/embed/v5jx1HCfBGY"></iframe>`,
			"v5jx1HCfBGY",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstYouTubeID(tc.body); got != tc.want {
				t.Errorf("firstYouTubeID() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A 12th id-legal character means the token is not an 11-character video id. RE2
// has no lookahead, so this is enforced by a trailing optional capture group;
// if that guard regresses, ids get silently truncated to their first 11 chars.
func TestFirstYouTubeIDRejectsOverlongTokens(t *testing.T) {
	body := `<a href="https://www.youtube.com/watch?v=UHlleCi7j_cEXTRA">not a video</a>`
	if got := firstYouTubeID(body); got != "" {
		t.Errorf("firstYouTubeID() = %q on an overlong token, want %q", got, "")
	}
}

// Live check against i.ytimg.com, skipped under -short. Worth keeping: the size
// ladder and the placeholder rejection are the parts most likely to rot silently,
// and a broken fetch shows up as posts quietly falling back to the site default
// rather than as an error.
func TestCacheYouTubeThumbLive(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	t.Chdir("..")

	// microdata/2022/secondmoon.md - a post whose entire content is this video,
	// and which shipped the site logo as its card before this change.
	const id = "D0y5K120Vvw"

	got := cacheYouTubeThumb(id)
	if got != ytThumbWebDir+id+".jpg" {
		t.Fatalf("cacheYouTubeThumb(%q) = %q, want %q", id, got, ytThumbWebDir+id+".jpg")
	}

	if _, err := os.Stat(filepath.Join(ytThumbDiskDir, id+".jpg")); err != nil {
		t.Fatalf("thumbnail was not written to the cache: %v", err)
	}

	// The copy-ordering hazard: images/ is copied to the web root before
	// generation runs, so a thumb fetched during load must be mirrored across or
	// it 404s in production and self-heals on the next build.
	if _, err := os.Stat(filepath.Join(publicHtmlRoot, "images", "yt", id+".jpg")); err != nil {
		t.Errorf("thumbnail was not mirrored into the web root: %v", err)
	}

	img := validateImage(got, "alt", "youtube")
	if !img.OK() {
		t.Fatal("cached thumbnail did not validate")
	}
	if !img.Landscape() {
		t.Errorf("cached thumbnail is %dx%d, which will not earn a large card", img.Width, img.Height)
	}
	t.Logf("cached %s at %dx%d", id, img.Width, img.Height)
}

func TestAbsAsset(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/images/blog/ball.gif", siteBaseURL + "/images/blog/ball.gif"},
		// images/micro/2023 con duck.jpg is a real file whose raw space used to
		// ship into og:image, twitter:image and the RSS enclosure url.
		{"/images/micro/2023 con duck.jpg", siteBaseURL + "/images/micro/2023%20con%20duck.jpg"},
		{"images/no/leading/slash.png", siteBaseURL + "/images/no/leading/slash.png"},
		{"https://i.ytimg.com/vi/abc/hq.jpg", "https://i.ytimg.com/vi/abc/hq.jpg"},
		{"", ""},
	}

	for _, tc := range cases {
		if got := AbsAsset(tc.in); got != tc.want {
			t.Errorf("AbsAsset(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// AbsAsset must not double-encode: ResolvedImage.Path is stored decoded, because
// cleanImagePath unescapes on the way in.
func TestAbsAssetDoesNotDoubleEncode(t *testing.T) {
	decoded := cleanImagePath("/images/micro/2023%20con%20duck.jpg")
	if got, want := AbsAsset(decoded), siteBaseURL+"/images/micro/2023%20con%20duck.jpg"; got != want {
		t.Errorf("AbsAsset(cleanImagePath(...)) = %q, want %q", got, want)
	}
}

func TestResolvedImageLandscape(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want bool
	}{
		{"TitleBoard default", 1066, 600, true},
		{"youtube maxres", 1280, 720, true},
		{"youtube hqdefault", 480, 360, true},
		{"tiny logo", 120, 120, false},
		{"smlImage thumbnail", 150, 150, false},
		{"tall portrait", 600, 1066, false},
		{"wide but too short", 400, 100, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ri := ResolvedImage{Path: "/x.png", Width: tc.w, Height: tc.h}
			if got := ri.Landscape(); got != tc.want {
				t.Errorf("Landscape() = %v for %dx%d, want %v", got, tc.w, tc.h, tc.want)
			}
		})
	}
}

func TestResolvedImageZeroValueIsNotOK(t *testing.T) {
	var ri ResolvedImage
	if ri.OK() {
		t.Error("zero ResolvedImage reported OK; the hero and thumbnail guards depend on it not being")
	}
}

// canonicalCase is what stops /images/Blog/... (which Windows opens and S3 404s)
// reaching production. Runs from the repo root, where the build itself runs.
func TestCanonicalCase(t *testing.T) {
	t.Chdir("..")

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			// blogdata/post/2014/game-designers-are-all-on-steroids.html:9
			"capital B legacy blog path",
			"/images/Blog/capture1-150x150.png",
			"/images/blog/capture1-150x150.png",
		},
		{"already correct", "/images/blog/ball.gif", "/images/blog/ball.gif"},
		{"default share image", siteDefaultImagePath, siteDefaultImagePath},
		{"unknown path passes through", "/images/nope/missing.png", "/images/nope/missing.png"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalCase(tc.in)
			if got != tc.want {
				t.Errorf("canonicalCase(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if again := canonicalCase(got); again != got {
				t.Errorf("canonicalCase not idempotent: %q -> %q", got, again)
			}
		})
	}
}

// The ladder's contract: local images beat YouTube, dead paths fall through
// rather than shipping, and "nothing found" is the zero value rather than the
// site default.
func TestResolvePostImage(t *testing.T) {
	t.Chdir("..")

	const deadTumblr = `<img src="http://kimau.tumblr.com/photo/1280/484012510/1/tumblr_l03eqrBjj71qbssk5" />`
	const localImg = `<img src="/images/blog/ball.gif" />`

	t.Run("banner wins", func(t *testing.T) {
		got := resolvePostImage("/images/blog/2015-ld34.gif", "", localImg, "alt")
		if got.Source != "banner" || got.Path != "/images/blog/2015-ld34.gif" {
			t.Errorf("got %+v, want the banner", got)
		}
	})

	t.Run("small image used when there is no banner", func(t *testing.T) {
		got := resolvePostImage("", "/images/blog/2015-ld34-tiny.gif", "", "alt")
		if got.Source != "small" {
			t.Errorf("got source %q, want %q", got.Source, "small")
		}
	})

	t.Run("dead external body image is skipped", func(t *testing.T) {
		if got := resolvePostImage("", "", deadTumblr, "alt"); got.OK() {
			t.Errorf("got %+v, want nothing - that path has been dead for a decade", got)
		}
	})

	t.Run("dead image skipped in favour of a later local one", func(t *testing.T) {
		got := resolvePostImage("", "", deadTumblr+localImg, "alt")
		if got.Source != "body" || got.Path != "/images/blog/ball.gif" {
			t.Errorf("got %+v, want the local body image", got)
		}
	})

	t.Run("broken banner falls through to the body", func(t *testing.T) {
		got := resolvePostImage("/images/blog/does-not-exist.png", "", localImg, "alt")
		if got.Source != "body" {
			t.Errorf("got source %q, want %q", got.Source, "body")
		}
	})

	t.Run("local image beats a youtube embed", func(t *testing.T) {
		body := `<iframe src="https://www.youtube.com/embed/UHlleCi7j_c"></iframe>` + localImg
		got := resolvePostImage("", "", body, "alt")
		if got.Source != "body" {
			t.Errorf("got source %q, want %q - local images are meant to win", got.Source, "body")
		}
	})

	t.Run("nothing found is the zero value not the default", func(t *testing.T) {
		got := resolvePostImage("", "", `<p>Just words.</p>`, "alt")
		if got.OK() {
			t.Errorf("got %+v, want the zero value", got)
		}
		if got.Path == siteDefaultImagePath {
			t.Error("resolver substituted the site default; that belongs in SubPage.Normalise only")
		}
	})

	t.Run("svg banner is rejected", func(t *testing.T) {
		// microdata/2024/volume_vs_surface.md.json points at an SVG; there is no
		// SVG decoder and Twitter refuses SVG cards.
		if got := resolvePostImage("/images/2024/cubevsball.svg", "", "", "alt"); got.OK() {
			t.Errorf("got %+v, want nothing for an SVG banner", got)
		}
	})
}

// Standalone splits "this is the card image" from "the reader should also see it
// here". Getting it wrong is invisible in the <head> and obvious on the page:
// too strict and an author's banner never appears, too loose and every post with
// a picture shows that picture twice.
func TestResolvedImageStandalone(t *testing.T) {
	cases := []struct {
		name   string
		img    ResolvedImage
		want   bool
		reason string
	}{
		{"real image not in the body", ResolvedImage{Path: "/images/a.png"}, true,
			"nothing else on the page shows it"},
		{"real image already in the body", ResolvedImage{Path: "/images/a.png", InBody: true}, false,
			"the reader can already see it"},
		{"zero value", ResolvedImage{}, false,
			"nothing was found"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.img.Standalone(); got != c.want {
				t.Errorf("Standalone() = %v, want %v - %s", got, c.want, c.reason)
			}
		})
	}
}

// imageInBody is the half of the decision that has to look at content rather than
// provenance. The first version of this keyed off Source alone, which would have
// rendered a second copy of the banner on 22 of the 23 micro posts that set one:
// authors overwhelmingly pick a picture they also inline.
func TestImageInBody(t *testing.T) {
	t.Chdir("..")

	const ball = "/images/blog/ball.gif"
	banner := validateImage(ball, "alt", srcBanner)
	if !banner.OK() {
		t.Fatalf("fixture %s did not resolve", ball)
	}

	cases := []struct {
		name string
		img  ResolvedImage
		body string
		want bool
	}{
		{"banner the author also inlined", banner,
			`<p>Hi</p><img src="/images/blog/ball.gif" />`, true},
		{"banner inlined without a leading slash", banner,
			`<img src="images/blog/ball.gif" />`, true},
		{"banner inlined with legacy capital-B casing", banner,
			`<img src="/images/Blog/ball.gif" />`, true},
		{"banner that appears nowhere in the post", banner,
			`<p>Just words.</p><img src="/images/blog/2015-ld34.gif" />`, false},
		{"banner in a post with no images at all", banner,
			`<p>Just words.</p>`, false},
		{"scraped from the body", ResolvedImage{Path: ball, Source: srcBody},
			`<img src="/images/blog/ball.gif" />`, true},
		{"thumbnail of the post's own video", ResolvedImage{Path: "/images/yt/x.jpg", Source: srcYouTube},
			`<iframe src="https://www.youtube.com/embed/D0y5K120Vvw"></iframe>`, true},
		{"nothing resolved", ResolvedImage{}, `<img src="/images/blog/ball.gif" />`, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := imageInBody(c.img, c.body); got != c.want {
				t.Errorf("imageInBody() = %v, want %v", got, c.want)
			}
		})
	}
}

// The end-to-end version of the above, through the ladder that sets the flag.
func TestResolvePostImageMarksInBody(t *testing.T) {
	t.Chdir("..")

	t.Run("banner the author also inlined", func(t *testing.T) {
		got := resolvePostImage("/images/blog/ball.gif", "", `<img src="images/blog/ball.gif" />`, "alt")
		if !got.InBody || got.Standalone() {
			t.Errorf("got %+v, want InBody - the post already shows this picture", got)
		}
	})

	t.Run("banner kept off the page", func(t *testing.T) {
		got := resolvePostImage("/images/blog/ball.gif", "", `<p>Just words.</p>`, "alt")
		if got.InBody || !got.Standalone() {
			t.Errorf("got %+v, want a standalone hero - this is the case micro posts were losing", got)
		}
	})
}
