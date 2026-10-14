package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// A page only carries a robots meta when it asks for one. An empty
// <meta name="robots" content=""> is harmless but noise on 400 pages.
func TestRootTemplateRobots(t *testing.T) {
	t.Chdir("..")
	setupTemplates()

	plain := renderRoot(t, &SubPage{Title: "Blog", FullURL: "/blog/"})
	if strings.Contains(plain, `name="robots"`) {
		t.Error("indexable page emitted a robots meta")
	}

	listing := renderRoot(t, &SubPage{Title: "Micro Posts", FullURL: "/micro/", Robots: robotsListingOnly})
	if !strings.Contains(listing, `<meta name="robots" content="noindex, follow">`) {
		t.Errorf("noindex page did not emit its robots meta\n--- head ---\n%s", headOf(listing))
	}
}

// The redirect stubs used to say noindex alongside rel=canonical, which asks
// Google to both drop the page and treat it as the target.
func TestRedirectStubHasNoNoindex(t *testing.T) {
	t.Chdir(t.TempDir())

	WriteRedirectStub("/blog/cat/LudumDare/", "/blog/cat/Ludum_Dare_48/")

	b, err := os.ReadFile(publicHtmlRoot + "blog/cat/LudumDare/index.html")
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)

	if strings.Contains(out, "noindex") {
		t.Error("redirect stub still carries noindex")
	}
	for _, w := range []string{
		`<link rel="canonical" href="` + siteBaseURL + `/blog/cat/Ludum_Dare_48/">`,
		`<meta http-equiv="refresh" content="0; url=` + siteBaseURL + `/blog/cat/Ludum_Dare_48/">`,
	} {
		if !strings.Contains(out, w) {
			t.Errorf("redirect stub is missing:\n  %s", w)
		}
	}
}

// The sitemap must not list a page that tells Google not to index it, and its
// lastmod values have to be the content's dates, not the build's.
func TestSitemapIndexability(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(publicHtmlRoot, 0777); err != nil {
		t.Fatal(err)
	}

	old := time.Date(2010, 8, 1, 12, 0, 0, 0, time.UTC)
	mid := time.Date(2015, 5, 1, 12, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	genData = &GenerateData{
		Feed: BlogList{
			{Link: "/blog/2010/08/chess-and-unity/", Date: old},
			{Link: "/blog/2015/05/go-blog-generate/", Date: mid},
			{Link: "/blog/2026/08/exporting-investment/", Date: newest},
		},
		Categories: []CategoryPage{
			{Cat: "Chess", Count: minIndexedCategoryPosts - 1, Newest: old},
			{Cat: "Programming", Count: minIndexedCategoryPosts, Newest: mid},
		},
	}

	GenerateSiteMap()

	b, err := os.ReadFile(publicHtmlRoot + "sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)

	for _, unwanted := range []string{"/micro/", "/blog/cat/Chess/"} {
		if strings.Contains(out, siteBaseURL+unwanted+"<") {
			t.Errorf("sitemap lists noindex page %s", unwanted)
		}
	}

	// Each entry is <loc>, then <lastmod> on the next line.
	wantMod := map[string]string{
		"/":                      "2026-08-21",
		"/blog/":                 "2026-08-21",
		"/blog/cat/Programming/": "2015-05-01",
	}
	for loc, mod := range wantMod {
		entry := "<loc>" + siteBaseURL + loc + "</loc>\n    <lastmod>" + mod + "</lastmod>"
		if !strings.Contains(out, entry) {
			t.Errorf("sitemap entry for %s should have lastmod %s\n--- sitemap ---\n%s", loc, mod, out)
		}
	}
}
