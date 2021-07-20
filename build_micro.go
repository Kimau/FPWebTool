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

type MicroPost struct {
	Title string    `json:"title"`
	Date  time.Time `json:"pubDate"`

	Body    template.HTML `json:"-"`
	DateStr string        `json:"-"`
	Pubdate string        `json:"-"`
}

////////////////////////////////////////////////////////////////////////////////
// Blog Listing
type MicroList []*MicroPost

func (bl MicroList) Len() int           { return len(bl) }
func (bl MicroList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl MicroList) Less(i, j int) bool { return bl[i].Date.After(bl[j].Date) }

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

	// See if there is a meta data
	if _, err := os.Stat(path + ".json"); os.IsNotExist(err) {
		newPost.Title = title
		newPost.Date = info.ModTime()
	} else {
		loadJSONBlob(path+".json", &newPost)
	}

	newPost.Pubdate = newPost.Date.Format(longformPubStr)
	newPost.DateStr = fmt.Sprintf("%d %v %d", newPost.Date.Day(), newPost.Date.Month(), newPost.Date.Year())
	genData.Micro = append(genData.Micro, &newPost)

	saveJSONBlob(path+".json", &newPost)
	return nil
}

func LoadFromMicroListFolder() {
	err := filepath.Walk("./microdata", LoadSingleFile)
	if err != nil {
		log.Println(err)
	}

	// merge microdata into blog feed
	re := regexp.MustCompile("/[^a-z0-9]/")

	for _, v := range genData.Micro {
		k := strings.ToLower(v.Title)
		k = re.ReplaceAllString(k, "")
		k = strings.ReplaceAll(k, "/[^a-z0-9]/g", "")

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

		// Convert to Blog
		blogFromMicro := BlogPost{
			Key:   k,
			Title: v.Title,
			Date:  v.Date,
			Body:  template.HTML(braw),
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
		blogFromMicro.ShortDesc = html.UnescapeString(plainBody)
		blogFromMicro.ShortDesc = strings.ReplaceAll(blogFromMicro.ShortDesc, ".", ". ")

		blogFromMicro.RawCategory = []BlogCat{"micro"}
		blogFromMicro.Category = []BlogCat{"micro"}
		blogFromMicro.SetNewPubDate(v.Date)
		blogFromMicro.IsMicro = true
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
