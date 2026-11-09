package main

import (
	"regexp"
	"strings"
)

var (
	// Links and embeds pointing at this site over http, or at the bare domain.
	// Each one hands Google another http:// URL to crawl, which is exactly the
	// duplicate the canonical tags are trying to get rid of.
	regOwnSiteHTTP = regexp.MustCompile(`(=\s*["'])https?://(?:www\.)?claire-blackshaw\.com/`)

	// An http iframe on an https page is blocked outright as mixed content, so
	// these YouTube embeds rendered as empty boxes.
	regYouTubeHTTP = regexp.MustCompile(`(=\s*["'])http://(www\.)?youtube\.com/`)

	// The 2007 Flash-era embed: an <object> wrapping youtube.com/v/ID. Flash is
	// gone, so it is swapped for the iframe player. Matched per <object> block,
	// so a Slideshare object can't be swallowed along with a YouTube one.
	regObject       = regexp.MustCompile(`(?is)<object\b.*?</object>`)
	regYouTubeFlash = regexp.MustCompile(`youtube\.com/v/([A-Za-z0-9_-]{11})`)

	// Local image references, in src or in the href of a click-to-enlarge link.
	regLocalImageRef = regexp.MustCompile(`(?i)(\b(?:src|href)\s*=\s*["'])(/?images/[^"'%]+)(["'])`)

	// Links into the 2006-2011 site, from when flammablepenguins.com was this
	// site. The WordPress import writes most of them root-relative, so they now
	// resolve against this site and 404. Also catches absolute links to
	// flammablepenguins.com and the old itpc:// podcast subscribe links. Only
	// paths in legacyPaths are rewritten; the query string is dropped.
	regLegacyLink = regexp.MustCompile(`(?i)(\bhref\s*=\s*["'])(?:(?:https?|itpc)://(?:www\.)?flammablepenguins\.com)?(/[^"'?#\s]*)[^"']*(["'])`)

	// The dead Google Fusion "add to reader" buttons carry the podcast feed,
	// URL-encoded, at its old flammablepenguins.com address.
	regLegacyFeed = regexp.MustCompile(`(?i)http(%3A|:)//(?:www\.)?flammablepenguins\.com/FPPod\.xml`)
)

// legacyPaths maps pages of the old flammablepenguins.com personal site to
// where that content lives now. Keys are lowercase. On flammablepenguins.com
// itself the same paths 301 to the same places (compweb's
// .cloudfront/redirects.js), so keep the two tables in step.
var legacyPaths = map[string]string{
	"/index.php":      "/",
	"/about.php":      "/",
	"/feedback.html":  "/",
	"/gamefeed.php":   "/",
	"/hobbies.html":   "/",
	"/hobby":          "/",
	"/hobby/":         "/",
	"/drama.html":     "/",
	"/tales.php":      "/",
	"/talesother.php": "/",

	"/cv.html":       "/job/",
	"/cv.php":        "/job/",
	"/profwork.html": "/job/",

	"/code.html":     "/projects/",
	"/code.php":      "/projects/",
	"/code_api.php":  "/projects/",
	"/code_tut.php":  "/projects/",
	"/codebox.php":   "/projects/",
	"/coderef.php":   "/blog/cat/Ptk/",
	"/code_week.php": "/blog/cat/Weekend_of_Code_Contest/",

	"/art.html":   "/gallery/",
	"/comic.html": "/gallery/",
	"/comic.php":  "/gallery/",

	"/rpg.html":    "/blog/cat/Roleplaying_Games/",
	"/talesrp.php": "/blog/cat/Roleplaying_Games/",

	"/game.php":       "/blog/cat/Game_Pit/",
	"/gamecomp.php":   "/blog/cat/Game_Pit/",
	"/gamesubmit.php": "/blog/cat/Game_Pit/",
	"/gamepit.xml":    "/blog/cat/Game_Pit/",

	"/podcast.php":     "/blog/cat/Podcast/",
	"/gamepod.php":     "/blog/cat/Podcast/",
	"/podsuggest.html": "/blog/cat/Podcast/",

	"/fppod.xml": "/FPPod.xml",
}

// fixBodyURLs repairs post bodies on the way out. The sources are left as they
// are: blogdata/ is the untouched WordPress import, and micro posts are Claire's
// markdown.
func fixBodyURLs(body string) string {
	body = regOwnSiteHTTP.ReplaceAllString(body, "${1}"+siteBaseURL+"/")

	// The legacy posts write /images/Blog/... and the folder is images/blog/.
	// canonicalCase already fixed this for share images, but the <img> tags in
	// the body kept the wrong case, and S3 is case-sensitive: images that show
	// fine on a Windows preview are 404s on the live site. Only a pure case
	// difference is rewritten; a file that is genuinely missing is left alone.
	body = regLocalImageRef.ReplaceAllStringFunc(body, func(m string) string {
		p := regLocalImageRef.FindStringSubmatch(m)
		want := cleanImagePath(p[2])
		real := canonicalCase(want)
		if real == want || !strings.EqualFold(real, want) {
			return m
		}
		return p[1] + real + p[3]
	})

	body = regLegacyLink.ReplaceAllStringFunc(body, func(m string) string {
		p := regLegacyLink.FindStringSubmatch(m)
		to, ok := legacyPaths[strings.ToLower(p[2])]
		if !ok {
			return m
		}
		return p[1] + to + p[3]
	})
	body = regLegacyFeed.ReplaceAllString(body, "https${1}//www.claire-blackshaw.com/FPPod.xml")

	body = regYouTubeHTTP.ReplaceAllString(body, "${1}https://www.youtube.com/")
	body = regObject.ReplaceAllStringFunc(body, func(obj string) string {
		m := regYouTubeFlash.FindStringSubmatch(obj)
		if m == nil {
			return obj
		}
		return `<iframe width="560" height="315" src="https://www.youtube.com/embed/` + m[1] +
			`" title="YouTube video" frameborder="0" allowfullscreen></iframe>`
	})
	return body
}
