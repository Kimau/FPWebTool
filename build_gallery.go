package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type GalleryPost struct {
	Date     time.Time     `json:"pubDate"`
	File     string        `json:"file"`
	Link     string        `json:"link"`
	PostType string        `json:"posttype"`
	Body     template.HTML `json:"body"`
	DateStr  string        `json:"datestr"`
	Pubdate  string        `json:"pubdate"`
	Brief    string        `json:"brief"`
	Include  []string      `json:"include"`
}

var (
	regUrlSrc     *regexp.Regexp
	regHeader     *regexp.Regexp
	galleryTemp   *template.Template
	galSingleTemp *template.Template
	gallerySrcDir string
)

////////////////////////////////////////////////////////////////////////////////
//

func init() {
	regUrlSrc = regexp.MustCompile(`src="([^"]+)"`)
	// [^<]+ rather than [^"]+ so a page with several headings doesn't match from
	// the first opening tag all the way to the last closing one.
	regHeader = regexp.MustCompile(`<h(1|2|3)>([^<]+)</h(1|2|3)>`)
	gallerySrcDir = filepath.Clean("./gallery")
}

// DateISO is the post date in ISO 8601, as required by <time datetime>.
func (gp *GalleryPost) DateISO() string {
	return gp.Date.Format(time.RFC3339)
}

// HeroImage resolves the gallery post's own picture into a share image. Include[0]
// is the thumbnail gallery.html already renders, so this costs nothing new - it
// was simply never wired to og:image, and gallery pages are the one page type that
// is nothing but a picture.
//
// Include entries come in two shapes: bare relative for images ("Dreams/x.jpg")
// and leading-slash for markdown and html posts ("/Dreams/x.png"). path.Join
// normalises both, which also removes the double slash the template produces.
// Video posts resolve to nothing, since an .mp4 is not a usable card image.
func (gp *GalleryPost) HeroImage() ResolvedImage {
	if len(gp.Include) == 0 {
		return ResolvedImage{}
	}
	return validateImage(path.Join("/gallery", gp.Include[0]), gp.Brief, srcGallery)
}

// //////////////////////////////////////////////////////////////////////////////
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

type GalleryListByDate []*GalleryPost

func (gl GalleryListByDate) Len() int           { return len(gl) }
func (gl GalleryListByDate) Swap(i, j int)      { gl[i], gl[j] = gl[j], gl[i] }
func (gl GalleryListByDate) Less(i, j int) bool { return gl[i].Date.After(gl[j].Date) }

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
	if true { // _, err := os.Stat(path + ".json"); os.IsNotExist(err) {
		newPost.Date = info.ModTime()
		newPost.File = filepath.Clean(path)
		newPost.PostType = "post"

		relPath, err = filepath.Rel(gallerySrcDir, newPost.File)
		newPost.Link = filepath.ToSlash(strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".html")
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
		newPost.Include = append(newPost.Include, filepath.ToSlash(relPath))
		newPost.PostType = "image"
	} else if (ext == ".png") || (ext == ".jpg") || (ext == ".jpeg") || (ext == ".webp") {
		newPost.Body = template.HTML(`<img src="` + newPost.File + `">`)
		newPost.Include = append(newPost.Include, filepath.ToSlash(relPath))
		newPost.PostType = "image"
	} else if (ext == ".mp4") || (ext == ".avi") || (ext == ".mov") {
		newPost.Body = template.HTML(`<video controls><source src="` + newPost.File + `" type="video/mp4"></video>`)
		newPost.Include = append(newPost.Include, filepath.ToSlash(relPath))
		newPost.PostType = "movie"
	} else if ext == ".txt" {
		newPost.PostType = "txt"

		body, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}

		newPost.Body = template.HTML("<pre>" + string(body) + "</pre>")
		newPost.Brief = truncateRunes(string(body), 128)

	} else if ext == ".md" {

		markdown, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}

		newPost.Body = template.HTML(regUrlSrc.ReplaceAllStringFunc(string(MarkdownToHTML(markdown)), func(src string) string {
			m := regUrlSrc.FindStringSubmatch(src)
			dFile := fmt.Sprintf(`/%s/%s`, filepath.Dir(relPath), string(m[1]))
			newPost.Include = append(newPost.Include, filepath.ToSlash(dFile))

			return fmt.Sprintf(`src="%s"`, dFile)
		}))

		headers := regHeader.FindStringSubmatch(string(newPost.Body))
		if len(headers) > 2 {
			newPost.Brief = headers[2]
		} else {
			newPost.Brief = string(string(newPost.Body))
		}

	} else if ext == ".html" {
		body, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Failed to Read: " + path + " - " + err.Error())
			return err
		}

		newPost.Body = template.HTML(regUrlSrc.ReplaceAllStringFunc(string(template.HTML(body)), func(src string) string {
			m := regUrlSrc.FindStringSubmatch(src)
			dFile := fmt.Sprintf(`/%s/%s`, filepath.Dir(relPath), string(m[1]))
			newPost.Include = append(newPost.Include, filepath.ToSlash(dFile))

			return fmt.Sprintf(`src="%s"`, dFile)
		}))

		headers := regHeader.FindStringSubmatch(string(newPost.Body))
		if len(headers) > 2 {
			// Group 1 is the heading level digit, group 2 is the text.
			newPost.Brief = headers[2]
		} else {
			newPost.Brief = string(string(newPost.Body))
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
			srcPathInclude := filepath.Join(gallerySrcDir, subF)
			tarPathInclude := filepath.Join(tarDir, subF)

			err := os.MkdirAll(filepath.Dir(tarPathInclude), 0777)
			CheckErr(err)

			_, err = CopyFileLazy(srcPathInclude, tarPathInclude)
			CheckErr(err)
		}

		{
			prevLink := "/gallery/"
			if (i - 1) >= 0 {
				prevLink = "/gallery/" + genData.Gallery[i-1].Link
			}
			nextLink := "/gallery/"
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

			title := "Gallery: " + g.DateStr
			desc := g.Brief
			if strings.TrimSpace(desc) == "" {
				desc = "A picture from Claire Blackshaw's sketchbook, posted " + g.DateStr + "."
			}
			desc = truncateRunes(desc, 200)

			WritePage(&SubPage{
				Title:     title,
				FullURL:   "/gallery/" + g.Link,
				ShortDesc: desc,
				Content:   template.HTML(outBuffer.String()),
				Social:    listingCard(title, desc, g.HeroImage()),
			}, tarPath)
		}
	}

	// Make Index
	{
		var outBuffer bytes.Buffer
		err = galleryTemp.Execute(&outBuffer, genData)
		CheckErrContext(err, "Error in Template ")

		const desc = "Sketches, screenshots and experiments from Claire Blackshaw."

		var newest ResolvedImage
		for _, g := range genData.Gallery {
			if img := g.HeroImage(); img.OK() {
				newest = img
				break
			}
		}

		WritePage(&SubPage{
			Title:     "Gallery Posts",
			FullURL:   "/gallery/",
			ShortDesc: desc,
			Content:   template.HTML(outBuffer.String()),
			Social:    listingCard("Gallery Posts", desc, newest),
		}, publicHtmlRoot+"gallery/index.html")
	}

	genData.ShortGallery = nil
	genData.ShortGallery = append(genData.ShortGallery, genData.Gallery...)
	sort.Sort(genData.ShortGallery)
	if len(genData.ShortGallery) > 8 {
		genData.ShortGallery = genData.ShortGallery[:8]
	}
}
