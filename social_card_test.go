package main

import (
	"bytes"
	"html"
	"html/template"
	"strings"
	"testing"
	"time"
)

// renderRoot runs root.html the way WritePage does, minus the file write.
func renderRoot(t *testing.T, sp *SubPage) string {
	t.Helper()

	sp.Normalise()

	var buf bytes.Buffer
	if err := RootTemp.Execute(&buf, sp); err != nil {
		t.Fatalf("root.html failed to execute: %v", err)
	}
	return buf.String()
}

// Templates are parsed from the repo root, which is where the build runs.
func TestTemplatesParse(t *testing.T) {
	t.Chdir("..")
	setupTemplates()
}

// Parsing a template does not catch a wrong field name - that only blows up when
// it executes, which without this would mean at generate time, halfway through a
// rebuild. These are the templates whose thumbnail block was rewritten to read
// .Hero instead of .BannerImage/.SmallImage.
func TestListingTemplatesExecute(t *testing.T) {
	t.Chdir("..")
	setupTemplates()

	withImage := &BlogPost{
		Key: "with", Title: "Has an image", Link: "/blog/2026/07/with/",
		ShortDesc: "Something.", Category: []BlogCat{"Godot"},
		Hero: ResolvedImage{Path: "/images/blog/ball.gif", Width: 800, Height: 600, Alt: "Has an image", Source: srcBanner},
	}
	withoutImage := &BlogPost{
		Key: "without", Title: "No image", Link: "/blog/2026/07/without/",
		ShortDesc: "Nothing.", IsMicro: true,
	}
	list := BlogList{withImage, withoutImage}

	genData = &GenerateData{
		Feed:       list,
		ShortFeed:  list,
		ShortMicro: list,
		Micro: MicroList{{
			Title: "A micro post", ShortDesc: "Short.",
			Hero: ResolvedImage{Path: "/images/blog/ball.gif", Width: 800, Height: 600, Source: srcBanner},
		}},
	}

	microTemp, err := template.ParseFiles("Templates/micro.html")
	if err != nil {
		t.Fatalf("micro.html failed to parse: %v", err)
	}
	aboutTemp, err := template.ParseFiles("Templates/about.html")
	if err != nil {
		t.Fatalf("about.html failed to parse: %v", err)
	}

	cases := []struct {
		name string
		tmpl *template.Template
		data interface{}
	}{
		{"blogindex.html", blogIndexTemp, &list},
		{"blogcat.html", blogCatTemp, &list},
		{"blogpost.html (with image)", blogTemp, withImage},
		{"blogpost.html (no image)", blogTemp, withoutImage},
		{"micro.html", microTemp, genData},
		{"about.html", aboutTemp, genData},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.tmpl.Execute(&buf, tc.data); err != nil {
				t.Fatalf("execute failed: %v", err)
			}

			out := buf.String()
			if strings.Contains(out, "postTitleImgBack") && strings.Contains(out, "url('')") {
				t.Error("emitted a thumbnail div with an empty background url")
			}
		})
	}

	// The post with no resolved image must not render a hero <img> at all - that
	// is the change that stops ~175 legacy posts showing a 120x120 logo inline.
	var buf bytes.Buffer
	if err := blogTemp.Execute(&buf, &BlogPost{Title: "Bare", Link: "/x/"}); err != nil {
		t.Fatalf("blogpost.html failed on an imageless post: %v", err)
	}
	if strings.Contains(buf.String(), `itemtype="http://schema.org/ImageObject"`) {
		t.Error("imageless post still rendered the hero image block")
	}
}

// /micro/ links straight into the blog, so MicroPost.Link has to land on the page
// BlogPost.GeneratePage actually writes. Both go through postPath; this is what
// keeps them there, because a drift here turns all 46 listing entries into 404s
// and nothing else in the build would notice.
func TestMicroLinkMatchesGeneratedPostPath(t *testing.T) {
	date := time.Date(2026, 4, 29, 11, 31, 35, 0, time.UTC)
	mp := &MicroPost{Key: "better_pr", Date: date}

	// What GenerateBlog writes for the same post.
	bp := BlogPost{Key: mp.Key}
	bp.SetNewPubDate(date)

	if mp.Link() != bp.Link {
		t.Errorf("micro listing links to %q but the post is generated at %q", mp.Link(), bp.Link)
	}
	if want := "/blog/2026/04/better_pr/"; mp.Link() != want {
		t.Errorf("got %q, want %q", mp.Link(), want)
	}
}

// Which posts render their share image on the page itself. The gate used to be
// "not a micro post", which meant the four biggest micro posts on the site set a
// bannerImage, saw it in the card and in every listing, and never once saw it on
// the post. Swapping to Standalone also stops a body-scraped or YouTube hero being
// shown above the very thing it was taken from.
func TestBlogPostHeroVisibility(t *testing.T) {
	t.Chdir("..")
	setupTemplates()

	const heroBlock = `itemtype="http://schema.org/ImageObject"`

	cases := []struct {
		name string
		post *BlogPost
		want bool
	}{
		{"blog post with a banner", &BlogPost{
			Title: "Banner", Link: "/x/",
			Hero: ResolvedImage{Path: "/images/blog/ball.gif", Width: 800, Height: 600, Source: srcBanner},
		}, true},
		{"micro post with a banner", &BlogPost{
			Title: "Micro banner", Link: "/x/", IsMicro: true,
			Hero: ResolvedImage{Path: "/images/blog/ball.gif", Width: 800, Height: 600, Source: srcBanner},
		}, true},
		{"micro post whose banner is also inlined", &BlogPost{
			Title: "Micro inline", Link: "/x/", IsMicro: true,
			Hero: ResolvedImage{Path: "/images/blog/ball.gif", Width: 800, Height: 600, Source: srcBanner, InBody: true},
		}, false},
		{"post whose image was scraped from its own body", &BlogPost{
			Title: "Body", Link: "/x/",
			Hero: ResolvedImage{Path: "/images/blog/ball.gif", Width: 800, Height: 600, Source: srcBody, InBody: true},
		}, false},
		{"post whose image is a thumbnail of its own video", &BlogPost{
			Title: "Video", Link: "/x/",
			Hero: ResolvedImage{Path: "/images/yt/D0y5K120Vvw.jpg", Width: 1280, Height: 720, Source: srcYouTube, InBody: true},
		}, false},
		{"post with nothing", &BlogPost{Title: "Bare", Link: "/x/"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := blogTemp.Execute(&buf, tc.post); err != nil {
				t.Fatalf("execute failed: %v", err)
			}

			if got := strings.Contains(buf.String(), heroBlock); got != tc.want {
				t.Errorf("hero rendered = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRootTemplateArticleCard(t *testing.T) {
	t.Chdir("..")
	setupTemplates()

	sp := &SubPage{
		Title:     "Shipping Godot VR",
		FullURL:   "/blog/2026/07/shipping-godot-vr/",
		ShortDesc: "A partial post mortem.",
		Social: &SocialCard{
			Title:         "Shipping Godot VR",
			Description:   "A partial post mortem.",
			Img:           ResolvedImage{Path: "/images/blog/2015-ld34.gif", Width: 1200, Height: 630, Mime: "image/gif", Alt: "Shipping Godot VR"},
			Type:          "article",
			PublishedTime: "2026-07-18T13:46:27+01:00",
			ModifiedTime:  "2026-07-18T13:46:27+01:00",
			Tags:          []string{"Godot", "VR"},
		},
	}

	// Compared after entity-decoding, because html/template treats <meta content>
	// as a special context and escapes characters like + to &#43;. That is valid
	// HTML and every consumer's parser decodes it - the live site already ships
	// &#39; inside descriptions - so asserting on raw bytes would just be
	// asserting on Go's escaping choices.
	out := html.UnescapeString(renderRoot(t, sp))

	// The tags that used to be computed and dropped, or never emitted at all.
	want := []string{
		`<meta property="og:image" content="` + siteBaseURL + `/images/blog/2015-ld34.gif" />`,
		`<meta property="og:image:width" content="1200" />`,
		`<meta property="og:image:height" content="630" />`,
		`<meta property="og:image:type" content="image/gif" />`,
		`<meta property="og:image:alt" content="Shipping Godot VR" />`,
		`<meta property="og:type" content="article" />`,
		`<meta property="og:url" content="` + siteBaseURL + `/blog/2026/07/shipping-godot-vr/" />`,
		`<meta name="twitter:creator" content="@EvilKimau" />`,
		`<meta name="twitter:image:alt" content="Shipping Godot VR" />`,
		`<meta property="article:published_time" content="2026-07-18T13:46:27+01:00" />`,
		`<meta property="article:tag" content="Godot" />`,
		`<meta property="article:tag" content="VR" />`,
		// 1200x630 is comfortably over Twitter's large-card floor.
		`<meta name="twitter:card" content="summary_large_image" />`,
	}

	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("root.html did not emit:\n  %s\n--- actual head ---\n%s", w, headOf(out))
		}
	}
}

func headOf(page string) string {
	if i := strings.Index(page, "</head>"); i >= 0 {
		return page[:i]
	}
	return page
}

// The whole point of the change: pages that are not blog posts used to get no
// og:image, no og:type and no twitter tags whatsoever, so they shared as bare
// text links.
func TestRootTemplateListingPageStillGetsACard(t *testing.T) {
	t.Chdir("..")
	setupTemplates()

	sp := &SubPage{
		Title:   "Blog",
		FullURL: "/blog/",
	}

	out := renderRoot(t, sp)

	for _, w := range []string{
		`<meta property="og:image" content="` + siteBaseURL + siteDefaultImagePath + `" />`,
		`<meta property="og:type" content="website" />`,
		`<meta name="twitter:card" content="`,
		`<meta property="og:image:width" content="`,
	} {
		if !strings.Contains(out, w) {
			t.Errorf("listing page did not emit:\n  %s", w)
		}
	}

	// article:* is meaningless on an index and confuses Facebook's parser.
	for _, unwanted := range []string{"article:published_time", "article:tag", "article:author"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("listing page should not emit %s", unwanted)
		}
	}

	// Falls back to the site blurb rather than an empty description.
	if !strings.Contains(out, siteDefaultDesc) {
		t.Error("listing page did not fall back to the site description")
	}
}

// A raw space in an image path is a real case (images/micro/2023 con duck.jpg)
// and both Facebook and Twitter reject the tag outright when it appears.
func TestRootTemplateEncodesImagePaths(t *testing.T) {
	t.Chdir("..")
	setupTemplates()

	sp := &SubPage{
		Title:   "Contingency",
		FullURL: "/blog/2023/01/contingency/",
		Social: &SocialCard{
			Img: ResolvedImage{Path: "/images/micro/2023 con duck.jpg", Width: 800, Height: 600},
		},
	}

	out := renderRoot(t, sp)

	if !strings.Contains(out, "/images/micro/2023%20con%20duck.jpg") {
		t.Error("image path was not percent-encoded")
	}
	if strings.Contains(out, `content="`+siteBaseURL+`/images/micro/2023 con duck.jpg"`) {
		t.Error("raw space survived into a meta tag")
	}
}

func TestNormaliseDefaults(t *testing.T) {
	t.Chdir("..")

	t.Run("nil card is filled in", func(t *testing.T) {
		sp := &SubPage{Title: "Gallery", FullURL: "/gallery/"}
		sp.Normalise()

		if sp.Social == nil {
			t.Fatal("Social is still nil; the template would emit nothing")
		}
		if sp.Social.Title != "Gallery" {
			t.Errorf("card title = %q, want the page title", sp.Social.Title)
		}
		if sp.Social.Type != "website" {
			t.Errorf("card type = %q, want %q", sp.Social.Type, "website")
		}
		if !sp.Social.Img.OK() {
			t.Error("no image was substituted")
		}
	})

	t.Run("small image gets a summary card not a large one", func(t *testing.T) {
		sp := &SubPage{
			Title:  "Old post",
			Social: &SocialCard{Img: ResolvedImage{Path: "/x.png", Width: 120, Height: 120}},
		}
		sp.Normalise()

		if sp.Social.Card != "summary" {
			t.Errorf("card = %q for a 120x120 image, want %q", sp.Social.Card, "summary")
		}
	})

	t.Run("long description is trimmed for the card only", func(t *testing.T) {
		long := strings.Repeat("word ", 200)
		sp := &SubPage{Title: "T", ShortDesc: long}
		sp.Normalise()

		if len(sp.Social.Description) > cardDescLimit {
			t.Errorf("card description is %d bytes, want <= %d", len(sp.Social.Description), cardDescLimit)
		}
		if sp.ShortDesc != long {
			t.Error("ShortDesc was truncated; the listing pages render it as body text")
		}
	})

	t.Run("alt falls back to the title", func(t *testing.T) {
		sp := &SubPage{
			Title:  "A Post",
			Social: &SocialCard{Img: ResolvedImage{Path: "/x.png", Width: 800, Height: 600}},
		}
		sp.Normalise()

		if sp.Social.Img.Alt != "A Post" {
			t.Errorf("alt = %q, want the title", sp.Social.Img.Alt)
		}
	})
}

// stripLeadHeading is the fix for descriptions that opened by repeating their own
// title, and for /micro/ rendering every heading twice.
func TestStripLeadHeading(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantHeading string
		wantRest    string
	}{
		{
			"leading h1",
			"<h1>Standards Matter</h1>\n\n<p>Back from Contingency.</p>",
			"Standards Matter",
			"\n\n<p>Back from Contingency.</p>",
		},
		{
			"heading with an id attribute",
			`<h2 id="intro">Intro</h2><p>Body.</p>`,
			"Intro",
			"<p>Body.</p>",
		},
		{
			"trailing full stop and spaces are trimmed",
			"<h1> Shipping VR. </h1><p>x</p>",
			"Shipping VR",
			"<p>x</p>",
		},
		{
			// microdata/2021/first.md opens with an image, not a heading
			"no heading leaves the body alone",
			`<span class="alignright"><img src="/images/micro/happykitty.gif" /></span><p>So I realised</p>`,
			"",
			`<span class="alignright"><img src="/images/micro/happykitty.gif" /></span><p>So I realised</p>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			heading, rest := stripLeadHeading(tc.body)
			if heading != tc.wantHeading {
				t.Errorf("heading = %q, want %q", heading, tc.wantHeading)
			}
			if rest != tc.wantRest {
				t.Errorf("rest = %q, want %q", rest, tc.wantRest)
			}
			if strings.Contains(rest, tc.wantHeading) && tc.wantHeading != "" {
				t.Errorf("heading %q survived in the body", tc.wantHeading)
			}
		})
	}
}

func TestMojibakeTripwire(t *testing.T) {
	corrupt := "important realisations about modern safety tools????????particularly the"
	if !regMojibake.MatchString(corrupt) {
		t.Error("tripwire missed the ASCII-rewrite damage that is in ten sidecars today")
	}

	for _, clean := range []string{
		"A normal description with an em dash — like this.",
		"What? Really? Yes.",
		"Back from Contingency, a lovely residential roleplaying convention.",
	} {
		if regMojibake.MatchString(clean) {
			t.Errorf("tripwire false-positived on %q", clean)
		}
	}
}
