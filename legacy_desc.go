package main

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// legacyDescLimit is the length of a description derived for an old WordPress
// post. Long enough for Google's snippet and a listing summary, and the cut
// lands on a sentence end where there is one.
const legacyDescLimit = 300

var (
	// Closing a block, or a line break, ends a run of text. Without this a list
	// or a heading runs straight into the next paragraph: "Build World Manager
	// Make base Hand & Frank Get Hand to Pick up Frank So currently I'm...".
	regBlockEnd = regexp.MustCompile(`(?i)</(?:p|div|li|h[1-6]|td|th|blockquote|pre|dt|dd|figcaption)\s*>|<br\s*/?>|<hr\s*/?>`)
	regHeading  = regexp.MustCompile(`(?is)<h[1-6][^>]*>.*?</h[1-6]\s*>`)

	// A sentence end followed by a space, or by the end of the text.
	regSentenceEnd = regexp.MustCompile(`[.!?\x{2026}]["'\x{2019}\x{201D})]*(?:\s|$)`)
)

// legacyDesc derives a description from a legacy post's body.
//
// The desc field in blogData.js is the WordPress importer's auto-excerpt, and it
// is bad on nearly all 310 posts: cut mid-word at a fixed length ("is written
// an...", "Web Came"), punctuation mangled ("That's" became "That is", "wasn't" became
// "wasnt", "0.1" became "01"), the odd raw <br /> or <strong>, and on
// podcast-3-aqua-attack a different post's text entirely. blogdata/ is left as
// imported, so this is fixed on the way out instead. The body is the real text.
func legacyDesc(body string) string {
	// A heading before any text is the post's own title again, or near enough
	// ("Day & Night Cycle" on day-night-cycle). Same rule as micro posts.
	if loc := regHeading.FindStringIndex(body); loc != nil {
		before := html.UnescapeString(regStripMarkup.ReplaceAllString(body[:loc[0]], ""))
		if strings.TrimSpace(before) == "" {
			body = body[:loc[0]] + body[loc[1]:]
		}
	}

	const mark = "\x00"
	s := regBlockEnd.ReplaceAllString(body, mark)
	s = regStripMarkup.ReplaceAllString(s, " ")
	// &nbsp; decodes to U+00A0, which RE2's \s does not match, so it would
	// survive the whitespace collapse below as a double space.
	s = strings.ReplaceAll(html.UnescapeString(s), " ", " ")

	var b strings.Builder
	for _, chunk := range strings.Split(s, mark) {
		chunk = strings.TrimSpace(regUrlSpace.ReplaceAllString(chunk, " "))
		if chunk == "" {
			continue
		}
		if b.Len() > 0 {
			if !endsClause(b.String()) {
				b.WriteString(".")
			}
			b.WriteString(" ")
		}
		b.WriteString(chunk)
		if b.Len() > 2*legacyDescLimit {
			break
		}
	}

	return excerpt(b.String(), legacyDescLimit)
}

// endsClause reports whether text already ends in punctuation that a following
// block can sit after, looking through closing quotes and brackets.
func endsClause(s string) bool {
	s = strings.TrimRight(s, "\"')]’”")
	r, _ := utf8.DecodeLastRuneInString(s)
	return strings.ContainsRune(".!?:;,…", r)
}

// excerpt cuts s to at most max bytes. It ends on the last full sentence when
// that keeps at least 40% of the room, and otherwise on a word boundary with an
// ASCII ellipsis so a reader can see it was cut.
func excerpt(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}

	cut := truncateRunes(s, max)
	if ends := regSentenceEnd.FindAllStringIndex(cut, -1); len(ends) > 0 {
		if end := ends[len(ends)-1][1]; end >= max*2/5 {
			return strings.TrimSpace(cut[:end])
		}
	}
	return strings.TrimRight(cut, " ,;:-") + "..."
}
