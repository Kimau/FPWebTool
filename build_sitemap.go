package main

import (
	"encoding/xml"
	"log"
	"os"
	"time"
)

type SiteMapLink struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod"`
	Changefreq string   `xml:"changefreq"` //always hourly daily weekly monthly yearly never
	Priority   float64  `xml:"priority"`
}

// sitemapDate formats a timestamp as W3C date, which is what the sitemap
// protocol asks for. time.Time marshalled directly gives sub-second precision
// that some validators reject.
func sitemapDate(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format("2006-01-02")
}

// //////////////////////////////////////////////////////////////////////////////
// Site Map
func GenerateSiteMap() {
	var siteLinks []SiteMapLink

	// add appends one entry. loc must be a site-relative path; it is made
	// absolute here because the sitemap protocol rejects relative URLs and
	// Google silently discards the whole file when it sees them.
	add := func(loc string, mod time.Time, freq string, pri float64) {
		siteLinks = append(siteLinks, SiteMapLink{
			Loc:        AbsURL(loc),
			LastMod:    sitemapDate(mod),
			Changefreq: freq,
			Priority:   pri,
		})
	}

	now := time.Now()

	// Landing pages
	add("/", now, "daily", 1.0)
	add("/blog/", now, "daily", 1.0)
	add("/projects/", now, "monthly", 0.8)
	add("/job/", now, "monthly", 0.8)
	add("/gallery/", now, "weekly", 0.6)
	add("/micro/", now, "weekly", 0.6)

	// Blog posts
	newestPost := time.Time{}
	for _, v := range genData.Feed {
		add(v.Link, v.Date, "monthly", 0.7)
		if v.Date.After(newestPost) {
			newestPost = v.Date
		}
	}

	// Category listings
	for _, c := range genData.Categories {
		add("/blog/cat/"+c.UrlVer()+"/", newestPost, "weekly", 0.4)
	}

	// Gallery posts. g.Link is relative to the gallery folder, so it needs the
	// /gallery/ prefix before it means anything.
	for _, g := range genData.Gallery {
		add("/gallery/"+g.Link, g.Date, "monthly", 0.3)
	}

	f, err := os.Create(publicHtmlRoot + "sitemap.xml")
	CheckErrContext(err, "Error in Sitemap ")

	f.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	f.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, v := range siteLinks {
		s, err := xml.MarshalIndent(v, "  ", "  ")
		if err != nil {
			log.Fatalln("Problem writing link ", err)
		}
		f.Write(s)
		f.WriteString("\n")
	}
	f.WriteString(`</urlset>` + "\n")

	f.Close()

	log.Printf("Sitemap written with %d urls\n", len(siteLinks))
}
