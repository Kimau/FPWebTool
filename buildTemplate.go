package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// AbsURL turns a site-relative path into a fully-qualified URL on the canonical
// origin. Anything already absolute is passed through untouched. Required for
// canonical tags, og:url and sitemap <loc>, all of which reject relative paths.
func AbsURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return siteBaseURL + path
}

type SubPage struct {
	Title     string        `json:"title"`
	Content   template.HTML `json:"content"`
	ShortDesc string
	FullURL   string
	Twitter   *TwitterCard
}

// Canonical is the absolute URL for this page, used by <link rel="canonical">
// and og:url. Both are ignored by crawlers if given a relative path.
func (sp *SubPage) Canonical() string {
	return AbsURL(sp.FullURL)
}

// AbsImage is the absolute URL of the card image. Twitter and Facebook both
// silently drop relative image paths.
func (tc *TwitterCard) AbsImage() string {
	return AbsURL(tc.Image)
}

type WebLink struct {
	Title string `json:"name"`
	Link  string `json:"url"`
}

type TwitterCard struct {
	Card        string
	Site        string
	Title       string
	Description string
	Image       string
}

type GenerateData struct { // Loaded from files and Generated
	Gallery      GalleryList
	Micro        MicroList
	Feed         BlogList
	Hobby        HobbyList
	Job          JobList
	ShortFeed    BlogList
	ShortMicro   BlogList
	ShortGallery GalleryList
	GameList     GameList
	Platforms    []string
	Categories   []BlogCat // categories that survived the single-use filter
}

type TemplateRoot struct {
	SubData interface{}
}

var (
	genData  *GenerateData
	RootTemp *template.Template
)

func loadJSONBlob(filename string, jObj interface{}) {
	log.Println("Loading ", filename)
	jsonBlob, err := os.ReadFile(filename)
	CheckErr(err)

	err = json.Unmarshal(jsonBlob, jObj)
	CheckErrContext(err, "Error in JSON ", filename, " - ")
}

func saveJSONBlob(filename string, jObj interface{}) {
	log.Println("Saving ", filename)
	b, err := json.MarshalIndent(jObj, "", "  ")
	CheckErrContext(err, "Error in JSON ", filename, " - ")

	os.Remove(filename)
	err = ioutil.WriteFile(filename, b, 0777)
	CheckErr(err)
}

// //////////////////////////////////////////////////////////////////////////////
// Generate About
func GenerateAbout() {
	os.RemoveAll(publicHtmlRoot + "index.html")
	aboutIndexTemp, err := template.ParseFiles("Templates/about.html")
	CheckErr(err)

	// Run Template

	var outBuffer bytes.Buffer
	err = aboutIndexTemp.Execute(&outBuffer, genData)
	CheckErrContext(err, "Error in Template ")

	// Write out Frame
	frameData := &SubPage{
		Title:   "Claire Blackshaw",
		FullURL: "/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(publicHtmlRoot + "index.html")
	CheckErrContext(fileErr, "Error in File ")

	err = RootTemp.Execute(f, frameData)
	CheckErr(err)

	f.Close()
}

// //////////////////////////////////////////////////////////////////////////////
// Generate Data
func generateDataOnly() {
	genData = &GenerateData{
		Feed:  BlogList{},
		Hobby: HobbyList{},
		Job:   JobList{},
	}

	log.Println("Do Jobs...")
	genData.Job.LoadFromFile()
	log.Println("Do Feed...")
	genData.Feed.LoadFromFile()

	// Date is derived from Pubdate, not stored. Without this the no-flag startup
	// path regenerates rss.xml with every post on the zero date, so the feed
	// comes out in arbitrary order.
	for _, v := range genData.Feed {
		v.FixupDateFromPubStr()
	}
	log.Println("Do Hobby...")
	genData.Hobby.LoadFromFile()
	LoadFromMicroListFolder()
	LoadFromGalleryListFolder()

	// Build Game List
	genData.GameList = BuildFromJobs(&genData.Job)

	// Build Platform List
	platformMap := make(map[string]int)
	genData.Platforms = []string{}
	for _, g := range genData.GameList {
		for _, p := range g.Platform {
			_, ok := platformMap[p]
			if !ok {
				platformMap[p] = 1
				genData.Platforms = append(genData.Platforms, p)
			}
		}
	}
}

// buildShortLists fills the front-page lists: the newest post rendered in full,
// then the next few as summaries. Must run after genData.Feed is sorted, and
// takes copies rather than sub-slices so a later re-sort of Feed can't silently
// change what the front page shows.
func buildShortLists() {
	newest := min(len(genData.Feed), 1)
	genData.ShortMicro = append(BlogList{}, genData.Feed[:newest]...)

	rest := min(len(genData.Feed), 4)
	genData.ShortFeed = append(BlogList{}, genData.Feed[newest:rest]...)
}

func setupRoot() {
	var err error
	RootTemp, err = template.ParseFiles("Templates/root.html")
	CheckErr(err)
}

func genWebsite() {
	generateDataOnly()

	setupRoot()

	log.Println("Generating Gallery")
	GenerateGallery()

	log.Println("Generating Micro")
	GenerateMicro()

	log.Println("Generating Blog ")
	GenerateBlog()

	log.Println("Generating Hobby ")
	GenerateHobby()

	log.Println("Generating Job ")
	GenerateJob()

	log.Println("Generating About ")
	GenerateAbout()

	log.Println("Generating Feed ")
	GenerateFeed()

	log.Println("Generating Sitemap ")
	GenerateSiteMap()
}
