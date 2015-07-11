package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"
)

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

////////////////////////////////////////////////////////////////////////////////
// Blog Listing
type BlogList []*BlogPost

func (bl BlogList) Len() int           { return len(bl) }
func (bl BlogList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl BlogList) Less(i, j int) bool { return bl[i].Date.After(bl[j].Date) }

func (bl BlogList) Get(key string) *BlogPost {
	for _, v := range bl {
		if v.Key == key {
			return v
		}
	}

	return nil
}

func (bl BlogList) GenerateIndexPage() {
	// Get Index Template
	blogIndexTemp, err := template.ParseFiles("Templates/blogindex.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	var outBuffer bytes.Buffer
	blogIndexTemp.Execute(&outBuffer, bl)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog",
		FullURL: "http://www.flammablepenguins.com/blog/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create("./blog/index.html")
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
// Blog Post
func (bp *BlogPost) LoadBodyFromFile() {
	srcFile := fmt.Sprintf("blogdata/post/%s.html", bp.Key)
	bodyBytes, err := ioutil.ReadFile(srcFile)
	if err != nil {
		log.Fatalln(err)
	}
	bp.Body = template.HTML(bodyBytes)
}

func (bp *BlogPost) FixupDateFromPubStr() {
	const longform = "Mon, 02 Jan 2006 15:04:05 -0700"
	var err error

	bp.Date, err = time.Parse(longform, bp.Pubdate)
	if err != nil {
		log.Fatalln(err)
	}

	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
}

func (bp *BlogPost) SetNewPubDate(newPubDate time.Time) {
	bp.Date = newPubDate
	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
	bp.Pubdate = bp.Date.String()
}

func (bp *BlogPost) GeneratePage(blogTemp *template.Template) {
	var err error

	fileLoc := fmt.Sprintf("./blog/%04d/%02d/%s/", bp.Date.Year(), bp.Date.Month(), bp.Key)

	log.Println(fileLoc)

	err = os.MkdirAll(fileLoc, 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	var outBuffer bytes.Buffer
	blogTemp.Execute(&outBuffer, bp)

	// Twitter Card
	if len(bp.ShortDesc) < 4 {
		// Build Desc
		sum := regStripMarkup.ReplaceAllString(string(bp.Body), " ")
		if len(sum) > 200 {
			sum = sum[0:200]
		}

		bp.ShortDesc = sum
	}

	tc := &TwitterCard{
		Card:        "summary",
		Site:        "@EvilKimau",
		Title:       bp.Title,
		Description: bp.ShortDesc,
		Image:       "http://www.flammablepenguins.com/images/fp_twitter_tiny.png",
	}

	if len(bp.BannerImage) > 3 {
		tc.Card = "summary_large_image"
		tc.Image = "http://www.flammablepenguins.com" + bp.BannerImage
	} else if len(bp.SmallImage) > 3 {
		tc.Image = "http://www.flammablepenguins.com" + bp.SmallImage
	}

	// Write out Frame
	frameData := &SubPage{
		Title:     bp.Title,
		FullURL:   "http://www.flammablepenguins.com" + bp.Link,
		ShortDesc: bp.ShortDesc,
		Content:   template.HTML(outBuffer.String()),
		Twitter:   tc,
	}

	f, fileErr := os.Create(fileLoc + "/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	// Note: Don't like the fact we reference RootTemp here
	err = RootTemp.Execute(f, frameData)
	if err != nil {
		log.Fatalln(err)
		return
	}

	f.Close()
}

////////////////////////////////////////////////////////////////////////////////
// Entry Point
func GenerateBlog() {
	var err error

	os.RemoveAll("./blog/")
	err = os.MkdirAll("./blog/", 0777)

	loadJSONBlob("blogdata/blogData.js", &myData.Feed)

	blogTemp, err := template.ParseFiles("Templates/blogpost.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	blogCatTemp, err := template.ParseFiles("Templates/blogcat.html")
	if err != nil {
		log.Fatalln(err)
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
		v.FixupDateFromPubStr()
		v.LoadBodyFromFile()
		v.GeneratePage(blogTemp)

	}
	log.Println("Removed ", removedCat)

	sort.Sort(myData.Feed)
	myData.Feed.GenerateIndexPage()

	for k, v := range catMap {
		GenerateBlogCatergoryPage(k, &v, blogCatTemp)
	}

	// Save Out
	saveJSONBlob("blogdata/blogData2.js", &myData.Feed)
}

func GenerateBlogCatergoryPage(cat BlogCat, blist *BlogList, blogCat *template.Template) {
	var err error
	var outBuffer bytes.Buffer
	blogCat.Execute(&outBuffer, blist)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog - " + string(cat),
		FullURL: "http://www.flammablepenguins.com/blog/cat/" + cat.UrlVer() + "/",
		Content: template.HTML(outBuffer.String()),
	}

	err = os.MkdirAll("./blog/cat/"+cat.UrlVer(), 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	f, fileErr := os.Create("./blog/cat/" + cat.UrlVer() + "/index.html")
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
