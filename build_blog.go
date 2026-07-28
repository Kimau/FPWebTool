package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type BlogCat string
type BlogPost struct {
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Pubdate     string    `json:"pubDate"`
	SmallImage  string    `json:"smlImage,omitempty"`
	BannerImage string    `json:"bannerImage,omitempty"`
	ShortDesc   string    `json:"desc,omitempty"`
	RawCategory []BlogCat `json:"category"`
	Class       string    `json:"classname"`

	Image       string `json:"image,omitempty"`
	ImageWidth  string `json:"imageWidth,omitempty"`
	ImageHeight string `json:"imageHeight,omitempty"`

	Category []BlogCat     `json:"-"`
	Date     time.Time     `json:"-"`
	Body     template.HTML `json:"-"`
	DateStr  string        `json:"-"`
	IsMicro  bool          `json:"-"`
}

var (
	blogTemp, blogIndexTemp, blogCatTemp *template.Template

	regUrlChar       *regexp.Regexp
	regUrlSpace      *regexp.Regexp
	regLegacyUrlChar *regexp.Regexp
	regStripMarkup   *regexp.Regexp
)

const longformPubStr = "Mon, 02 Jan 2006 15:04:05 -0700"

////////////////////////////////////////////////////////////////////////////////
//

func init() {
	var err error

	// Drop punctuation but keep digits, then collapse whitespace runs into a
	// single underscore. The old version stripped digits and spaces in one pass,
	// which silently turned "Html5" into "Html" and made the space rule dead.
	regUrlChar = regexp.MustCompile(`[^A-Za-z0-9\s]`)
	regUrlSpace = regexp.MustCompile(`\s+`)
	regLegacyUrlChar = regexp.MustCompile(`[^A-Za-z]`)
	regStripMarkup = regexp.MustCompile("<[^<>]*>")

	blogIndexTemp, err = template.ParseFiles("Templates/blogindex.html")
	CheckErr(err)

	blogTemp, err = template.ParseFiles("Templates/blogpost.html")
	CheckErr(err)

	blogCatTemp, err = template.ParseFiles("Templates/blogcat.html")
	CheckErr(err)
}

// truncateRunes cuts s to at most maxBytes without splitting a UTF-8 rune, then
// backs up to the last word boundary so descriptions don't end mid-word.
func truncateRunes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return strings.TrimSpace(s)
	}

	// Back off to a valid rune boundary.
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	s = s[:cut]

	// Then back off to the last space, provided that leaves something useful.
	if idx := strings.LastIndexFunc(s, unicode.IsSpace); idx > maxBytes/2 {
		s = s[:idx]
	}

	return strings.TrimSpace(s)
}

// //////////////////////////////////////////////////////////////////////////////
// Blog Cat
func (c BlogCat) UrlVer() string {
	s := regUrlChar.ReplaceAllString(string(c), "")
	return regUrlSpace.ReplaceAllString(strings.TrimSpace(s), "_")
}

// LegacyUrlVer reproduces the pre-fix slug, which stripped digits and spaces in
// one pass. Only used to decide where to leave a redirect behind.
func (c BlogCat) LegacyUrlVer() string {
	return regLegacyUrlChar.ReplaceAllString(string(c), "")
}

// //////////////////////////////////////////////////////////////////////////////
// Blog Listing
type BlogList []*BlogPost

func (bl BlogList) Len() int           { return len(bl) }
func (bl BlogList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl BlogList) Less(i, j int) bool { return bl[i].Date.After(bl[j].Date) }

func (bl *BlogList) Get(key string) *BlogPost {
	for _, v := range *bl {
		if v.Key == key {
			return v
		}
	}

	return nil
}

func (bl *BlogList) LoadFromFile() {
	loadJSONBlob("blogdata/blogData.js", bl)
}

func (bl *BlogList) SaveToFile() {
	for _, v := range *bl {
		v.SaveBodyToFile()
	}
	saveJSONBlob("blogdata/blogData.js", bl)
}

func (bl *BlogList) GeneratePage() {
	var outBuffer bytes.Buffer
	err := blogIndexTemp.Execute(&outBuffer, bl)
	CheckErrContext(err, "Error in Template ")

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog",
		FullURL: "/blog/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(publicHtmlRoot + "blog/index.html")
	CheckErrContext(fileErr, "Error in File ")

	err = RootTemp.Execute(f, frameData)
	CheckErr(err)

	f.Close()
}

// //////////////////////////////////////////////////////////////////////////////
// Blog Post
func (bp *BlogPost) LoadBodyFromFile() error {
	srcFile := fmt.Sprintf("blogdata/post/%d/%s.html", bp.Date.Year(), bp.Key)
	bodyBytes, err := os.ReadFile(srcFile)
	CheckErr(err)

	bp.Body = template.HTML(bodyBytes)
	return nil
}

func (bp *BlogPost) SaveBodyToFile() error {
	if len(bp.Body) < 8 {
		return errors.New("body is null or less than 8 characters")
	}

	// Make Folder
	destFolder := fmt.Sprintf("blogdata/post/%d", bp.Date.Year())
	err := os.MkdirAll(destFolder, 0755)
	if err != nil {
		return err
	}

	srcFile := fmt.Sprintf("blogdata/post/%d/%s.html", bp.Date.Year(), bp.Key)

	os.Remove(srcFile)
	err = ioutil.WriteFile(srcFile, []byte(bp.Body), 0777)
	if err != nil {
		CheckErr(err)
	}

	return nil
}

// DateISO is the post date in ISO 8601. The HTML <time datetime> attribute and
// schema.org datePublished both require it; Pubdate is RFC1123 and is rejected.
func (bp *BlogPost) DateISO() string {
	return bp.Date.Format(time.RFC3339)
}

// AbsLink is the fully-qualified post URL, for schema.org mainEntityOfPage.
func (bp *BlogPost) AbsLink() string {
	return AbsURL(bp.Link)
}

func (bp *BlogPost) FixupDateFromPubStr() {
	var err error

	bp.Date, err = time.Parse(longformPubStr, bp.Pubdate)
	if err != nil {
		CheckErr(err)
	}

	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
	bp.Link = fmt.Sprintf("/blog/%04d/%02d/%s/", bp.Date.Year(), bp.Date.Month(), bp.Key)
}

func (bp *BlogPost) SetNewPubDate(newPubDate time.Time) {
	bp.Date = newPubDate
	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
	bp.Link = fmt.Sprintf("/blog/%04d/%02d/%s/", bp.Date.Year(), bp.Date.Month(), bp.Key)
	bp.Pubdate = bp.Date.Format(longformPubStr)
}

func (bp *BlogPost) GeneratePage() {
	var err error

	log.Println(bp.Link)

	err = os.MkdirAll(publicHtmlRoot+bp.Link, 0777)
	CheckErrContext(err, "Error in Mkdir ")

	// Get Banner Image Size (if I have one)
	if len(bp.BannerImage) > 3 {
		bp.BannerImage = cleanImagePath(bp.BannerImage)
		w, h, e := getImageDimension("." + bp.BannerImage)
		if e != nil {
			log.Println("Warning: Error getting Banner:", bp.Title, "\n>", bp.BannerImage, "\n>", e)
			// Fall through to SmallImage or default
		} else {
			bp.Image = bp.BannerImage
			bp.ImageWidth = fmt.Sprintf("%d", w)
			bp.ImageHeight = fmt.Sprintf("%d", h)
		}
	}

	if bp.Image == "" && len(bp.SmallImage) > 3 {
		bp.SmallImage = cleanImagePath(bp.SmallImage)
		bp.Image = bp.SmallImage
		bp.ImageWidth, bp.ImageHeight = "120", "120"
	}

	if bp.Image == "" {
		bp.Image = "/images/fp_twitter_tiny.png"
		bp.ImageWidth, bp.ImageHeight = "120", "120"
	}

	var outBuffer bytes.Buffer
	err = blogTemp.Execute(&outBuffer, bp)
	CheckErr(err)

	blogBody := template.HTML(outBuffer.String())

	// Twitter Card
	if len(bp.ShortDesc) < 4 {
		// Build Desc
		sum := regStripMarkup.ReplaceAllString(string(bp.Body), " ")
		bp.ShortDesc = truncateRunes(sum, 200)
	}

	tc := &TwitterCard{
		Card:        "summary",
		Site:        "@EvilKimau",
		Title:       bp.Title,
		Description: bp.ShortDesc,
		Image:       "/images/fp_twitter_tiny.png",
	}

	if len(bp.BannerImage) > 3 {
		tc.Card = "summary_large_image"
		tc.Image = bp.BannerImage
	} else if len(bp.SmallImage) > 3 {
		tc.Image = bp.SmallImage
	}

	// Write out Frame
	frameData := &SubPage{
		Title:     bp.Title,
		FullURL:   bp.Link,
		ShortDesc: bp.ShortDesc,
		Content:   blogBody,
		Twitter:   tc,
	}

	f, fileErr := os.Create(publicHtmlRoot + bp.Link + "index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	// Note: Don't like the fact we reference RootTemp here
	err = RootTemp.Execute(f, frameData)
	CheckErr(err)

	f.Close()
}

// //////////////////////////////////////////////////////////////////////////////
// Entry Point
func GenerateBlog() {
	var err error

	os.RemoveAll(publicHtmlRoot + "blog/")
	err = os.MkdirAll(publicHtmlRoot+"blog/", 0777)
	CheckErrContext(err, "Unable to make folder")

	// Gather Catergories and filter out single use catergories
	catMap := make(map[BlogCat]BlogList)
	for _, v := range genData.Feed {
		for _, c := range v.RawCategory {
			catMap[c] = append(catMap[c], v)
		}
	}

	removedCat := []BlogCat{}
	for _, v := range genData.Feed {
		v.Category = []BlogCat{}
		for _, c := range v.RawCategory {
			if len(catMap[c]) < 2 {
				if _, stillThere := catMap[c]; stillThere {
					removedCat = append(removedCat, "-"+c)
				}
				delete(catMap, c)
			} else {
				v.Category = append(v.Category, c)
			}
		}

		v.FixupDateFromPubStr()
		if len(v.Body) < 1 {
			err := v.LoadBodyFromFile()
			if err != nil {
				CheckErr(err)
				return
			}
		}

		v.GeneratePage()
	}
	log.Println("Removed ", removedCat)

	sort.Sort(genData.Feed)
	genData.Feed.GeneratePage()

	// Record the surviving categories so the sitemap can list them, and shout if
	// two of them slug down to the same folder (last write would silently win).
	slugOwner := make(map[string]BlogCat)
	genData.Categories = genData.Categories[:0]
	for k, v := range catMap {
		if prev, clash := slugOwner[k.UrlVer()]; clash {
			log.Printf("WARNING: categories %q and %q both map to /blog/cat/%s/ - one will overwrite the other\n", prev, k, k.UrlVer())
		}
		slugOwner[k.UrlVer()] = k

		genData.Categories = append(genData.Categories, k)
		GenerateBlogCatergoryPage(k, &v)
	}
	sort.Slice(genData.Categories, func(i, j int) bool { return genData.Categories[i] < genData.Categories[j] })

	// Fixing the slug (it used to eat digits, so "Ludum Dare 48" became
	// "LudumDare") moves category URLs Google has already crawled. Leave a
	// redirect at each old path rather than letting it 404.
	legacyDone := make(map[string]bool)
	for _, c := range genData.Categories {
		legacy := c.LegacyUrlVer()
		if legacy == "" || legacy == c.UrlVer() || legacyDone[legacy] {
			continue
		}
		if _, taken := slugOwner[legacy]; taken {
			continue // a real category already lives there
		}

		legacyDone[legacy] = true
		WriteRedirectStub("/blog/cat/"+legacy+"/", "/blog/cat/"+c.UrlVer()+"/")
	}
	log.Printf("Wrote %d legacy category redirects\n", len(legacyDone))

	buildShortLists()
}

// WriteRedirectStub leaves a page that sends both crawlers and browsers to dest.
// The bucket is served as an S3 static site behind CloudFront, so we can't emit
// a real 301 from the build; a canonical plus a zero-delay refresh is the
// closest equivalent and Google treats it as a permanent move.
func WriteRedirectStub(fromPath string, dest string) {
	err := os.MkdirAll(publicHtmlRoot+fromPath, 0777)
	CheckErrContext(err, "Error in Mkdir ")

	f, err := os.Create(publicHtmlRoot + fromPath + "index.html")
	CheckErrContext(err, "Error in File ")
	defer f.Close()

	absDest := AbsURL(dest)
	fmt.Fprintf(f, `<!doctype html>
<html lang="en-GB">
<head>
<meta charset="utf-8">
<title>Moved</title>
<link rel="canonical" href="%s">
<meta name="robots" content="noindex, follow">
<meta http-equiv="refresh" content="0; url=%s">
</head>
<body><p>This page has moved to <a href="%s">%s</a>.</p></body>
</html>
`, absDest, absDest, absDest, absDest)
}

func GenerateBlogCatergoryPage(cat BlogCat, blist *BlogList) {
	var err error
	var outBuffer bytes.Buffer

	err = blogCatTemp.Execute(&outBuffer, blist)
	CheckErrContext(err, "Error in Template ")

	// Write out Frame
	frameData := &SubPage{
		Title:     "Blog - " + string(cat),
		FullURL:   "/blog/cat/" + cat.UrlVer() + "/",
		ShortDesc: "Posts by Claire Blackshaw tagged " + string(cat) + ".",
		Content:   template.HTML(outBuffer.String()),
	}

	err = os.MkdirAll(publicHtmlRoot+"blog/cat/"+cat.UrlVer(), 0777)
	CheckErrContext(err, "Error in Mkdir ")

	f, fileErr := os.Create(publicHtmlRoot + "blog/cat/" + cat.UrlVer() + "/index.html")
	CheckErrContext(fileErr, "Error in File ")

	err = RootTemp.Execute(f, frameData)
	CheckErr(err)

	f.Close()
}
