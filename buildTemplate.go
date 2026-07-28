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

type WebLink struct {
	Title string `json:"name"`
	Link  string `json:"url"`
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

	WritePage(&SubPage{
		Title:   "Claire Blackshaw",
		FullURL: "/",
		Content: template.HTML(outBuffer.String()),
	}, publicHtmlRoot+"index.html")
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

	// Share images and descriptions are resolved here, before any generator runs,
	// so that every consumer sees the same answer. Note this is the one part of
	// loading that can touch the network, to fetch YouTube thumbnails it has not
	// cached yet; it degrades to the site default rather than failing.
	resolveAllSocial()

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

// setupTemplates parses every template that outlives a single generator call.
// These used to be parsed in package init(), which meant importing the package at
// all required the process to be sitting in the repo root with Templates/ present
// - so the package could not be tested, and a missing template killed the process
// before main() got a chance to say anything useful. Both callers of this (the
// -gen path and the plain startup path) run it before any generator.
func setupTemplates() {
	var err error

	RootTemp, err = template.ParseFiles("Templates/root.html")
	CheckErr(err)

	blogIndexTemp, err = template.ParseFiles("Templates/blogindex.html")
	CheckErr(err)

	blogTemp, err = template.ParseFiles("Templates/blogpost.html")
	CheckErr(err)

	blogCatTemp, err = template.ParseFiles("Templates/blogcat.html")
	CheckErr(err)

	galleryTemp, err = template.ParseFiles("Templates/gallery.html")
	CheckErr(err)

	galSingleTemp, err = template.ParseFiles("Templates/galsingle.html")
	CheckErr(err)

	hobbyIndexTemp, err = template.ParseFiles("Templates/projects.html")
	CheckErr(err)
}

func genWebsite() {
	setupTemplates()

	generateDataOnly()

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
