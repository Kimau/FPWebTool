package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"time"
)

type SubPage struct {
	Title   string        `json:"title"`
	Content template.HTML `json:"content"`
}

type BlogCat string

type BlogPost struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Link     string `json:"link"`
	Pubdate  string `json:"pubDate"`
	Date     time.Time
	Category []BlogCat     `json:"category"`
	Body     template.HTML `post Body`
	DateStr  string        `friendly date string`
}

type SiteMapLink struct {
	XMLName    xml.Name  `xml:"url"`
	Loc        string    `xml:"loc"`
	LastMod    time.Time `xml:"lastmod"`
	Changefreq string    `xml:"changefreq"` //always hourly daily weekly monthly yearly never
	Priority   float64   `xml:"priority"`
}

type WebLink struct {
	Title string
	Link  string
}

type HobbyProject struct {
	Title    string    `json:"link"`
	Tooltip  string    `json:"link"`
	Tools    string    `json:"tools"`
	Links    []WebLink `json:"link"`
	BodyList []string  `json:"desc"`
	Images   []string  `json:"images"`
	Tags     []string  `json:"tags"`
}

type JobObject struct {
	Company string `json:"company"`
	Role    string `json:"role"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Body    string `json:"body"`
	Games   []GameProject
}

type GameProject struct {
	Title     string   `json:"title"`
	Platform  string   `json:"platform"`
	Developer string   `json:"developer"`
	Job       string   `json:"job"`
	Publisher string   `json:"publisher"`
	Released  string   `json:"released"`
	Position  string   `json:"position"`
	Website   string   `json:"website"`
	Youtube   string   `json:"youtube"`
	Images    []string `json:"images"`
	Body      []string `json:"body"`
}

type TemplateRoot struct {
	SubData interface{}
}

type BlogList []*BlogPost
type HobbyList []*HobbyProject

var (
	feed        BlogList
	hobby       HobbyList
	regUrlChar  *regexp.Regexp
	regUrlSpace *regexp.Regexp
)

func (f BlogList) Len() int           { return len(f) }
func (f BlogList) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f BlogList) Less(i, j int) bool { return f[i].Date.After(f[j].Date) }

func (c BlogCat) UrlVer() string {
	return regUrlSpace.ReplaceAllString(regUrlChar.ReplaceAllString(string(c), ""), "_")
}

func init() {
	regUrlChar = regexp.MustCompile("[^A-Za-z]")
	regUrlSpace = regexp.MustCompile(" ")
}

func GenerateSiteMap() {
	var siteLinks []SiteMapLink

	siteLinks = append(siteLinks, SiteMapLink{
		Loc:        "http://www.claire-blackshaw.com/",
		LastMod:    time.Now(),
		Changefreq: "daily",
		Priority:   1.0,
	})

	siteLinks = append(siteLinks, SiteMapLink{
		Loc:        "http://www.claire-blackshaw.com/blog/",
		LastMod:    time.Now(),
		Changefreq: "daily",
		Priority:   1.0,
	})

	for _, v := range feed {
		siteLinks = append(siteLinks, SiteMapLink{
			Loc:        "http://www.claire-blackshaw.com" + v.Link,
			LastMod:    v.Date,
			Changefreq: "monthly",
			Priority:   0.5,
		})
	}

	f, err := os.Create("sitemap.xml")
	if err != nil {
		log.Fatalln("Error in Sitemap ", err)
	}

	f.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
	<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	`)
	for _, v := range siteLinks {
		s, e := xml.MarshalIndent(v, "  ", "  ")
		if e != nil {
			log.Fatalln("Problem writing link ", err)
		}
		f.Write(s)
	}
	f.WriteString(`
	</urlset>`)

	f.Close()
}

func LoadJSONBlob(filename string, jObj interface{}) {

	jsonBlob, e := ioutil.ReadFile(filename)
	if e != nil {
		log.Fatalln(e)
		return
	}

	e = json.Unmarshal(jsonBlob, jObj)
	if e != nil {
		log.Fatalln("Error in JSON ", e)
	}
}

func GenerateHobby() {
	var e error
	os.RemoveAll("./hobby/")
	e = os.MkdirAll("./hobby/", 0777)

	LoadJSONBlob("data/hobby.js", &hobby)

	rootTemp, e := template.ParseFiles("Templates/root.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	GenerateHobbyPage(rootTemp)
}

func GenerateHobbyPage(rootTemp *template.Template) {
	hobbyIndexTemp, e := template.ParseFiles("Templates/hobby.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	var outBuffer bytes.Buffer
	hobbyIndexTemp.Execute(&outBuffer, hobby)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Hobby",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./hobby/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	rootTemp.Execute(f, frameData)
	f.Close()
}

func GenerateBlog() {
	var e error

	os.RemoveAll("./blog/")
	e = os.MkdirAll("./blog/", 0777)

	LoadJSONBlob("blogdata/blogData.js", &feed)

	rootTemp, e := template.ParseFiles("Templates/root.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	blogTemp, e := template.ParseFiles("Templates/blogpost.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	blogCatTemp, e := template.ParseFiles("Templates/blogcat.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	// Gather Catergories and filter out single use catergories
	var catMap map[BlogCat]BlogList
	catMap = make(map[BlogCat]BlogList)
	for _, v := range feed {
		GenerateBlogPage(v, blogTemp, rootTemp)

		for _, c := range v.Category {
			catMap[c] = append(catMap[c], v)
		}
	}

	removedCat := []BlogCat{}
	for _, v := range feed {
		newCat := []BlogCat{}
		for _, c := range v.Category {
			if len(catMap[c]) < 2 {
				delete(catMap, c)
				removedCat = append(removedCat, "-"+c)
			} else {
				newCat = append(newCat, c)
			}
		}
		v.Category = newCat
		GenerateBlogPage(v, blogTemp, rootTemp)

	}
	log.Println("Removed ", removedCat)

	sort.Sort(feed)
	GenerateBlogIndexPage(rootTemp)

	for k, v := range catMap {
		GenerateBlogCatergoryPage(k, v, rootTemp, blogCatTemp)
	}
}

func GenerateBlogIndexPage(rootTemp *template.Template) {
	blogIndexTemp, e := template.ParseFiles("Templates/blogindex.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	var outBuffer bytes.Buffer
	blogIndexTemp.Execute(&outBuffer, feed)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./blog/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	rootTemp.Execute(f, frameData)
	f.Close()
}

func GenerateBlogCatergoryPage(cat BlogCat, blist BlogList, rootTemp *template.Template, blogCat *template.Template) {

	var outBuffer bytes.Buffer
	blogCat.Execute(&outBuffer, blist)

	// Write out Frame
	frameData := &SubPage{
		Title:   string(cat),
		Content: template.HTML(outBuffer.String()),
	}

	err := os.MkdirAll("./blog/cat/"+cat.UrlVer(), 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	f, fileErr := os.Create("./blog/cat/" + cat.UrlVer() + "/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	rootTemp.Execute(f, frameData)
	f.Close()
}

func GenerateBlogPage(v *BlogPost, blogTemp *template.Template, rootTemp *template.Template) {

	srcFile := fmt.Sprintf("blogdata/post/%s.html", v.Key)
	bodyBytes, err := ioutil.ReadFile(srcFile)
	v.Body = template.HTML(string(bodyBytes))

	const longform = "Mon, 02 Jan 2006 15:04:05 -0700"
	v.Date, err = time.Parse(longform, v.Pubdate)
	if err != nil {
		fmt.Println(err)
	}

	v.DateStr = fmt.Sprintf("%d %v %d", v.Date.Day(), v.Date.Month(), v.Date.Year())

	fileLoc := fmt.Sprintf("./blog/%04d/%02d/%s/", v.Date.Year(), v.Date.Month(), v.Key)

	log.Println(fileLoc)

	err = os.MkdirAll(fileLoc, 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	var outBuffer bytes.Buffer
	blogTemp.Execute(&outBuffer, v)

	// Write out Frame
	frameData := &SubPage{
		Title:   v.Title,
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(fileLoc + "/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	rootTemp.Execute(f, frameData)
	f.Close()

}
