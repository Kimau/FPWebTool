package main

import (
	"testing"
	"time"
)

// The front page only shows gallery posts from the last six months, newest first,
// and shows nothing at all rather than padding the strip out with old work.
func TestRecentGallery(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	post := func(name string, monthsAgo int) *GalleryPost {
		return &GalleryPost{File: name, Date: now.AddDate(0, -monthsAgo, 0)}
	}

	// Folder order deliberately disagrees with date order: the strip must not
	// inherit the gallery page's folder-first sort.
	all := GalleryList{
		post("a/old.jpg", 9),
		post("a/recent.jpg", 2),
		post("b/newest.jpg", 0),
		post("b/stale.jpg", 7),
	}

	got := recentGallery(all, now)
	want := []string{"b/newest.jpg", "a/recent.jpg"}
	if len(got) != len(want) {
		t.Fatalf("got %d posts, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].File != w {
			t.Errorf("post %d: got %s, want %s", i, got[i].File, w)
		}
	}

	if stale := recentGallery(GalleryList{post("a/old.jpg", 9)}, now); len(stale) != 0 {
		t.Errorf("expected no posts when everything is stale, got %d", len(stale))
	}

	var many GalleryList
	for i := 0; i < 12; i++ {
		many = append(many, post("c/x.jpg", 0))
	}
	if capped := recentGallery(many, now); len(capped) != 8 {
		t.Errorf("expected the strip capped at 8, got %d", len(capped))
	}
}
