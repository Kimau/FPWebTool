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
	Micro      MicroList
	Feed       BlogList
	Hobby      HobbyList
	Job        JobList
	ShortFeed  BlogList
	ShortMicro BlogList
	GameList   GameList
	Platforms  []string
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
	jsonBlob, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
		return
	}

	err = json.Unmarshal(jsonBlob, jObj)
	if err != nil {
		log.Fatalln("Error in JSON ", filename, " - ", err)
	}
}

func saveJSONBlob(filename string, jObj interface{}) {
	log.Println("Saving ", filename)
	b, err := json.MarshalIndent(jObj, "", "  ")
	if err != nil {
		log.Fatalln("Error in JSON ", filename, " - ", err)
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
	os.RemoveAll(publicHtmlRoot + "index.html")
	aboutIndexTemp, err := template.ParseFiles("Templates/about.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Run Template

	var outBuffer bytes.Buffer
	err = aboutIndexTemp.Execute(&outBuffer, genData)
	if err != nil {
		log.Fatalln("Error in Template ", err)
	}

	// Write out Frame
	frameData := &SubPage{
		Title:   "Claire Blackshaw",
		FullURL: "/",
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

func generateDataOnly() {
	genData = &GenerateData{
		Feed:  BlogList{},
		Hobby: HobbyList{},
		Job:   JobList{},
	}

	log.Println("Do Jobs...");
	genData.Job.LoadFromFile()
	log.Println("Do Feed...");
	genData.Feed.LoadFromFile()
	log.Println("Do Hobby...");
	genData.Hobby.LoadFromFile()
	LoadFromMicroListFolder()

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
	if err != nil {
		log.Fatalln(err)
		return
	}
}

func genWebsite() {
	generateDataOnly()

	setupRoot()

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

	log.Println("Generating Sitemap ")
	GenerateSiteMap()
}
