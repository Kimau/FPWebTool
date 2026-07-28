package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"strings"
)

// RSS represents the root element of the RSS feed
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	XMLNS   string   `xml:"xmlns:atom,attr"`
	Channel Channel  `xml:"channel"`
}

type ImageHeader struct {
	URL   string `xml:"url"`
	Link  string `xml:"link"`
	Title string `xml:"title"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

// Channel represents the channel element of the RSS feed
type Channel struct {
	Title       string      `xml:"title"`
	Image       ImageHeader `xml:"image"`
	Link        string      `xml:"link"`
	Description string      `xml:"description"`
	Language    string      `xml:"language"`
	AtomLink    AtomLink    `xml:"atom:link"`
	Items       []Item      `xml:"item"`
}

type Item struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Guid        string     `xml:"guid"`
	PubDate     string     `xml:"pubDate"`
	Description string     `xml:"description"`
	Enclosure   *Enclosure `xml:"enclosure"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

func blogPostToItem(post *BlogPost) Item {
	return Item{
		Title:       post.Title,
		Link:        AbsURL(post.Link),
		Guid:        AbsURL(post.Link),
		PubDate:     post.Pubdate,
		Description: post.ShortDesc,
		Enclosure:   createEnclosure(post.Hero),
	}
}

var mimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	// Add more as needed
}

// createEnclosure builds the RSS enclosure from an already-resolved image, so it
// inherits the same validation, casing and percent-encoding as the social card.
//
// A post with no real image gets no enclosure at all. Substituting the site
// default here would put the same logo on every item in the feed, which is worse
// than leaving it out - this is the one consumer that should not see the default.
func createEnclosure(img ResolvedImage) *Enclosure {
	if !img.OK() {
		return nil
	}

	mimeType, exists := mimeTypes[strings.ToLower(filepath.Ext(img.Path))]
	if !exists {
		mimeType = "application/octet-stream"
	}

	enc := &Enclosure{
		URL:  img.AbsPath(),
		Type: mimeType,
	}

	info, err := os.Stat("." + img.Path)
	if err != nil {
		fmt.Println("Warning: Could not stat enclosure image:", img.Path, "-", err)
		enc.Length = "0"
	} else {
		enc.Length = fmt.Sprintf("%d", info.Size())
	}

	return enc
}

// //////////////////////////////////////////////////////////////////////////////
// Generate Feed
func GenerateFeed() error {
	num_posts := min(len(genData.Feed), 30)

	rss := RSS{
		Version: "2.0",
		XMLNS:   "http://www.w3.org/2005/Atom",
		Channel: Channel{
			Title: "CBs GameDev Blog",
			Link:  siteBaseURL + "/",
			Image: ImageHeader{
				URL:   siteBaseURL + "/images/TitleBoard_Square.png",
				Link:  siteBaseURL + "/",
				Title: "CBs GameDev Blog",
			},
			AtomLink: AtomLink{
				Href: siteBaseURL + "/rss.xml",
				Rel:  "self",
				Type: "application/rss+xml",
			},
			Description: "Claire Blackshaw's random blog posts on gamedev, roleplaying and various bits n bobs.",
			Language:    "en-gb",
			Items:       make([]Item, num_posts),
		},
	}

	sort.Sort(genData.Feed)

	for i := 0; i < num_posts; i++ {
		rss.Channel.Items[i] = blogPostToItem(genData.Feed[i])
	}

	// Marshal the RSS data into XML
	xmlData, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling RSS data: %w", err)
	}

	// Write the XML data to the specified file
	file, err := os.Create(publicHtmlRoot + "rss.xml")
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	if _, err = file.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n"); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	if _, err = file.Write(xmlData); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}
