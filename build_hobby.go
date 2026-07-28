package main

import (
	"bytes"
	"html/template"
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

// //////////////////////////////////////////////////////////////////////////////
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
	CheckErr(err)

	// Written to /projects/, so FullURL has to say /projects/ - it feeds the
	// canonical tag and the og:url.
	WritePage(&SubPage{
		Title:     "Experiments",
		FullURL:   "/projects/",
		ShortDesc: "Hobby projects, experiments and game jam entries by Claire Blackshaw.",
		Content:   template.HTML(outBuffer.String()),
	}, publicHtmlRoot+"projects/index.html")
}

// //////////////////////////////////////////////////////////////////////////////
// Generate Hobby
func GenerateHobby() {
	// The page lives at /projects/; the old empty /hobby/ directory was never
	// written to and just shipped an empty folder to the bucket.
	os.RemoveAll(publicHtmlRoot + "projects/")
	_ = os.MkdirAll(publicHtmlRoot+"projects/", 0777)

	genData.Hobby.GeneratePage()
}
