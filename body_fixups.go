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
)

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
