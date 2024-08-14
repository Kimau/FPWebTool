package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"os"
)

type SubPage struct {
	Title     string        `json:"title"`
	Content   template.HTML `json:"content"`
	ShortDesc string
	FullURL   string
	Twitter   *TwitterCard
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
	log.Println("Do Hobby...")
	genData.Hobby.LoadFromFile()
	LoadFromMicroListFolder()
	LoadFromGalleryListFolder()

	// Build Short Feed
	genData.ShortFeed = genData.Feed[1:4]
	genData.ShortMicro = genData.Feed[:1]

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
