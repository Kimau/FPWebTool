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

type AboutMe struct {
	Feed      BlogList
	Hobby     HobbyList
	Job       JobList
	ShortFeed BlogList
	GameList  GameList
	Platforms []string
}

type TemplateRoot struct {
	SubData interface{}
}

var (
	myData   *AboutMe
	RootTemp *template.Template
)

func init() {

	myData = &AboutMe{
		Feed:  BlogList{},
		Hobby: HobbyList{},
		Job:   JobList{},
	}

	myData.Feed.LoadFromFile()
	myData.Hobby.LoadFromFile()
	myData.Hobby.LoadFromFile()

	var err error
	RootTemp, err = template.ParseFiles("Templates/root.html")
	if err != nil {
		log.Fatalln(err)
		return
	}
}

func loadJSONBlob(filename string, jObj interface{}) {
	log.Println("Loading ", filename)
	jsonBlob, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
		return
	}

	err = json.Unmarshal(jsonBlob, jObj)
	if err != nil {
		log.Fatalln("Error in JSON ", err)
	}
}

func saveJSONBlob(filename string, jObj interface{}) {
	log.Println("Saving ", filename)
	b, err := json.MarshalIndent(jObj, "", "  ")
	if err != nil {
		log.Fatalln("Error in JSON ", err)
	}

	os.Remove(filename)
	err = ioutil.WriteFile(filename, b, 0777)
	if err != nil {
		log.Fatalln(err)
		return
	}
}

////////////////////////////////////////////////////////////////////////////////
// Generate About
func GenerateAbout() {
	var err error

	os.RemoveAll(publicHtmlRoot + "index.html")
	aboutIndexTemp, err := template.ParseFiles("Templates/about.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Build Short Feed
	myData.ShortFeed = myData.Feed[0:10]

	// Build Game List
	myData.GameList = BuildFromJobs(&myData.Job)

	// Build Platform List

	platformMap := make(map[string]int)
	for _, g := range myData.GameList {
		for _, p := range g.Platform {
			platformMap[p] = platformMap[p]
		}
	}

	myData.Platforms = make([]string, len(platformMap))

	i := 0
	for k := range platformMap {
		myData.Platforms[i] = k
		i += 1
	}

	// Run Template

	var outBuffer bytes.Buffer
	aboutIndexTemp.Execute(&outBuffer, myData)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Claire Blackshaw",
		FullURL: "http://www.claire-blackshaw.com/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(publicHtmlRoot + "index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	err = RootTemp.Execute(f, frameData)
	if err != nil {
		log.Fatalln(err)
		return
	}

	f.Close()
}
