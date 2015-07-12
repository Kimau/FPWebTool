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

var (
	blogTemp, blogIndexTemp *template.Template
)

const longformPubStr = "Mon, 02 Jan 2006 15:04:05 -0700"

////////////////////////////////////////////////////////////////////////////////
//

func init() {
	var err error

	blogIndexTemp, err = template.ParseFiles("Templates/blogindex.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

	blogTemp, err = template.ParseFiles("Templates/blogpost.html")
	if err != nil {
		log.Fatalln(err)
		return
	}

}

////////////////////////////////////////////////////////////////////////////////
// Blog Listing
type BlogList []*BlogPost

func (bl BlogList) Len() int           { return len(bl) }
func (bl BlogList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl BlogList) Less(i, j int) bool { return bl[i].Date.After(bl[j].Date) }

func (bl *BlogList) Get(key string) *BlogPost {
	for _, v := range *bl {
		if v.Key == key {
			return v
		}
	}

	return nil
}

func (bl *BlogList) LoadFromFile() {
	loadJSONBlob("blogdata/blogData.js", bl)
}

func (bl *BlogList) SaveToFile() {
	for _, v := range *bl {
		v.SaveBodyToFile()
	}
	saveJSONBlob("blogdata/blogData.js", bl)
}

func (bl *BlogList) GenerateIndexPage() {
	var outBuffer bytes.Buffer
	blogIndexTemp.Execute(&outBuffer, bl)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog",
		FullURL: "http://www.flammablepenguins.com/blog/",
		Content: template.HTML(outBuffer.String()),
	}

	f, fileErr := os.Create(publicHtmlRoot + "blog/index.html")
	if fileErr != nil {
		log.Fatalln("Error in File ", fileErr)
	}

	err := RootTemp.Execute(f, frameData)
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

func (bp *BlogPost) SaveBodyToFile() {
	if len(bp.Body) < 8 {
		log.Fatalln("Body is null")
		return
	}

	srcFile := fmt.Sprintf("blogdata/post/%s.html", bp.Key)

	os.Remove(srcFile)
	err := ioutil.WriteFile(srcFile, []byte(bp.Body), 0777)
	if err != nil {
		log.Fatalln(err)
	}
}

func (bp *BlogPost) FixupDateFromPubStr() {
	var err error

	bp.Date, err = time.Parse(longformPubStr, bp.Pubdate)
	if err != nil {
		log.Fatalln(err)
	}

	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
}

func (bp *BlogPost) SetNewPubDate(newPubDate time.Time) {
	bp.Date = newPubDate
	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
	bp.Pubdate = bp.Date.Format(longformPubStr)
}

func (bp *BlogPost) GeneratePage() {
	var err error

	bp.Link = fmt.Sprintf("blog/%04d/%02d/%s/", bp.Date.Year(), bp.Date.Month(), bp.Key)

	log.Println(bp.Link)

	err = os.MkdirAll(publicHtmlRoot+bp.Link, 0777)
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

	f, fileErr := os.Create(publicHtmlRoot + bp.Link + "index.html")
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

	os.RemoveAll(publicHtmlRoot + "blog/")
	err = os.MkdirAll(publicHtmlRoot+"blog/", 0777)
	if err != nil {
		log.Fatalln("Unable to make folder")
	}

	loadJSONBlob("blogdata/blogData.js", &myData.Feed)

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

		go func(bp *BlogPost) {
			bp.FixupDateFromPubStr()
			bp.LoadBodyFromFile()
			bp.GeneratePage()
		}(v)

	}
	log.Println("Removed ", removedCat)

	sort.Sort(myData.Feed)
	myData.Feed.GenerateIndexPage()

	for k, v := range catMap {
		GenerateBlogCatergoryPage(k, &v)
	}
}

func GenerateBlogCatergoryPage(cat BlogCat, blist *BlogList) {
	var err error
	var outBuffer bytes.Buffer

	blogIndexTemp.Execute(&outBuffer, blist)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog - " + string(cat),
		FullURL: "http://www.flammablepenguins.com/blog/cat/" + cat.UrlVer() + "/",
		Content: template.HTML(outBuffer.String()),
	}

	err = os.MkdirAll(publicHtmlRoot+"blog/cat/"+cat.UrlVer(), 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	f, fileErr := os.Create(publicHtmlRoot + "blog/cat/" + cat.UrlVer() + "/index.html")
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
