package main

import (
	"strings"
	"testing"
)

func TestLegacyDesc(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"leading heading is dropped and blocks get separated",
			`<h3>Today’s Goals</h3><ul><li>Build World Manager</li><li>Make base Hand &amp; Frank</li></ul><p>So currently I’m doing one of the things I like least.</p>`,
			"Build World Manager. Make base Hand & Frank. So currently I’m doing one of the things I like least.",
		},
		{
			"heading after an image still counts as leading",
			`<img src="x.jpg" alt=""/><br/><br/><h3>Cube Maps Ahoy</h3><p>So there is a slight glitch with the reflection but ALMOST there.</p>`,
			"So there is a slight glitch with the reflection but ALMOST there.",
		},
		{
			"heading after text is kept",
			`<p>Intro line</p><h2>Query Response</h2><p>Working now.</p>`,
			"Intro line. Query Response. Working now.",
		},
		{
			"text outside a paragraph runs into it without a stray full stop",
			"Hi all,\n<p>So the website rework has been quite an endeavour.</p>",
			"Hi all, So the website rework has been quite an endeavour.",
		},
		{
			"punctuation inside a closing quote is respected",
			`<p>She said "stop."</p><p>Then went.</p>`,
			`She said "stop." Then went.`,
		},
		{
			"image-only post has no description",
			`<img src="x.jpg" alt="x"/>`,
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyDesc(tc.body); got != tc.want {
				t.Errorf("\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestExcerpt(t *testing.T) {
	t.Run("ends on the last full sentence", func(t *testing.T) {
		s := strings.Repeat("Short sentence here. ", 30)
		got := excerpt(s, legacyDescLimit)
		if len(got) > legacyDescLimit || !strings.HasSuffix(got, "here.") {
			t.Errorf("got %d bytes ending %q", len(got), got[len(got)-10:])
		}
	})

	t.Run("no sentence end in reach cuts on a word with an ellipsis", func(t *testing.T) {
		s := strings.Repeat("word ", 100)
		got := excerpt(s, legacyDescLimit)
		if !strings.HasSuffix(got, "word...") || len(got) > legacyDescLimit+3 {
			t.Errorf("got %d bytes ending %q", len(got), got[len(got)-10:])
		}
	})

	t.Run("an early sentence end is not worth losing most of the text for", func(t *testing.T) {
		s := "Yes. " + strings.Repeat("word ", 100)
		if got := excerpt(s, legacyDescLimit); got == "Yes." {
			t.Error("cut back to a 4 byte sentence")
		}
	})

	t.Run("short text is untouched", func(t *testing.T) {
		if got := excerpt("  My year as per Facebook. ", legacyDescLimit); got != "My year as per Facebook." {
			t.Errorf("got %q", got)
		}
	})
}

// Micro posts keep the desc from their sidecar, which is their TLDR; only the
// legacy imports are re-derived.
func TestEnsureShortDescLeavesMicroPostsAlone(t *testing.T) {
	micro := &BlogPost{IsMicro: true, ShortDesc: "The TLDR.", Body: "<p>Body text that differs.</p>"}
	micro.EnsureShortDesc()
	if micro.ShortDesc != "The TLDR." {
		t.Errorf("micro desc was replaced with %q", micro.ShortDesc)
	}

	legacy := &BlogPost{ShortDesc: "The good news is the Comic Viewer 01 is written an...", Body: "<p>The Comic Viewer 0.1 is written and working.</p>"}
	legacy.EnsureShortDesc()
	if legacy.ShortDesc != "The Comic Viewer 0.1 is written and working." {
		t.Errorf("legacy desc = %q", legacy.ShortDesc)
	}

	imageOnly := &BlogPost{ShortDesc: "Stored fallback.", Body: `<img src="x.jpg" alt=""/>`}
	imageOnly.EnsureShortDesc()
	if imageOnly.ShortDesc != "Stored fallback." {
		t.Errorf("image-only post lost its stored desc: %q", imageOnly.ShortDesc)
	}
}
