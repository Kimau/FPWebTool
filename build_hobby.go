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
}

var (
	hobbyIndexTemp *template.Template
)

func init() {
	var err error

	hobbyIndexTemp, err = template.ParseFiles("Templates/hobby.html")
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
		FullURL: "http://www.claire-blackshaw.com/hobby/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(publicHtmlRoot + "hobby/index.html")
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

	myData.Hobby.LoadFromFile()
	myData.Hobby.GeneratePage()
}
