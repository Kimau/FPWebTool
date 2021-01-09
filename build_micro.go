package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/russross/blackfriday"
)

type MicroPost struct {
	Title string    `json:"title"`
	Date  time.Time `json:"pubDate"`

	Body    template.HTML `json:"-"`
	DateStr string        `json:"-"`
	Pubdate string        `json:"-"`
}

var (
	microTemp *template.Template
)

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
	body := []byte{}
	ext := filepath.Ext(path)
	title = strings.TrimSuffix(title, ext)

	if ext == ".md" {
		body, err = ioutil.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}

		body = MarkdownToHTML(body)

	} else if ext == ".html" {
		body, err = ioutil.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}
	} else if ext == ".json" {
		return nil
	} else {
		fmt.Println("Didn't parse: " + path)
		return nil
	}

	// See if there is a meta data
	var newPost MicroPost

	if _, err := os.Stat(path + ".json"); os.IsNotExist(err) {
		newPost.Title = title
		newPost.Date = info.ModTime()
	} else {
		loadJSONBlob(path+".json", &newPost)
	}

	newPost.Body = template.HTML(body)
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
}

////////////////////////////////////////////////////////////////////////////////
//

func init() {

}

// MarkdownToHTML - Convert Markdown to HTML
func MarkdownToHTML(input []byte) []byte {

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
	return output
}

func GenerateMicro() {
	microTemp, err := template.ParseFiles("Templates/micro.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	sort.Sort(genData.Micro)

	var outBuffer bytes.Buffer
	err = microTemp.Execute(&outBuffer, genData)
	if err != nil {
		log.Fatalln("Error in Template ", err)
	}

	// Write out Frame
	frameData := &SubPage{
		Title:   "Micro Posts",
		FullURL: "/micro/",
		Content: template.HTML(outBuffer.String()),
	}

	err = os.MkdirAll(publicHtmlRoot+"micro", 0777)
	if err != nil {
		log.Fatalln(err)
		return
	}

	var outFile *os.File
	outFile, err = os.Create(publicHtmlRoot + "micro/index.html")
	if err != nil {
		log.Fatalln("Error in File ", err)
	}

	err = RootTemp.Execute(outFile, frameData)
	if err != nil {
		log.Fatalln("Error in Template ", err)
	}

	outFile.Close()
}
