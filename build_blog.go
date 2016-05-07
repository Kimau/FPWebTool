package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"sync"
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

	Image       string `json:"image,omitempty"`
	ImageWidth  string `json:"imageWidth,omitempty"`
	ImageHeight string `json:"imageHeight,omitempty"`

	Category []BlogCat     `json:"-"`
	Date     time.Time     `json:"-"`
	Body     template.HTML `json:"-"`
	DateStr  string        `json:"-"`
}

var (
	blogTemp, blogIndexTemp *template.Template

	regUrlChar     *regexp.Regexp
	regUrlSpace    *regexp.Regexp
	regStripMarkup *regexp.Regexp
	regCatchImage  *regexp.Regexp
)

const longformPubStr = "Mon, 02 Jan 2006 15:04:05 -0700"

////////////////////////////////////////////////////////////////////////////////
//

func init() {
	var err error

	regUrlChar = regexp.MustCompile("[^A-Za-z]")
	regUrlSpace = regexp.MustCompile(" ")
	regStripMarkup = regexp.MustCompile("<[^<>]*>")
	regCatchImage = regexp.MustCompile("<img[^>]*\"(/images/Blog[^\"]*)\"[^>]*>")

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
// Blog Cat
func (c BlogCat) UrlVer() string {
	return regUrlSpace.ReplaceAllString(regUrlChar.ReplaceAllString(string(c), ""), "_")
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

func (bl *BlogList) GeneratePage() {
	var outBuffer bytes.Buffer
	blogIndexTemp.Execute(&outBuffer, bl)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog",
		FullURL: "http://www.claire-blackshaw.com/blog/",
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
func (bp *BlogPost) LoadBodyFromFile() error {
	srcFile := fmt.Sprintf("blogdata/post/%d/%s.html", bp.Date.Year(), bp.Key)
	bodyBytes, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return err
	}

	bp.Body = template.HTML(bodyBytes)
	return nil
}

func (bp *BlogPost) SaveBodyToFile() error {
	if len(bp.Body) < 8 {
		return errors.New("Body is null or less than 8 characters")
	}

	// Make Folder
	destFolder := fmt.Sprintf("blogdata/post/%d", bp.Date.Year())
	err := os.MkdirAll(destFolder, 077)
	if err != nil {
		return err
	}

	srcFile := fmt.Sprintf("blogdata/post/%d/%s.html", bp.Date.Year(), bp.Key)

	os.Remove(srcFile)
	err = ioutil.WriteFile(srcFile, []byte(bp.Body), 0777)
	if err != nil {
		log.Fatalln(err)
	}

	return nil
}

func (bp *BlogPost) FixupDateFromPubStr() {
	var err error

	bp.Date, err = time.Parse(longformPubStr, bp.Pubdate)
	if err != nil {
		log.Fatalln(err)
	}

	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
	bp.Link = fmt.Sprintf("blog/%04d/%02d/%s/", bp.Date.Year(), bp.Date.Month(), bp.Key)
}

func (bp *BlogPost) SetNewPubDate(newPubDate time.Time) {
	bp.Date = newPubDate
	bp.DateStr = fmt.Sprintf("%d %v %d", bp.Date.Day(), bp.Date.Month(), bp.Date.Year())
	bp.Link = fmt.Sprintf("blog/%04d/%02d/%s/", bp.Date.Year(), bp.Date.Month(), bp.Key)
	bp.Pubdate = bp.Date.Format(longformPubStr)
}

func (bp *BlogPost) GeneratePage() {
	var err error

	log.Println(bp.Link)

	err = os.MkdirAll(publicHtmlRoot+bp.Link, 0777)
	if err != nil {
		log.Fatalln("Error in Mkdir ", err)
	}

	// Get Banner Image Size (if I have one)
	if len(bp.BannerImage) > 3 {
		w, h, e := getImageDimension("." + bp.BannerImage)
		if e != nil {
			log.Fatalln("Error getting Banner:", bp.Title, "\n>", bp.BannerImage, "\n>", e)
		}

		bp.Image = bp.BannerImage
		bp.ImageWidth = fmt.Sprintf("%d", w)
		bp.ImageHeight = fmt.Sprintf("%d", h)
	} else if len(bp.SmallImage) > 3 {
		bp.Image = bp.SmallImage
		bp.ImageWidth, bp.ImageHeight = "120", "120"
	} else {
		bp.Image = "/images/fp_twitter_tiny.png"
		bp.ImageWidth, bp.ImageHeight = "120", "120"
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
		Image:       "http://www.claire-blackshaw.com/images/fp_twitter_tiny.png",
	}

	if len(bp.BannerImage) > 3 {
		tc.Card = "summary_large_image"
		tc.Image = "http://www.claire-blackshaw.com" + bp.BannerImage
	} else if len(bp.SmallImage) > 3 {
		tc.Image = "http://www.claire-blackshaw.com" + bp.SmallImage
	}

	// Write out Frame
	frameData := &SubPage{
		Title:     bp.Title,
		FullURL:   "http://www.claire-blackshaw.com" + bp.Link,
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
	var wg sync.WaitGroup

	os.RemoveAll(publicHtmlRoot + "blog/")
	err = os.MkdirAll(publicHtmlRoot+"blog/", 0777)
	if err != nil {
		log.Fatalln("Unable to make folder")
	}

	myData.Feed.LoadFromFile()

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
		wg.Add(1)
		go func(bp *BlogPost) {
			defer wg.Done()
			err := bp.LoadBodyFromFile()
			if err != nil {
				log.Fatalln(err)
				return
			}

			bp.GeneratePage()

		}(v)
	}
	log.Println("Removed ", removedCat)

	sort.Sort(myData.Feed)
	myData.Feed.GeneratePage()

	for k, v := range catMap {
		GenerateBlogCatergoryPage(k, &v)
	}

	wg.Wait()
}

func GenerateBlogCatergoryPage(cat BlogCat, blist *BlogList) {
	var err error
	var outBuffer bytes.Buffer

	blogIndexTemp.Execute(&outBuffer, blist)

	// Write out Frame
	frameData := &SubPage{
		Title:   "Blog - " + string(cat),
		FullURL: "http://www.claire-blackshaw.com/blog/cat/" + cat.UrlVer() + "/",
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
