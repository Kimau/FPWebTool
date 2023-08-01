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

// //////////////////////////////////////////////////////////////////////////////
// Site Map
func GenerateSiteMap() {
	var siteLinks []SiteMapLink

	siteLinks = append(siteLinks, SiteMapLink{
		Loc:        "/",
		LastMod:    time.Now(),
		Changefreq: "daily",
		Priority:   1.0,
	})

	siteLinks = append(siteLinks, SiteMapLink{
		Loc:        "/blog/",
		LastMod:    time.Now(),
		Changefreq: "daily",
		Priority:   1.0,
	})

	for _, v := range genData.Feed {
		siteLinks = append(siteLinks, SiteMapLink{
			Loc:        v.Link,
			LastMod:    v.Date,
			Changefreq: "monthly",
			Priority:   0.5,
		})
	}

	for _, g := range genData.Gallery {
		siteLinks = append(siteLinks, SiteMapLink{
			Loc:        g.Link,
			LastMod:    g.Date,
			Changefreq: "monthly",
			Priority:   0.5,
		})
	}

	f, err := os.Create(publicHtmlRoot + "sitemap.xml")
	CheckErrContext(err, "Error in Sitemap ")

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
