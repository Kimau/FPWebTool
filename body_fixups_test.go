package main

import (
	"strings"
	"testing"
)

func TestFixBodyURLs(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"own site over http",
			`<a href="http://www.claire-blackshaw.com/blog/2011/08/judging-daring-doers/">x</a>`,
			`<a href="https://www.claire-blackshaw.com/blog/2011/08/judging-daring-doers/">x</a>`,
		},
		{
			"bare domain",
			`<a href='http://claire-blackshaw.com/projects/'>x</a>`,
			`<a href='https://www.claire-blackshaw.com/projects/'>x</a>`,
		},
		{
			"own-site audio",
			`<audio src="http://www.claire-blackshaw.com/Audio/fppod01_crossplatform.mp3">`,
			`<audio src="https://www.claire-blackshaw.com/Audio/fppod01_crossplatform.mp3">`,
		},
		{
			"http youtube iframe",
			`<iframe src="http://www.youtube.com/embed/fzMhh8zhTiY" width="560">`,
			`<iframe src="https://www.youtube.com/embed/fzMhh8zhTiY" width="560">`,
		},
		{
			"link text and other sites are left alone",
			`<p>See http://www.claire-blackshaw.com/ or <a href="http://example.com/">this</a></p>`,
			`<p>See http://www.claire-blackshaw.com/ or <a href="http://example.com/">this</a></p>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fixBodyURLs(tc.in); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// Runs against the real images/ folder, which is images/blog in lowercase.
func TestFixBodyURLsImageCase(t *testing.T) {
	t.Chdir("..")

	in := `<a href="/images/Blog/itunes.gif"><img src="/images/Blog/itunes.gif" alt=""></a>` +
		`<img src="/images/Blog/does-not-exist-anywhere.png">` +
		`<img src="images/blog/itunes.gif">`
	want := `<a href="/images/blog/itunes.gif"><img src="/images/blog/itunes.gif" alt=""></a>` +
		`<img src="/images/Blog/does-not-exist-anywhere.png">` +
		`<img src="images/blog/itunes.gif">`

	if got := fixBodyURLs(in); got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestFixBodyURLsFlashYouTube(t *testing.T) {
	// Verbatim from blogdata/post/2007/the-best-pick-me-up-ever.html, followed by
	// a Slideshare object that must survive untouched.
	slides := `<object id="__sse5446385" width="425" height="355"><param name="movie" value="http://static.slidesharecdn.com/swf/ssplayer2.swf?doc=fpp1"/></object>`
	in := `<p>Intro.</p>
<object width="425" height="350"><param name="movie" value="http://www.youtube.com/v/QGO2hVA3P58"></param><param name="wmode" value="transparent"></param><embed src="http://www.youtube.com/v/QGO2hVA3P58" type="application/x-shockwave-flash" wmode="transparent" width="425" height="350"></embed></object>` + slides

	got := fixBodyURLs(in)

	if !strings.Contains(got, `src="https://www.youtube.com/embed/QGO2hVA3P58"`) {
		t.Errorf("flash embed was not converted:\n%s", got)
	}
	if strings.Contains(got, "shockwave") {
		t.Error("flash markup survived")
	}
	if !strings.Contains(got, slides) {
		t.Error("slideshare object was altered")
	}
}
