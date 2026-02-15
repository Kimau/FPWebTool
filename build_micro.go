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
}

const microPostVersion = 2

var regFindImage = regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
var	reNonAlnum = regexp.MustCompile(`[^a-z0-9 ]+`)
var	reSpaces = regexp.MustCompile(`\s+`)

// //////////////////////////////////////////////////////////////////////////////
// Blog Listing
type MicroList []*MicroPost

func (bl MicroList) Len() int           { return len(bl) }
func (bl MicroList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl MicroList) Less(i, j int) bool { return bl[i].Date.After(bl[j].Date) }

// enrichMicroPost extracts metadata from the body content.
// Returns true if the post was modified.
func enrichMicroPost(post *MicroPost) bool {
	changed := false
	braw := string(post.Body)

	// Extract first image for social card
	if post.BannerImage == "" {
		if loc := regFindImage.FindStringSubmatch(braw); loc != nil {
			post.BannerImage = cleanImagePath(loc[1])
			changed = true
		}
	}

	// Build short description from plain text body
	if post.ShortDesc == "" && len(braw) > 0 {
		p := bluemonday.StripTagsPolicy()
		plain := p.Sanitize(braw)
		plain = strings.ReplaceAll(plain, "\n", " ")
		plain = strings.TrimSpace(plain)
		if len(plain) > 400 {
			plain = plain[0:400]
			// Walk back to word boundary
			r, size := utf8.DecodeLastRuneInString(plain)
			for !unicode.IsSpace(r) {
				if r == utf8.RuneError && (size == 0 || size == 1) {
					break
				}
				plain = plain[:len(plain)-size]
				r, size = utf8.DecodeLastRuneInString(plain)
			}
		}
		post.ShortDesc = html.UnescapeString(strings.TrimSpace(plain))
		changed = true
	}

	// Extract header as title if current title looks like a filename
	if post.Title != "" {
		hre := regexp.MustCompile(`<h[1-6]>([^<]*)</h[1-6]>`)
		if loc := hre.FindStringSubmatchIndex(braw); loc != nil {
			extracted := strings.Trim(braw[loc[2]:loc[3]], " .\n")
			if extracted != "" {
				post.Title = strings.ToUpper(extracted[0:1]) + extracted[1:]
				changed = true
			}
		}
	}

	return changed
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

	// Load existing JSON if present
	if _, statErr := os.Stat(jsonPath); statErr == nil {
		loadJSONBlob(jsonPath, &newPost)
		hasExistingJSON = true
	} else {
		newPost.Title = title
		newPost.Date = info.ModTime()
	}

	// Always recompute derived fields
	newPost.Pubdate = newPost.Date.Format(longformPubStr)
	newPost.DateStr = fmt.Sprintf("%d %v %d", newPost.Date.Day(), newPost.Date.Month(), newPost.Date.Year())

	// Enrich if new or outdated version
	needsSave := !hasExistingJSON
	if newPost.Version < microPostVersion {
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


	for _, v := range genData.Micro {
		// Use persisted key if available, otherwise generate one
		k := v.Key

		// Extract Header if there is one
		hre := regexp.MustCompile("<h[0-9]>([^<]*)</h[0-9]>")
		braw := string(v.Body)
		loc := hre.FindStringSubmatchIndex(braw)
		if loc != nil {
			v.Title = braw[loc[2]:loc[3]]
			braw = braw[0:loc[0]] + braw[loc[1]:]
		}
		v.Title = strings.Trim(v.Title, " .\n")
		v.Title = strings.ToUpper(v.Title[0:1]) + v.Title[1:]

		// In LoadFromMicroListFolder, inside the merge loop:
		blogFromMicro := BlogPost{
			Key:         k,
			Title:       v.Title,
			Date:        v.Date,
			Body:        template.HTML(braw),
			ShortDesc:   v.ShortDesc,   // already computed
			BannerImage: v.BannerImage, // already computed
			SmallImage:  v.SmallImage,  // already computed
		}

		// strip html from body
		plainBody := braw
		p := bluemonday.StripTagsPolicy()
		plainBody = p.Sanitize(plainBody)
		plainBody = strings.ReplaceAll(plainBody, "\n", "")
		if len(plainBody) > 400 {
			plainBody = plainBody[0:400]

			r, size := utf8.DecodeLastRuneInString(plainBody)
			for !unicode.IsSpace(r) {
				if r == utf8.RuneError && (size == 0 || size == 1) {
					size = 0
				}

				plainBody = plainBody[:len(plainBody)-size]
				r, size = utf8.DecodeLastRuneInString(plainBody)
			}
		}

		if blogFromMicro.ShortDesc == "" {
			blogFromMicro.ShortDesc = html.UnescapeString(plainBody)
			blogFromMicro.ShortDesc = strings.ReplaceAll(blogFromMicro.ShortDesc, ".", ". ")
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

	// Write out Frame
	frameData := &SubPage{
		Title:   "Micro Posts",
		FullURL: "/micro/",
		Content: template.HTML(outBuffer.String()),
	}

	err = os.MkdirAll(publicHtmlRoot+"micro", 0777)
	CheckErr(err)

	var outFile *os.File
	outFile, err = os.Create(publicHtmlRoot + "micro/index.html")
	CheckErrContext(err, "Error in File ")

	err = RootTemp.Execute(outFile, frameData)
	CheckErrContext(err, "Error in Template ")

	outFile.Close()
}
