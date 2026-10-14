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

// fileModTime is the newest modification time among the source files a page is
// built from, or the zero time (which sitemapDate turns into "now") if none of
// them can be read.
func fileModTime(paths ...string) time.Time {
	var newest time.Time
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
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

	// lastmod has to be the date the content actually changed. It used to be the
	// build time for every landing page and the newest post on the whole site for
	// every category, so /blog/cat/Chess/ (newest post 2010) claimed to change on
	// every build. Google only uses lastmod while it keeps matching what it finds
	// on the page, and stops trusting it for the whole sitemap once it doesn't.
	newestPost := time.Time{}
	for _, v := range genData.Feed {
		if v.Date.After(newestPost) {
			newestPost = v.Date
		}
	}
	newestGallery := time.Time{}
	for _, g := range genData.Gallery {
		if g.Date.After(newestGallery) {
			newestGallery = g.Date
		}
	}

	// Landing pages. /micro/ is deliberately absent: it is noindex, because it
	// repeats every micro post in full.
	add("/", newestPost, "daily", 1.0)
	add("/blog/", newestPost, "daily", 1.0)
	add("/projects/", fileModTime("Data/hobby.js", "Templates/projects.html"), "monthly", 0.8)
	add("/job/", fileModTime("Data/job.js", "Templates/job.html"), "monthly", 0.8)
	add("/gallery/", newestGallery, "weekly", 0.6)

	// Blog posts
	for _, v := range genData.Feed {
		add(v.Link, v.Date, "monthly", 0.7)
	}

	// Category listings. The thin ones are noindex, and a noindex URL in the
	// sitemap is an error in Search Console.
	for _, c := range genData.Categories {
		if !c.Indexable() {
			continue
		}
		add("/blog/cat/"+c.Cat.UrlVer()+"/", c.Newest, "weekly", 0.4)
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
