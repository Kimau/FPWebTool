package main

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// siteDefaultImagePath is the share image for anything with nothing better.
// TitleBoard.png is 1066x600, so it renders as a real landscape card. The old
// default was fp_twitter_tiny.png at 120x120, which is a logo, not a card.
const siteDefaultImagePath = "/images/TitleBoard.png"

const (
	ytThumbDiskDir = "images/yt"  // git-committed cache, copied to the web root
	ytThumbWebDir  = "/images/yt/"
)

// offlineBuild suppresses every outbound request. Set by the -offline flag in
// main(). A build without it still survives having no network, it just pays one
// timeout before latching ytOffline.
var offlineBuild bool

// ResolvedImage is one image that has been proven to exist on disk and to decode
// with the codecs image_util.go registers. The zero value means "nothing real was
// found" - that is a meaningful state, not an error, and it is what suppresses the
// hero <img> and the listing thumbnails.
//
// Path is site-absolute, cased to match the filesystem, and stored URL-DECODED.
// Escaping happens on output in AbsPath, never here.
type ResolvedImage struct {
	Path   string
	Width  int
	Height int
	Alt    string
	Mime   string // og:image:type; empty when the extension isn't a known image
	Source string // which rung of the ladder produced it; see the src* constants

	// InBody records that this image is already on the page it belongs to, so
	// the post should not also render it as a hero. Deliberately not derivable
	// from Source: 21 of the 23 micro posts that set a bannerImage set it to a
	// picture they also include inline, so trusting the source alone would show
	// those posts the same image twice.
	InBody bool
}

// Where a resolved image came from. Worth naming rather than spelling inline,
// because Standalone turns on the difference between two of them.
const (
	srcBanner  = "banner"  // the author's bannerImage
	srcSmall   = "small"   // the author's smlImage
	srcBody    = "body"    // scraped from an <img> in the post
	srcYouTube = "youtube" // thumbnail of a video embedded in the post
	srcGallery = "gallery" // first image of a gallery set
	srcDefault = "default" // site fallback, substituted in SubPage.Normalise
)

// OK reports whether a real image was found. Callers use it to decide whether to
// render an <img> at all; the site default is substituted separately, and only at
// the frame boundary in SubPage.Normalise.
func (ri ResolvedImage) OK() bool { return ri.Path != "" }

// AbsPath is the fully-qualified URL. Unlike AbsURL it percent-encodes the path:
// /images/micro/2023 con duck.jpg is a real file, and its raw space used to ship
// straight into og:image, twitter:image and the RSS enclosure, all of which reject
// it. html/template does not escape inside content="...", so this has to happen here.
func (ri ResolvedImage) AbsPath() string { return AbsAsset(ri.Path) }

// Landscape decides summary vs summary_large_image. Twitter's documented floor for
// the large card is 300x157; below that, or on anything portrait, the large frame
// letterboxes or crops badly and a plain summary card looks better. The old code
// picked the large card whenever a bannerImage existed, so a 120x120 thumbnail
// could ask for a 2:1 frame.
func (ri ResolvedImage) Landscape() bool {
	return ri.Width >= 300 && ri.Height >= 157 && ri.Width >= ri.Height
}

// Standalone reports whether this image is worth rendering on the page that owns
// it. The card image always ships to the crawler; this is the separate question of
// whether the reader should also see it at the top of the post.
//
// The gate this replaced was "not a micro post", which was wrong in both
// directions. Micro posts that set a bannerImage but never inline it - the
// GodotCon writeups - had it in the card, in the blog listing and on /micro/, but
// never on the post itself. Meanwhile posts whose image was scraped out of their
// own body, or is the thumbnail of the video sitting in them, would render it a
// second time above the original.
func (ri ResolvedImage) Standalone() bool {
	return ri.OK() && !ri.InBody
}

// AbsAsset is AbsURL for a file path, with the path segments percent-encoded.
// INVARIANT: ResolvedImage.Path is URL-decoded (cleanImagePath unescapes), so this
// encodes exactly once.
func AbsAsset(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return siteBaseURL + (&url.URL{Path: p}).EscapedPath()
}

// //////////////////////////////////////////////////////////////////////////////
// Validation

// imageMimeTypes is the subset of extensions we are willing to hand to a social
// card. mimeTypes in build_feed.go also carries video types, which are valid
// enclosures but not valid og:image values.
var imageMimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// validateImage is the gate every candidate passes through, and the reason the
// scraping in resolvePostImage is safe to turn on at all. 39 of the 57 legacy post
// bodies containing <img> point at dead external hosts (kimau.tumblr.com and
// friends); without this they would become 39 broken og:image tags.
//
// rawSrc is whatever appeared in the source - an author's bannerImage, an <img>
// src, a gallery include. The disk path is always "." + the site path, which holds
// for /images/... and /gallery/... alike.
// canonicalSitePath turns whatever appeared in the source - an author's
// bannerImage, an <img> src, a gallery include - into the site-absolute path it
// refers to, cased to match the disk. Returns "" for anything that cannot name a
// local file.
//
// Split out of validateImage so imageInBody can compare paths without paying to
// decode every image in the post. Two normalisations earn their keep here: bodies
// write src="images/2026/x.jpg" relative to <base href="/"> while sidecars write
// "/images/2026/x.jpg", and the legacy posts write /images/Blog/ against a
// directory that is really images/blog/.
func canonicalSitePath(rawSrc string) string {
	if len(strings.TrimSpace(rawSrc)) < 4 {
		return ""
	}

	// Remote and inline sources can't be measured, and we can't guarantee they
	// still resolve. Reject before cleanImagePath mangles them into "/http:/...".
	low := strings.ToLower(rawSrc)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") ||
		strings.HasPrefix(low, "//") || strings.HasPrefix(low, "data:") {
		return ""
	}

	return canonicalCase(cleanImagePath(rawSrc))
}

func validateImage(rawSrc, alt, source string) ResolvedImage {
	sitePath := canonicalSitePath(rawSrc)
	if sitePath == "" {
		return ResolvedImage{}
	}

	mime, known := imageMimeTypes[strings.ToLower(filepath.Ext(sitePath))]
	if !known {
		// Most often an .svg banner. golang.org/x/image has no SVG decoder and
		// Twitter refuses SVG cards outright, so this is a real rejection, not a
		// gap - say so, because the author needs to drop a PNG alongside.
		log.Printf("Social: skipping %q (%s) - %s is not a usable card format\n",
			sitePath, source, filepath.Ext(sitePath))
		return ResolvedImage{}
	}

	w, h, err := getImageDimension("." + sitePath)
	if err != nil {
		return ResolvedImage{}
	}

	return ResolvedImage{
		Path:   sitePath,
		Width:  w,
		Height: h,
		Alt:    alt,
		Mime:   mime,
		Source: source,
	}
}

var (
	dirCacheMu sync.Mutex
	dirCache   = map[string]map[string]string{} // dir -> lowercase name -> real name
)

// canonicalCase rewrites a path to the casing that actually exists on disk.
//
// This is load-bearing, not defensive. The 19 legacy posts with usable local body
// images all write /images/Blog/... while the directory is images/blog. Windows
// opens either happily, so the build validates and the page looks fine locally -
// but the bucket is served case-sensitively and /images/Blog/x.png 404s in
// production. Returns the input unchanged if any segment can't be matched; the
// caller's os.Open then fails and the candidate is skipped.
func canonicalCase(sitePath string) string {
	parts := strings.Split(strings.Trim(sitePath, "/"), "/")
	dir := "."
	out := make([]string, 0, len(parts))

	for _, want := range parts {
		if want == "" {
			continue
		}
		real, ok := lookupEntry(dir, want)
		if !ok {
			return sitePath
		}
		out = append(out, real)
		dir = dir + "/" + real
	}

	return "/" + strings.Join(out, "/")
}

func lookupEntry(dir, want string) (string, bool) {
	dirCacheMu.Lock()
	defer dirCacheMu.Unlock()

	names, seen := dirCache[dir]
	if !seen {
		names = map[string]string{}
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				names[strings.ToLower(e.Name())] = e.Name()
			}
		}
		dirCache[dir] = names
	}

	real, ok := names[strings.ToLower(want)]
	return real, ok
}

var defaultImageOnce struct {
	sync.Once
	img ResolvedImage
}

// defaultSocialImage is the site-wide fallback, decoded once so the ~155 pages
// that land on it still get real og:image:width/height. The old code hardcoded
// 120x120 whether or not that matched the file.
func defaultSocialImage() ResolvedImage {
	defaultImageOnce.Do(func() {
		img := validateImage(siteDefaultImagePath, "Forest of Fun", srcDefault)
		if !img.OK() {
			log.Printf("WARNING: default social image %s did not resolve; cards will have no image\n",
				siteDefaultImagePath)
		}
		defaultImageOnce.img = img
	})
	return defaultImageOnce.img
}

// //////////////////////////////////////////////////////////////////////////////
// The resolution ladder

// resolvePostImage picks the share image for a post:
//
//	bannerImage -> smlImage -> first valid local <img> in the body ->
//	first YouTube thumbnail -> zero
//
// Local images beat YouTube deliberately: they are self-hosted, already sized for
// the site, and an author who put a picture in a post meant it to be seen. The
// YouTube rung only fires for posts whose only visual is a video - secondmoon and
// contingency being the clear cases - which used to share as the site logo.
//
// Returns the zero value when nothing survives validation. The caller substitutes
// the site default; this function never does, so "we found nothing" stays
// distinguishable from "we found the default".
func resolvePostImage(banner, small, body, alt string) ResolvedImage {
	img := pickPostImage(banner, small, body, alt)
	img.InBody = imageInBody(img, body)
	return img
}

func pickPostImage(banner, small, body, alt string) ResolvedImage {
	if img := validateImage(banner, alt, srcBanner); img.OK() {
		return img
	}
	if img := validateImage(small, alt, srcSmall); img.OK() {
		return img
	}
	if img := firstBodyImage(body, alt); img.OK() {
		return img
	}
	return firstYouTubeImage(body, alt)
}

// imageInBody answers whether the reader can already see this image in the post,
// which is what decides if the hero is worth rendering. See ResolvedImage.InBody
// for why the source alone is not enough to tell.
func imageInBody(img ResolvedImage, body string) bool {
	if !img.OK() {
		return false
	}

	// Scraped out of the body, or the thumbnail of a video embedded in it.
	if img.Source == srcBody || img.Source == srcYouTube {
		return true
	}

	for _, m := range regFindImage.FindAllStringSubmatch(body, -1) {
		if canonicalSitePath(m[1]) == img.Path {
			return true
		}
	}
	return false
}

// firstBodyImage returns the first <img> in the body that validates. It walks all
// matches rather than testing only the first, because the first <img> in a legacy
// post is very often a dead tumblr embed followed by a perfectly good local one.
func firstBodyImage(body, alt string) ResolvedImage {
	for _, m := range regFindImage.FindAllStringSubmatch(body, -1) {
		if img := validateImage(m[1], alt, srcBody); img.OK() {
			return img
		}
	}
	return ResolvedImage{}
}

// //////////////////////////////////////////////////////////////////////////////
// YouTube

// regYouTube matches every YouTube form that actually appears in blogdata/ and
// microdata/. Go uses RE2 so there is no lookahead; group 2 is a length guard -
// if a 12th id-legal character follows, the token is not an 11-character video id
// and the match is discarded.
//
// The protocol and host prefix are deliberately not anchored, so http, https,
// protocol-relative, www., m. and bare-host forms all match. Path segments are
// listed explicitly, which is what excludes /channel/UC... and /user/... - a
// channel link in a socials footer must not become the post's header image.
//
//	embed/ID          iframe embeds
//	embed//ID         one real post has a double slash; hence /+
//	/v/ID             2006-era Flash <embed>
//	youtu.be/ID       markdown links, usually followed by ) or .
//	watch?v=ID        markdown links
//	watch?a=b&v=ID    and its &amp; encoded form
var regYouTube = regexp.MustCompile(`(?i)(?:` +
	`youtube(?:-nocookie)?\.com/(?:embed|v|e|shorts|live)/+` + `|` +
	`youtube(?:-nocookie)?\.com/watch\?(?:[^"'<>\s]*&(?:amp;)?)?v=` + `|` +
	`youtu\.be/+` +
	`)([A-Za-z0-9_-]{11})([A-Za-z0-9_-]?)`)

// regYTID guards the id before it is interpolated into a file path and a URL.
var regYTID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// firstYouTubeID returns the first genuine video id in body, or "".
func firstYouTubeID(body string) string {
	for _, m := range regYouTube.FindAllStringSubmatch(body, -1) {
		if m[2] != "" {
			continue // 12+ id-legal chars, so not a video id
		}
		return m[1]
	}
	return ""
}

// firstYouTubeImage resolves the first video in the body to a cached thumbnail.
func firstYouTubeImage(body, alt string) ResolvedImage {
	id := firstYouTubeID(body)
	if id == "" {
		return ResolvedImage{}
	}

	webPath := cacheYouTubeThumb(id)
	if webPath == "" {
		return ResolvedImage{}
	}

	return validateImage(webPath, alt, srcYouTube)
}

var (
	ytClient  = &http.Client{Timeout: 10 * time.Second}
	ytOffline bool // latched on the first transport failure

	// Largest first. maxresdefault only exists if the uploader supplied an HD
	// source, so a 404 here is normal and just means try the next size down.
	ytThumbNames = []string{"maxresdefault.jpg", "sddefault.jpg", "hqdefault.jpg"}
)

// cacheYouTubeThumb guarantees images/yt/<id>.jpg exists and returns its site
// path, or "" on any failure. It never aborts the build: a missing thumbnail just
// means the post falls through to the site default.
func cacheYouTubeThumb(id string) string {
	if !regYTID.MatchString(id) {
		return ""
	}

	webPath := ytThumbWebDir + id + ".jpg"
	diskPath := filepath.Join(ytThumbDiskDir, id+".jpg")

	if _, err := os.Stat(diskPath); err == nil {
		mirrorThumbToWebRoot(diskPath, id)
		return webPath
	}

	if offlineBuild || ytOffline {
		return ""
	}

	for _, name := range ytThumbNames {
		body, err := ytFetch("https://i.ytimg.com/vi/" + id + "/" + name)
		if err != nil {
			if _, isHTTP := err.(ytStatusError); isHTTP {
				continue // this size doesn't exist, try the next
			}
			ytOffline = true
			log.Println("YouTube unreachable, skipping thumbnails for the rest of this build:", err)
			return ""
		}

		// Trust the pixels, not the status code: i.ytimg.com answers 200 with a
		// 120x90 grey placeholder for sizes it doesn't have, and enshrining that
		// in a committed cache would be worse than having no thumbnail.
		cfg, _, decErr := image.DecodeConfig(bytes.NewReader(body))
		if decErr != nil || cfg.Width < 320 || cfg.Height < 180 {
			continue
		}

		if err := writeFileAtomic(diskPath, body); err != nil {
			log.Println("Could not cache YouTube thumb", id, "-", err)
			return ""
		}

		mirrorThumbToWebRoot(diskPath, id)
		log.Printf("Cached YouTube thumb %s (%s, %dx%d)\n", id, name, cfg.Width, cfg.Height)
		return webPath
	}

	log.Printf("No usable thumbnail for YouTube video %s\n", id)
	return ""
}

// ytStatusError marks a non-200 response, which is recoverable (try a smaller
// size) as opposed to a transport failure, which means we are offline.
type ytStatusError int

func (e ytStatusError) Error() string { return fmt.Sprintf("http %d", int(e)) }

func ytFetch(rawURL string) ([]byte, error) {
	resp, err := ytClient.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ytStatusError(resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// mirrorThumbToWebRoot copies a cached thumb straight into public_html.
//
// Generate() copies images/ to the web root and waits for it (fpWebMain.go) before
// genWebsite() runs, so a thumbnail fetched during load lands in images/yt/ but
// misses this build's output entirely. That failure mode is nasty: the page 404s
// in production and then self-heals on the next build, so it never reproduces
// locally. Mirroring on both cache hit and cache miss closes it.
func mirrorThumbToWebRoot(diskPath, id string) {
	destDir := filepath.Join(publicHtmlRoot, "images", "yt")
	if err := os.MkdirAll(destDir, 0777); err != nil {
		return
	}
	if _, err := CopyFileLazy(diskPath, filepath.Join(destDir, id+".jpg")); err != nil {
		log.Println("Could not mirror YouTube thumb to web root:", err)
	}
}

// writeFileAtomic writes via a temp file and a rename. The cache is git-committed,
// so an interrupted build must not leave a truncated JPEG staged for commit.
func writeFileAtomic(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0777); err != nil {
		return err
	}

	// filepath.Base, not path.Base: dest is an OS path and on Windows path.Base
	// does not split on the backslash, so the "pattern" ends up containing a
	// separator and CreateTemp refuses it.
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return err
	}

	return nil
}
