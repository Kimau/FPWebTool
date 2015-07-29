package main

import (
	"encoding/xml"
	"log"
	"os"
	"time"
)

type SiteMapLink struct {
	XMLName    xml.Name  `xml:"url"`
	Loc        string    `xml:"loc"`
	LastMod    time.Time `xml:"lastmod"`
	Changefreq string    `xml:"changefreq"` //always hourly daily weekly monthly yearly never
	Priority   float64   `xml:"priority"`
}

////////////////////////////////////////////////////////////////////////////////
// Site Map
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
	f.WriteString(`</urlset><!--END-->`)

	f.Close()
}
