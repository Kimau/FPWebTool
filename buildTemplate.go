package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"time"
)

type SubPage struct {
	Title   string        `json:"title"`
	Content template.HTML `json:"content"`
	Twitter *TwitterCard
}

type BlogCat string

type BlogPost struct {
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Pubdate     string    `json:"pubDate"`
	SmallImage  string    `json:"smlImage,omitempty"`
	BannerImage string    `json:"bannerImage,omitempty"`
	ShortDesc   string    `json:"desc,omitempty"`
	RawCategory []BlogCat `json:"category"`

	Category []BlogCat     `json:"-"`
	Date     time.Time     `json:"-"`
	Body     template.HTML `json:"-"`
	DateStr  string        `json:"-"`
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

type BlogList []*BlogPost
type HobbyList []*HobbyProject
type JobList []*JobObject
type GameList []*GameProject

var (
	regUrlChar     *regexp.Regexp
	regUrlSpace    *regexp.Regexp
	regStripMarkup *regexp.Regexp
	myData         *AboutMe
	RootTemp       *template.Template
)

func (f BlogList) Len() int           { return len(f) }
func (f BlogList) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f BlogList) Less(i, j int) bool { return f[i].Date.After(f[j].Date) }

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

	myData = &AboutMe{
		Feed:  BlogList{},
		Hobby: HobbyList{},
		Job:   JobList{},
	}

	var e error
	RootTemp, e = template.ParseFiles("Templates/root.html")
	if e != nil {
		log.Fatalln(e)
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

	f, err := os.Create("sitemap.xml")
	if err != nil {
		log.Fatalln("Error in Sitemap ", err)
	}

	f.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
	<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	`)
	for _, v := range siteLinks {
		s, e := xml.MarshalIndent(v, "  ", "  ")
		if e != nil {
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
	jsonBlob, e := ioutil.ReadFile(filename)
	if e != nil {
		log.Fatalln(e)
		return
	}

	e = json.Unmarshal(jsonBlob, jObj)
	if e != nil {
		log.Fatalln("Error in JSON ", e)
	}
}

func saveJSONBlob(filename string, jObj interface{}) {
	log.Println("Saving ", filename)
	b, e := json.MarshalIndent(jObj, "", "  ")
	if e != nil {
		log.Fatalln("Error in JSON ", e)
	}

	os.Remove(filename)
	e = ioutil.WriteFile(filename, b, 0777)
	if e != nil {
		log.Fatalln(e)
		return
	}

}

func GenerateJob() {
	os.RemoveAll("./job/")
	_ = os.MkdirAll("./job/", 0777)

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
	jobIndexTemp, e := template.ParseFiles("Templates/job.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	var outBuffer bytes.Buffer
	jobIndexTemp.Execute(&outBuffer, myData.Job)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Games Career",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./job/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	RootTemp.Execute(f, frameData)
	f.Close()
}

func GenerateHobby() {
	os.RemoveAll("./hobby/")
	_ = os.MkdirAll("./hobby/", 0777)

	loadJSONBlob("Data/hobby.js", &myData.Hobby)

	genHobbyPage()
}

func genHobbyPage() {
	hobbyIndexTemp, e := template.ParseFiles("Templates/hobby.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	var outBuffer bytes.Buffer
	e = hobbyIndexTemp.Execute(&outBuffer, myData.Hobby)
	if e != nil {
		log.Fatalln(e)
		return
	}

	// Write out Frame
	frameData := &SubPage{
		Title:   "Hobby",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./hobby/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	e = RootTemp.Execute(f, frameData)
	if e != nil {
		log.Fatalln(e)
		return
	}

	f.Close()
}

//////////////////////////////////
func GenerateBlog() {
	var e error

	os.RemoveAll("./blog/")
	e = os.MkdirAll("./blog/", 0777)

	loadJSONBlob("blogdata/blogData.js", &myData.Feed)

	blogTemp, e := template.ParseFiles("Templates/blogpost.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	blogCatTemp, e := template.ParseFiles("Templates/blogcat.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	// Gather Catergories and filter out single use catergories
	var catMap map[BlogCat]BlogList
	catMap = make(map[BlogCat]BlogList)
	for _, v := range myData.Feed {
		for _, c := range v.RawCategory {
			catMap[c] = append(catMap[c], v)
		}
	}

	removedCat := []BlogCat{}
	for _, v := range myData.Feed {
		v.Category = []BlogCat{}
		for _, c := range v.RawCategory {
			if len(catMap[c]) < 2 {
				delete(catMap, c)
				removedCat = append(removedCat, "-"+c)
			} else {
				v.Category = append(v.Category, c)
			}
		}
		genBlogPage(v, blogTemp)

	}
	log.Println("Removed ", removedCat)

	sort.Sort(myData.Feed)
	genBlogIndexPage()

	for k, v := range catMap {
		genBlogCatergoryPage(k, v, blogCatTemp)
	}

	// Save Out
	saveJSONBlob("blogdata/blogData2.js", &myData.Feed)
}

func genBlogIndexPage() {
	blogIndexTemp, e := template.ParseFiles("Templates/blogindex.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	var outBuffer bytes.Buffer
	blogIndexTemp.Execute(&outBuffer, myData.Feed)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./blog/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	e = RootTemp.Execute(f, frameData)
	if e != nil {
		log.Fatalln(e)
		return
	}

	f.Close()
}

func genBlogCatergoryPage(cat BlogCat, blist BlogList, blogCat *template.Template) {
	var e error
	var outBuffer bytes.Buffer
	blogCat.Execute(&outBuffer, blist)

	// Write out Frame
	frameData := &SubPage{
		Title:   string(cat),
		Content: template.HTML(outBuffer.String()),
	}

	err := os.MkdirAll("./blog/cat/"+cat.UrlVer(), 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	f, fileErr := os.Create("./blog/cat/" + cat.UrlVer() + "/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	e = RootTemp.Execute(f, frameData)
	if e != nil {
		log.Fatalln(e)
		return
	}

	f.Close()
}

func genBlogPage(v *BlogPost, blogTemp *template.Template) {
	var e error

	srcFile := fmt.Sprintf("blogdata/post/%s.html", v.Key)
	bodyBytes, err := ioutil.ReadFile(srcFile)
	v.Body = template.HTML(string(bodyBytes))

	const longform = "Mon, 02 Jan 2006 15:04:05 -0700"
	v.Date, err = time.Parse(longform, v.Pubdate)
	if err != nil {
		fmt.Println(err)
	}

	v.DateStr = fmt.Sprintf("%d %v %d", v.Date.Day(), v.Date.Month(), v.Date.Year())

	fileLoc := fmt.Sprintf("./blog/%04d/%02d/%s/", v.Date.Year(), v.Date.Month(), v.Key)

	log.Println(fileLoc)

	err = os.MkdirAll(fileLoc, 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	var outBuffer bytes.Buffer
	blogTemp.Execute(&outBuffer, v)

	// Twitter Card
	if len(v.ShortDesc) < 4 {
		// Build Desc
		sum := regStripMarkup.ReplaceAllString(string(v.Body), " ")
		if len(sum) > 200 {
			sum = sum[0:200]
		}

		v.ShortDesc = sum
	}

	tc := &TwitterCard{
		Card:        "summary",
		Site:        "@EvilKimau",
		Title:       v.Title,
		Description: v.ShortDesc,
		Image:       "/images/fp_twitter_tiny.png",
	}

	if len(v.BannerImage) > 3 {
		tc.Card = "summary_large_image"
		tc.Image = v.BannerImage
	} else if len(v.SmallImage) > 3 {
		tc.Image = v.SmallImage
	}

	// Write out Frame
	frameData := &SubPage{
		Title:   v.Title,
		Content: template.HTML(outBuffer.String()),
		Twitter: tc,
	}

	f, fileErr := os.Create(fileLoc + "/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	e = RootTemp.Execute(f, frameData)
	if e != nil {
		log.Fatalln(e)
		return
	}

	f.Close()
}

func subSlice(source []interface{}, limit int) []interface{} {
	return source[0:limit]
}

func GenerateAbout() {
	var e error

	os.RemoveAll("./index.html")
	aboutIndexTemp, e := template.ParseFiles("Templates/about.html")
	if e != nil {
		log.Fatalln(e)
		return
	}

	myData.ShortFeed = myData.Feed[0:10]

	var outBuffer bytes.Buffer
	aboutIndexTemp.Execute(&outBuffer, myData)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Claire Blackshaw",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	e = RootTemp.Execute(f, frameData)
	if e != nil {
		log.Fatalln(e)
		return
	}

	f.Close()
}
