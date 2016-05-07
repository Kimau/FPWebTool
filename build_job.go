package main

import (
	"bytes"
	"html/template"
	"log"
	"os"
	"sort"
	"time"
)

type JobObject struct {
	Company string   `json:"company"`
	Role    string   `json:"role"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
	Body    string   `json:"body"`
	Games   GameList `json:"games"`
	Date    time.Time
}

type GameProject struct {
	Title     string   `json:"title"`
	Developer string   `json:"developer"`
	Job       string   `json:"job"`
	Publisher string   `json:"publisher"`
	Released  string   `json:"released"`
	Position  string   `json:"position"`
	Website   string   `json:"website"`
	Youtube   string   `json:"youtube"`
	Platform  []string `json:"platform"`
	Images    []string `json:"images"`
	Body      []string `json:"body"`
	Date      time.Time
}

////////////////////////////////////////////////////////////////////////////////
//
const (
	joblongform  = "2006 January"
	gamelongform = "02 January 2006"
)

var (
	jobIndexTemp *template.Template
)

func init() {
	var err error

	jobIndexTemp, err = template.ParseFiles("Templates/job.html")
	if err != nil {
		log.Fatalln(err)
		return
	}
}

////////////////////////////////////////////////////////////////////////////////
// Job List
type JobList []*JobObject

func (jo JobList) Len() int           { return len(jo) }
func (jo JobList) Swap(i, j int)      { jo[i], jo[j] = jo[j], jo[i] }
func (jo JobList) Less(i, j int) bool { return jo[i].Date.After(jo[j].Date) }

func (jo *JobList) LoadFromFile() {
	loadJSONBlob("Data/job.js", jo)

	for _, j := range myData.Job {
		j.Date, _ = time.Parse(joblongform, j.Start)
		for _, g := range j.Games {
			g.Date, _ = time.Parse(gamelongform, g.Released)
		}

		sort.Sort(j.Games)
	}

	sort.Sort(jo)
}

func (jo *JobList) GeneratePage() {
	var err error

	var outBuffer bytes.Buffer
	jobIndexTemp.Execute(&outBuffer, jo)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Games Career",
		FullURL: "http://www.claire-blackshaw.com/job/",
		Content: template.HTML(outBuffer.String()),
	}

	var outFile *os.File
	outFile, err = os.Create(publicHtmlRoot + "job/index.html")
	if err != nil {
		log.Fatalln("Error in File ", err)
	}

	RootTemp.Execute(outFile, frameData)
	outFile.Close()
}

////////////////////////////////////////////////////////////////////////////////
// Game List
type GameList []*GameProject

func (gl GameList) Len() int           { return len(gl) }
func (gl GameList) Swap(i, j int)      { gl[i], gl[j] = gl[j], gl[i] }
func (gl GameList) Less(i, j int) bool { return gl[i].Date.After(gl[j].Date) }

func BuildFromJobs(jo *JobList) (gl GameList) {
	for _, v := range *jo {
		gl = append(gl, v.Games...)
	}

	return gl
}

////////////////////////////////////////////////////////////////////////////////
// Job Page
func GenerateJob() {
	os.RemoveAll(publicHtmlRoot + "job/")
	_ = os.MkdirAll(publicHtmlRoot+"job/", 0777)

	myData.Job.LoadFromFile()

	myData.Job.GeneratePage()
}
