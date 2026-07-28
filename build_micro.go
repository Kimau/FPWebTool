package main

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
)

// Updated MicroPost struct
type MicroPost struct {
	Version     int       `json:"version"`
	Key         string    `json:"key,omitempty"`
	Title       string    `json:"title"`
	Date        time.Time `json:"pubDate"`
	Class       string    `json:"classname"`
	ShortDesc   string    `json:"desc,omitempty"`
	SmallImage  string    `json:"smlImage,omitempty"`
	BannerImage string    `json:"bannerImage,omitempty"`

	Body    template.HTML `json:"-"`
	DateStr string        `json:"-"`
	Pubdate string        `json:"-"`

	// Hero is the resolved share image, derived every build. Drives the thumbnail
	// on /micro/ and that page's own card. Never persisted - the sidecar's
	// bannerImage stays the author's field.
	Hero ResolvedImage `json:"-"`
}

// microPostVersion gates re-derivation of the sidecar's computed fields. Bumping
// it rewrites every .md.json on the next build.
//
// v3: descriptions are derived from the body *after* its leading heading has been
// removed. Under v2 they were derived before, so every micro post's
// og:description opened by repeating its own title word for word.
const microPostVersion = 3

var regFindImage = regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
var regMicroHeader = regexp.MustCompile(`<h[1-6][^>]*>([^<]*)</h[1-6]>`)
var reNonAlnum = regexp.MustCompile(`[^a-z0-9 ]+`)
var reSpaces = regexp.MustCompile(`\s+`)

// regMojibake spots the residue of a text tool that wrote UTF-8 out as ASCII: a
// single em dash becomes eight literal question marks. Ten sidecars carry this,
// and it was riding straight into og:description while the .md sources were clean
// all along. Matching it forces a re-derive, so the damage self-heals rather than
// needing another manual pass.
var regMojibake = regexp.MustCompile("\\?{3,}|\uFFFD|â€|Ã[\u0080-\u00BF]")

// stripLeadHeading removes the first heading from the body and returns it
// separately. Both the title and the description are then derived from the
// result.
//
// This used to happen in two places and neither was early enough: enrichMicroPost
// derived the description from a body that still had its <h1>, and the heading was
// only removed later, in LoadFromMicroListFolder, on a local copy that never made
// it back to MicroPost.Body. Hence descriptions that repeated their own title, and
// /micro/ rendering every heading twice.
func stripLeadHeading(body string) (heading, rest string) {
	loc := regMicroHeader.FindStringSubmatchIndex(body)
	if loc == nil {
		return "", body
	}

	heading = strings.Trim(body[loc[2]:loc[3]], " .\n")
	rest = body[:loc[0]] + body[loc[1]:]
	return heading, rest
}

// DateISO is the post date in ISO 8601, as required by <time datetime>.
func (mp *MicroPost) DateISO() string {
	return mp.Date.Format(time.RFC3339)
}

// Link is the post's permalink, in the blog where it is published. /micro/ renders
// every post in full but had no way to reach one: no entry on that page linked to
// anything but whatever the author happened to put in the body, so a micro post
// could be read there and never shared, cited or linked. The blog listing has had
// this since the beginning.
func (mp *MicroPost) Link() string {
	return postPath(mp.Date, mp.Key)
}

// upperFirst capitalises the first rune. Slicing [0:1] cuts a multi-byte rune in
// half, and panics outright on an empty string.
func upperFirst(s string) string {
	if s == "" {
		return s
	}

	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size <= 1 {
		return s
	}

	return string(unicode.ToUpper(r)) + s[size:]
}

// //////////////////////////////////////////////////////////////////////////////
// Blog Listing
type MicroList []*MicroPost

func (bl MicroList) Len() int           { return len(bl) }
func (bl MicroList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl MicroList) Less(i, j int) bool { return bl[i].Date.After(bl[j].Date) }

// enrichMicroPost re-derives the sidecar's computed fields from the body, which
// by this point has already had its leading heading removed.
//
// The description is derived data: it is rewritten whenever this runs, i.e. on a
// version bump or when the stored one is corrupt. A hand-edited desc survives
// normal builds but not a bump - that is the trade for being able to fix all of
// them at once.
//
// It no longer scrapes an image. Share images are resolved fresh on every build in
// resolvePostImage, so caching one here bought nothing and froze the result: the
// version gate meant the old scrape never re-ran once a sidecar was written, and
// the first-<img> answer was stale the moment the heuristic changed. bannerImage
// in the sidecar is now purely the author's override.
func enrichMicroPost(post *MicroPost) {
	braw := string(post.Body)
	if len(braw) == 0 {
		return
	}

	p := bluemonday.StripTagsPolicy()
	plain := p.Sanitize(braw)
	plain = html.UnescapeString(plain)
	plain = reSpaces.ReplaceAllString(plain, " ")
	post.ShortDesc = truncateRunes(plain, 400)

	// Three sidecars store this without a leading slash. cleanImagePath forgives
	// it downstream, but there is no reason to keep writing it out inconsistent.
	if post.BannerImage != "" {
		post.BannerImage = cleanImagePath(post.BannerImage)
	}
}

func LoadSingleFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	title := filepath.Base(path)
	ext := filepath.Ext(path)
	title = strings.TrimSuffix(title, ext)

	var newPost MicroPost
	jsonPath := path + ".json"
	hasExistingJSON := false

	if ext == ".md" {
		markdown, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}
		newPost.Body = MarkdownToHTML(markdown)

	} else if ext == ".html" {
		body, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}
		newPost.Body = template.HTML(body)
	} else if ext == ".json" {
		return nil
	} else {
		fmt.Println("Didn't parse: " + path)
		return nil
	}

	// Lift the heading out before anything reads the body, so the title and the
	// description are both derived from the same, heading-free text. This is the
	// ordering that was wrong: it used to happen after enrichment, and on a copy.
	heading, body := stripLeadHeading(string(newPost.Body))
	newPost.Body = template.HTML(body)

	// Load existing JSON if present
	if _, statErr := os.Stat(jsonPath); statErr == nil {
		loadJSONBlob(jsonPath, &newPost)
		hasExistingJSON = true
	} else {
		newPost.Date = info.ModTime()
	}

	// The body's own heading is the title. The filename is only a fallback for a
	// post that doesn't have one - microdata/2021/first.md is the only such post,
	// and its sidecar title is the lowercase filename, so the tidy-up has to apply
	// whichever source won.
	if heading != "" {
		newPost.Title = heading
	} else if newPost.Title == "" {
		newPost.Title = title
	}
	newPost.Title = upperFirst(strings.Trim(newPost.Title, " .\n"))

	// Always recompute derived fields
	newPost.Pubdate = newPost.Date.Format(longformPubStr)
	newPost.DateStr = fmt.Sprintf("%d %v %d", newPost.Date.Day(), newPost.Date.Month(), newPost.Date.Year())

	// Re-derive on a version bump, or when the stored description is visibly
	// corrupt - ten sidecars carry mojibake from a past ASCII rewrite, and their
	// .md sources are clean, so a re-derive is all it takes to fix them.
	needsSave := !hasExistingJSON
	if newPost.Version < microPostVersion || regMojibake.MatchString(newPost.ShortDesc) {
		enrichMicroPost(&newPost)
		newPost.Version = microPostVersion
		needsSave = true
	}

	if newPost.Key == "" {
		k := strings.ToLower(newPost.Title)
		k = reNonAlnum.ReplaceAllString(k, "")
		k = strings.TrimSpace(k)
		k = reSpaces.ReplaceAllString(k, "-")
		newPost.Key = k
		needsSave = true
	}

	genData.Micro = append(genData.Micro, &newPost)

	if needsSave {
		log.Printf("Updating micro JSON (v%d): %s\n", newPost.Version, jsonPath)
		saveJSONBlob(jsonPath, &newPost)
	}

	return nil
}

func LoadFromMicroListFolder() {
	err := filepath.Walk("./microdata", LoadSingleFile)
	if err != nil {
		log.Println(err)
	}

	// Micro posts are published as blog posts too, at the same /blog/YYYY/MM/key/
	// URL, so they flow through BlogPost.GeneratePage and pick up its social card
	// for free. The heading has already been lifted and the description derived at
	// load; anything still missing is filled by EnsureShortDesc in resolveAllSocial.
	for _, v := range genData.Micro {
		blogFromMicro := BlogPost{
			Key:         v.Key,
			Title:       v.Title,
			Date:        v.Date,
			Body:        v.Body,
			ShortDesc:   v.ShortDesc,
			BannerImage: v.BannerImage,
			SmallImage:  v.SmallImage,
		}

		blogFromMicro.RawCategory = []BlogCat{"micro"}
		blogFromMicro.Category = []BlogCat{"micro"}
		blogFromMicro.SetNewPubDate(v.Date)
		blogFromMicro.IsMicro = true
		blogFromMicro.Class = v.Class
		genData.Feed = append(genData.Feed, &blogFromMicro)
	}
}

////////////////////////////////////////////////////////////////////////////////
//

func init() {

}

// MarkdownToHTML - Convert Markdown to HTML
func MarkdownToHTML(input []byte) template.HTML {

	renderer := blackfriday.HtmlRenderer(0|
		blackfriday.HTML_USE_XHTML, "", "")
	output := blackfriday.MarkdownOptions(input, renderer,
		blackfriday.Options{
			Extensions: blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
				blackfriday.EXTENSION_TABLES |
				blackfriday.EXTENSION_FENCED_CODE |
				blackfriday.EXTENSION_AUTOLINK |
				blackfriday.EXTENSION_STRIKETHROUGH |
				blackfriday.EXTENSION_HARD_LINE_BREAK |
				blackfriday.EXTENSION_SPACE_HEADERS |
				blackfriday.EXTENSION_HEADER_IDS |
				blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
				blackfriday.EXTENSION_DEFINITION_LISTS,
		},
	)
	return template.HTML(output)
}

func GenerateMicro() {
	microTemp, err := template.ParseFiles("Templates/micro.html")
	CheckErr(err)

	sort.Sort(genData.Micro)

	var outBuffer bytes.Buffer
	err = microTemp.Execute(&outBuffer, genData)
	CheckErrContext(err, "Error in Template ")

	const desc = "Short posts and half-thoughts from Claire Blackshaw - too big for a tweet, too small for a blog post."

	// The newest micro with a real image, which is not always the newest post.
	var newest ResolvedImage
	for _, mp := range genData.Micro {
		if mp.Hero.OK() {
			newest = mp.Hero
			break
		}
	}

	WritePage(&SubPage{
		Title:     "Micro Posts",
		FullURL:   "/micro/",
		ShortDesc: desc,
		Content:   template.HTML(outBuffer.String()),
		Social:    listingCard("Micro Posts", desc, newest),
	}, publicHtmlRoot+"micro/index.html")
}
