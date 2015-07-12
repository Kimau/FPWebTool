package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"time"
)

type SubPage struct {
	Title     string        `json:"title"`
	Content   template.HTML `json:"content"`
	ShortDesc string
	FullURL   string
	Twitter   *TwitterCard
}

type SiteMapLink struct {
	XMLName    xml.Name  `xml:"url"`
	Loc        string    `xml:"loc"`
	LastMod    time.Time `xml:"lastmod"`
	Changefreq string    `xml:"changefreq"` //always hourly daily weekly monthly yearly never
	Priority   float64   `xml:"priority"`
}

type WebLink struct {
	Title string `json:"name"`
	Link  string `json:"url"`
}

type HobbyProject struct {
	Title    string    `json:"title"`
	Tooltip  string    `json:"tooltip"`
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
	Games   GameList
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

type HobbyList []*HobbyProject
type JobList []*JobObject
type GameList []*GameProject

var (
	regUrlChar     *regexp.Regexp
	regUrlSpace    *regexp.Regexp
	regStripMarkup *regexp.Regexp
	regCatchImage  *regexp.Regexp
	myData         *AboutMe
	RootTemp       *template.Template
)

func (jo JobList) Len() int           { return len(jo) }
func (jo JobList) Swap(i, j int)      { jo[i], jo[j] = jo[j], jo[i] }
func (jo JobList) Less(i, j int) bool { return jo[i].Date.After(jo[j].Date) }

func (gl GameList) Len() int           { return len(gl) }
func (gl GameList) Swap(i, j int)      { gl[i], gl[j] = gl[j], gl[i] }
func (gl GameList) Less(i, j int) bool { return gl[i].Date.After(gl[j].Date) }

func (c BlogCat) UrlVer() string {
	return regUrlSpace.ReplaceAllString(regUrlChar.ReplaceAllString(string(c), ""), "_")
}

func init() {
	regUrlChar = regexp.MustCompile("[^A-Za-z]")
	regUrlSpace = regexp.MustCompile(" ")
	regStripMarkup = regexp.MustCompile("<[^<>]*>")
	regCatchImage = regexp.MustCompile("<img[^>]*\"(/images/Blog[^\"]*)\"[^>]*>")

	myData = &AboutMe{
		Feed:  BlogList{},
		Hobby: HobbyList{},
		Job:   JobList{},
	}

	var err error
	RootTemp, err = template.ParseFiles("Templates/root.html")
	if err != nil {
		log.Fatalln(err)
		return
	}
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

	for _, v := range myData.Feed {
		siteLinks = append(siteLinks, SiteMapLink{
			Loc:        "http://www.claire-blackshaw.com" + v.Link,
			LastMod:    v.Date,
			Changefreq: "monthly",
			Priority:   0.5,
		})
	}

	f, err := os.Create(publicHtmlRoot + "sitemap.xml")
	if err != nil {
		log.Fatalln("Error in Sitemap ", err)
	}

	f.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
	<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	`)
	for _, v := range siteLinks {
		s, err := xml.MarshalIndent(v, "  ", "  ")
		if err != nil {
			log.Fatalln("Problem writing link ", err)
		}
		f.Write(s)
	}
	f.WriteString(`
	</urlset>`)

	f.Close()
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

	os.Remove(publicHtmlRoot + filename)
	err = ioutil.WriteFile(publicHtmlRoot+filename, b, 0777)
	if err != nil {
		log.Fatalln(err)
		return
	}

}

func GenerateJob() {
	os.RemoveAll(publicHtmlRoot + "job/")
	_ = os.MkdirAll(publicHtmlRoot+"job/", 0777)

	loadJSONBlob("Data/job.js", &myData.Job)
	loadJSONBlob("Data/work.js", &myData.GameList)

	platformMap := make(map[string]int)

	for _, j := range myData.Job {
		const joblongform = "2006 January"
		j.Date, _ = time.Parse(joblongform, j.Start)
		for _, g := range myData.GameList {
			if j.Company == g.Job {
				j.Games = append(j.Games, g)

				for _, p := range g.Platform {
					platformMap[p] = platformMap[p] + 1
				}

				const gamelongform = "02 January 2006"
				g.Date = time.Now()
				if len(g.Released) > 1 {
					g.Date, _ = time.Parse(gamelongform, g.Released)
				}
			}
		}
		sort.Sort(j.Games)
	}
	sort.Sort(myData.GameList)
	sort.Sort(myData.Job)

	myData.Platforms = make([]string, len(platformMap))

	i := 0
	for k := range platformMap {
		myData.Platforms[i] = k
		i += 1
	}

	genJobPage()
}

func genJobPage() {
	jobIndexTemp, err := template.ParseFiles("Templates/job.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	var outBuffer bytes.Buffer
	jobIndexTemp.Execute(&outBuffer, myData.Job)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Games Career",
		FullURL: "http://www.flammablepenguins.com/job/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(publicHtmlRoot + "job/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	RootTemp.Execute(f, frameData)
	f.Close()
}

func GenerateHobby() {
	os.RemoveAll(publicHtmlRoot + "hobby/")
	_ = os.MkdirAll(publicHtmlRoot+"hobby/", 0777)

	loadJSONBlob("Data/hobby.js", &myData.Hobby)

	genHobbyPage()
}

func genHobbyPage() {
	hobbyIndexTemp, err := template.ParseFiles("Templates/hobby.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	var outBuffer bytes.Buffer
	err = hobbyIndexTemp.Execute(&outBuffer, myData.Hobby)
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Write out Frame
	frameData := &SubPage{
		Title:   "Hobby",
		FullURL: "http://www.flammablepenguins.com/hobby/",
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

func GenerateAbout() {
	var err error

	os.RemoveAll(publicHtmlRoot + "index.html")
	aboutIndexTemp, err := template.ParseFiles("Templates/about.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	myData.ShortFeed = myData.Feed[0:10]

	var outBuffer bytes.Buffer
	aboutIndexTemp.Execute(&outBuffer, myData)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Claire Blackshaw",
		FullURL: "http://www.flammablepenguins.com/",
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
