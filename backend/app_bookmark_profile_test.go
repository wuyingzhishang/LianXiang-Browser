package backend

import "testing"

func TestBookmarksForProfileHonorsGlobalAndProfileClosures(t *testing.T) {
	bookmarks := []BrowserBookmark{
		{Name: "Enabled", URL: "https://enabled.example/"},
		{Name: "Globally disabled", URL: "https://global.example/", Disabled: true},
		{Name: "Disabled for A", URL: "https://profile.example/", DisabledProfileIDs: []string{"profile-a"}},
	}

	forProfileA := bookmarksForProfile(bookmarks, "profile-a")
	if len(forProfileA) != 1 || forProfileA[0].Name != "Enabled" {
		t.Fatalf("unexpected bookmarks for profile-a: %#v", forProfileA)
	}

	forProfileB := bookmarksForProfile(bookmarks, "profile-b")
	if len(forProfileB) != 2 || forProfileB[1].Name != "Disabled for A" {
		t.Fatalf("unexpected bookmarks for profile-b: %#v", forProfileB)
	}
}
