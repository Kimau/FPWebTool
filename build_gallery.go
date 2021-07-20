package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type GalleryPost struct {
	Date    time.Time     `json:"pubDate"`
	File    string        `json:"file"`
	Link    string        `json:"link"`
	Body    template.HTML `json:"body"`
	DateStr string        `json:"datestr"`
	Pubdate string        `json:"pubdate"`
	Include []string      `json:"include"`
}

var (
	regUrlSrc     *regexp.Regexp
	galleryTemp   *template.Template
	galSingleTemp *template.Template
	gallerySrcDir string
)

////////////////////////////////////////////////////////////////////////////////
//

func init() {
	var err error

	regUrlSrc = regexp.MustCompile(`src="([^"]+)"`)
	gallerySrcDir = filepath.Clean("./gallery")

	galleryTemp, err = template.ParseFiles("Templates/gallery.html")
	CheckErr(err)

	galSingleTemp, err = template.ParseFiles("Templates/galsingle.html")
	CheckErr(err)
}

////////////////////////////////////////////////////////////////////////////////
// Blog Listing
type GalleryList []*GalleryPost

func (gl GalleryList) Len() int      { return len(gl) }
func (gl GalleryList) Swap(i, j int) { gl[i], gl[j] = gl[j], gl[i] }
func (gl GalleryList) Less(i, j int) bool {
	a := filepath.Dir(gl[i].File)
	b := filepath.Dir(gl[j].File)
	if a == b {
		return gl[i].Date.After(gl[j].Date)
	}
	return a < b
}

// Support for .png .gif .jpg .mp4 .txt .html
func LoadGalleryFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	if strings.HasPrefix(info.Name(), "_") {
		return nil
	}

	var relPath string
	var newPost GalleryPost
	if _, err := os.Stat(path + ".json"); os.IsNotExist(err) {
		newPost.Date = info.ModTime()
		newPost.File = filepath.Clean(path)

		relPath, err = filepath.Rel(gallerySrcDir, newPost.File)
		newPost.Link = strings.TrimSuffix(relPath, filepath.Ext(relPath)) + "html"
		CheckErr(err)

		newPost.DateStr = fmt.Sprintf("%d %v %d", newPost.Date.Day(), newPost.Date.Month(), newPost.Date.Year())
		newPost.Pubdate = newPost.Date.Format(longformPubStr)
	} else {
		loadJSONBlob(path+".json", &newPost)
		genData.Gallery = append(genData.Gallery, &newPost)
		return nil // ALREADY PROCESSED
	}

	ext := filepath.Ext(path)

	if (ext == ".gif") || (ext == ".bmp") {
		newPost.Body = template.HTML(`<img class="pixel" src="` + newPost.File + `">`)
		newPost.Include = append(newPost.Include, relPath)
	} else if (ext == ".png") || (ext == ".jpg") || (ext == ".jpeg") {
		newPost.Body = template.HTML(`<img src="` + newPost.File + `">`)
		newPost.Include = append(newPost.Include, relPath)
	} else if (ext == ".mp4") || (ext == ".avi") || (ext == ".mov") {
		newPost.Body = template.HTML(`<video controls><source src="` + newPost.File + `" type="video/mp4"></video>`)
		newPost.Include = append(newPost.Include, relPath)
	} else if ext == ".txt" {

		body, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}

		newPost.Body = template.HTML("<pre>" + string(body) + "</pre>")

	} else if ext == ".md" {

		markdown, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}

		newPost.Body = MarkdownToHTML(markdown)

		files := regUrlSrc.FindAllSubmatch([]byte(newPost.Body), -1)
		for i := range files {
			dFile := filepath.Clean(string(files[i][1]))
			newPost.Include = append(newPost.Include, dFile)
		}

	} else if ext == ".html" {
		body, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}
		newPost.Body = template.HTML(body)

		files := regUrlSrc.FindAllSubmatch([]byte(newPost.Body), -1)
		for i := range files {
			dFile := filepath.Clean(string(files[i][1]))
			newPost.Include = append(newPost.Include, dFile)
		}
	} else if ext == ".json" {
		return nil
	} else {
		fmt.Println("Didn't parse: " + path)
		return nil
	}

	genData.Gallery = append(genData.Gallery, &newPost)
	saveJSONBlob(path+".json", &newPost)
	return nil
}

func LoadFromGalleryListFolder() {
	err := filepath.Walk(gallerySrcDir, LoadGalleryFile)
	if err != nil {
		log.Println(err)
	}
}

////////////////////////////////////////////////////////////////////////////////
//

func init() {

}

func GenerateGallery() {
	sort.Sort(genData.Gallery)

	// Sort out Folders
	tarDir := filepath.Join(publicHtmlRoot, "gallery")
	err := os.MkdirAll(tarDir, 0777)
	CheckErr(err)

	for i, g := range genData.Gallery {
		relPath, err := filepath.Rel(gallerySrcDir, g.File)
		if err != nil {
			log.Fatalln("Error in File Walk ", err)
		}
		htmlPath := strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".html"
		relPath = filepath.Dir(relPath)

		tarPath := filepath.Join(tarDir, htmlPath)

		// Copy Dependent Files
		for _, subF := range g.Include {
			srcPath := filepath.Join(gallerySrcDir, relPath, subF)
			tarPath = filepath.Join(tarDir, relPath, subF)

			err := os.MkdirAll(filepath.Dir(tarPath), 0777)
			CheckErr(err)

			_, err = CopyFileLazy(srcPath, tarPath)
			CheckErr(err)
		}

		{
			// Make Single
			var outFile *os.File
			outFile, err = os.Create(tarPath)
			CheckErrContext(err, "Error in File ")

			prevLink := ""
			if (i - 1) >= 0 {
				prevLink = "/gallery/" + genData.Gallery[i-1].Link
			}
			nextLink := ""
			if (i + 1) < len(genData.Gallery) {
				nextLink = "/gallery/" + genData.Gallery[i+1].Link
			}

			// Make Template
			var outBuffer bytes.Buffer
			err = galSingleTemp.Execute(&outBuffer, struct {
				Post *GalleryPost
				Prev string
				Next string
			}{g, prevLink, nextLink})
			CheckErrContext(err, "Error in Template ")

			// Write out Frame
			frameData := &SubPage{
				Title:   "Gallery: " + g.DateStr,
				FullURL: "/gallery/" + g.Link,
				Content: template.HTML(outBuffer.String()),
			}

			err = RootTemp.Execute(outFile, frameData)
			CheckErrContext(err, "Error in Template ")

			outFile.Close()
		}
	}

	// Make Index
	{
		var outFile *os.File
		outFile, err = os.Create(publicHtmlRoot + "gallery/index.html")
		CheckErrContext(err, "Error in File ")

		// Make Template
		var outBuffer bytes.Buffer
		err = galleryTemp.Execute(&outBuffer, genData)
		CheckErrContext(err, "Error in Template ")

		// Write out Frame
		frameData := &SubPage{
			Title:   "Gallery Posts",
			FullURL: "/gallery/",
			Content: template.HTML(outBuffer.String()),
		}

		err = RootTemp.Execute(outFile, frameData)
		CheckErrContext(err, "Error in Template ")

		outFile.Close()
	}
}
