package main

import (
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	siteTwitterHandle = "@EvilKimau"
	siteName          = "Forest of Fun"
	facebookAuthor    = "https://www.facebook.com/Kimau"
	facebookPublisher = "https://www.facebook.com/flammablepenguins"

	siteDefaultDesc = "Forest of Fun is the internet home of Claire Blackshaw, " +
		"programmer and designer, on which all her stuff can be found and linked from."

	// Twitter truncates around 200 characters and Facebook around 300. Cutting to
	// 200 on a word boundary reads better than letting either platform chop
	// mid-word. Only the card copy is shortened - ShortDesc itself stays long,
	// because the listing pages render it as body text.
	cardDescLimit = 200
)

// SocialCard is the resolved <head> payload for one page. Every page gets one;
// there is no "no card" state any more. It used to be called TwitterCard and to
// exist only on blog posts, which is why og:image and og:type were gated behind
// it and every listing page shipped without them.
type SocialCard struct {
	Card        string // twitter:card
	Site        string // twitter:site
	Creator     string // twitter:creator
	Title       string
	Description string

	Img ResolvedImage

	Type          string   // og:type - "article" or "website"
	PublishedTime string   // RFC3339; articles only
	ModifiedTime  string   // RFC3339; articles only
	Tags          []string // article:tag
}

// IsArticle gates the article:* block in root.html.
func (sc *SocialCard) IsArticle() bool { return sc.Type == "article" }

// FacebookAuthor and FacebookPublisher keep the two hardcoded profile URLs out of
// the template, next to the rest of the site identity.
func (sc *SocialCard) FacebookAuthor() string    { return facebookAuthor }
func (sc *SocialCard) FacebookPublisher() string { return facebookPublisher }
func (sc *SocialCard) SiteName() string          { return siteName }

type SubPage struct {
	Title     string        `json:"title"`
	Content   template.HTML `json:"content"`
	ShortDesc string
	FullURL   string
	Social    *SocialCard
}

// Canonical is the absolute URL for this page, used by <link rel="canonical">
// and og:url. Both are ignored by crawlers if given a relative path.
func (sp *SubPage) Canonical() string {
	return AbsURL(sp.FullURL)
}

// Desc is the page description, falling back to the site blurb. root.html used to
// spell this out as an {{if}}/{{else}} pair, twice, with the fallback text copied
// into both branches.
func (sp *SubPage) Desc() string {
	if strings.TrimSpace(sp.ShortDesc) != "" {
		return sp.ShortDesc
	}
	return siteDefaultDesc
}

// Normalise fills in everything site-wide so the template can address .Social.*
// unconditionally. This is the whole fix for the missing cards: og:image and
// og:type sat behind {{if .Twitter}}, and .Twitter was only ever set by
// BlogPost.GeneratePage, so /blog/, /blog/cat/*, /micro/, /gallery/, the gallery
// singles, /, /projects/ and /job/ all shared as bare text links.
//
// This is also the one and only place the site default image is substituted, so
// "we found nothing" and "we chose the default" cannot drift apart.
func (sp *SubPage) Normalise() {
	if sp.Social == nil {
		sp.Social = &SocialCard{}
	}
	c := sp.Social

	if c.Site == "" {
		c.Site = siteTwitterHandle
	}
	if c.Creator == "" {
		c.Creator = siteTwitterHandle
	}
	if c.Title == "" {
		c.Title = sp.Title
	}
	if c.Description == "" {
		c.Description = sp.Desc()
	}
	if c.Type == "" {
		c.Type = "website"
	}
	if !c.Img.OK() {
		c.Img = defaultSocialImage()
	}
	if c.Img.Alt == "" {
		c.Img.Alt = c.Title
	}

	c.Description = truncateRunes(c.Description, cardDescLimit)

	// Ask for the big card only when the image can actually fill it.
	if c.Card == "" {
		c.Card = "summary"
		if c.Img.Landscape() {
			c.Card = "summary_large_image"
		}
	}
}

// WritePage is the only way a framed page reaches disk. Nine generators used to
// open a file and call RootTemp.Execute by hand, which is how several page types
// quietly ended up with no social card at all; routing them all through here is
// what structurally guarantees Normalise ran.
func WritePage(sp *SubPage, outPath string) {
	sp.Normalise()

	err := os.MkdirAll(filepath.Dir(outPath), 0777)
	CheckErrContext(err, "Error in Mkdir ", outPath)

	f, err := os.Create(outPath)
	CheckErrContext(err, "Error in File ", outPath)
	defer f.Close()

	err = RootTemp.Execute(f, sp)
	CheckErrContext(err, "Error in Root Template ", outPath)
}

// //////////////////////////////////////////////////////////////////////////////
// Card builders

// SocialCard builds the head payload for a post. Hero is passed through as-is,
// including the zero value: the site default belongs to Normalise, so blogpost.html
// and the listings can still tell whether there is a real image to render.
//
// This runs at page-generation time rather than at load, because Category is only
// filled in by the GenerateBlog loop and article:tag needs it.
func (bp *BlogPost) SocialCard() *SocialCard {
	tags := make([]string, 0, len(bp.Category))
	for _, c := range bp.Category {
		tags = append(tags, string(c))
	}

	return &SocialCard{
		Title:         bp.Title,
		Description:   bp.ShortDesc,
		Img:           bp.Hero,
		Type:          "article",
		PublishedTime: bp.DateISO(),
		ModifiedTime:  bp.DateISO(),
		Tags:          tags,
	}
}

// listingCard is the card for an index page. It borrows the newest post's image
// so /blog/, /blog/cat/X/ and /micro/ share as something other than the site logo,
// falling back to the default when the list has nothing usable.
func listingCard(title, desc string, newest ResolvedImage) *SocialCard {
	return &SocialCard{
		Title:       title,
		Description: desc,
		Img:         newest,
		Type:        "website",
	}
}

// firstHero returns the first real image in a list of posts, which is not always
// the newest post's - the newest few may be text-only.
func firstHero(list BlogList) ResolvedImage {
	for _, bp := range list {
		if bp.Hero.OK() {
			return bp.Hero
		}
	}
	return ResolvedImage{}
}

// //////////////////////////////////////////////////////////////////////////////
// Resolution

// resolveAllSocial fills every derived social field before any template runs.
//
// This cannot live in GeneratePage, for four independent reasons:
//
//  1. GenerateMicro renders /micro/ before GenerateBlog ever calls
//     BlogPost.GeneratePage.
//  2. The plain startup path in main() runs generateDataOnly, GenerateGallery,
//     GenerateMicro and GenerateFeed and never calls GenerateBlog at all - so
//     rss.xml would ship with no enclosures on every non-"-gen" run.
//  3. blogindex.html, blogcat.html and about.html all render from these same
//     *BlogPost pointers, so anything computed per-page is a hidden ordering
//     dependency waiting for the next refactor to break.
//  4. ShortDesc has to exist before blogpost.html executes, and it used to be
//     derived six lines after it.
func resolveAllSocial() {
	log.Println("Resolving social images...")

	for _, bp := range genData.Feed {
		if len(bp.Body) < 1 {
			CheckErr(bp.LoadBodyFromFile())
		}
		bp.EnsureShortDesc()
		bp.Hero = resolvePostImage(bp.BannerImage, bp.SmallImage, string(bp.Body), bp.Title)
	}

	for _, mp := range genData.Micro {
		mp.Hero = resolvePostImage(mp.BannerImage, mp.SmallImage, string(mp.Body), mp.Title)
	}

	logResolutionSummary()
}

// logResolutionSummary prints where the share images came from. It is the
// regression alarm for this whole subsystem: youtube=0 means the build was
// silently offline, and a jump in the number of posts with no image at all means
// validation has broken.
func logResolutionSummary() {
	counts := map[string]int{}
	none, hero := 0, 0
	for _, bp := range genData.Feed {
		if !bp.Hero.OK() {
			none++
			continue
		}
		counts[bp.Hero.Source]++
		if bp.Hero.Standalone() {
			hero++
		}
	}

	sources := make([]string, 0, len(counts))
	for k := range counts {
		sources = append(sources, k)
	}
	sort.Strings(sources)

	parts := make([]string, 0, len(sources)+1)
	for _, s := range sources {
		parts = append(parts, s+"="+strconv.Itoa(counts[s]))
	}
	parts = append(parts, "none="+strconv.Itoa(none))

	log.Printf("Social images across %d posts: %s\n", len(genData.Feed), strings.Join(parts, " "))

	// Separately: how many of those are also drawn on the post itself. The rest are
	// card-only because the reader can already see the picture in the body.
	log.Printf("  of which rendered as a hero on the post: %d\n", hero)
}
