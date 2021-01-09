package main

import (
	"bytes"
	"html/template"
	"log"
	"os"
)

type HobbyProject struct {
	Title    string    `json:"title"`
	Tooltip  string    `json:"tooltip"`
	Tools    string    `json:"tools"`
	Links    []WebLink `json:"link"`
	BodyList []string  `json:"desc"`
	Images   []string  `json:"images"`
	Tags     []string  `json:"tags"`
	Active   bool      `json:"active"`
	Recent   bool      `json:"recent"`
}

var (
	hobbyIndexTemp *template.Template
)

func init() {
	var err error

	hobbyIndexTemp, err = template.ParseFiles("Templates/projects.html")
	if err != nil {
		log.Fatalln(err)
		return
	}
}

////////////////////////////////////////////////////////////////////////////////
// HobbyList
type HobbyList []*HobbyProject

func (hl *HobbyList) LoadFromFile() {
	loadJSONBlob("Data/hobby.js", hl)

	for _, v := range *hl {
		for _, t := range v.Tags {
			if t == "active" {
				v.Active = true
			}
			if t == "recent" {
				v.Recent = true
			}
		}
	}
}

func (hl *HobbyList) GeneratePage() {
	var err error
	var outBuffer bytes.Buffer

	err = hobbyIndexTemp.Execute(&outBuffer, hl)
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Write out Frame
	frameData := &SubPage{
		Title:   "Hobby",
		FullURL: "/hobby/",
		Content: template.HTML(outBuffer.String()),
	}

	os.MkdirAll(publicHtmlRoot+"projects/", 0777)

	f, fileErr := os.Create(publicHtmlRoot + "projects/index.html")
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

////////////////////////////////////////////////////////////////////////////////
// Generate Hobby
func GenerateHobby() {
	os.RemoveAll(publicHtmlRoot + "hobby/")
	_ = os.MkdirAll(publicHtmlRoot+"hobby/", 0777)

	genData.Hobby.GeneratePage()
}
