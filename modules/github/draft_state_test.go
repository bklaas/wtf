package github

import (
	"testing"

	ghb "github.com/google/go-github/v89/github"
	"github.com/rivo/tview"
	"gotest.tools/assert"
)

func draftWidget(show bool, iconSet string) *Widget {
	return &Widget{
		settings: &Settings{
			showDraftStatus: show,
			reviewIcons:     iconSet,
		},
	}
}

func prWithDraft(draft bool) *ghb.PullRequest {
	return &ghb.PullRequest{Draft: &draft}
}

func TestDraftString(t *testing.T) {
	tests := []struct {
		name     string
		show     bool
		iconSet  string
		draft    bool
		expected string
	}{
		{
			name:     "disabled renders nothing at all",
			show:     false,
			iconSet:  "emoji",
			draft:    true,
			expected: "",
		},
		{
			name:     "emoji draft",
			show:     true,
			iconSet:  "emoji",
			draft:    true,
			expected: "\U0001f32c️ ",
		},
		{
			name:     "emoji ready is blank",
			show:     true,
			iconSet:  "emoji",
			draft:    false,
			expected: "   ",
		},
		{
			name:     "text draft",
			show:     true,
			iconSet:  "text",
			draft:    true,
			expected: "[grey]✎[white] ",
		},
		{
			name:     "text ready is blank, colour tags not counted",
			show:     true,
			iconSet:  "text",
			draft:    false,
			expected: "  ",
		},
		{
			name:     "an unknown icon set falls back to emoji",
			show:     true,
			iconSet:  "nonsense",
			draft:    true,
			expected: "\U0001f32c️ ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := draftWidget(tt.show, tt.iconSet).draftString(prWithDraft(tt.draft))
			assert.Equal(t, tt.expected, got)
		})
	}
}

// A pull request fetched without the draft field must not be treated as a
// draft, since GetDraft() is the only thing standing between a nil pointer and
// a panic here.
func TestDraftStringHandlesMissingDraftField(t *testing.T) {
	got := draftWidget(true, "emoji").draftString(&ghb.PullRequest{})
	assert.Equal(t, "   ", got)
}

// The whole point of the blank is that a draft row and a ready row push the
// following columns to the same place.
func TestDraftMarkerWidthsMatch(t *testing.T) {
	for name := range draftIcons {
		widget := draftWidget(true, name)

		drafted := widget.draftString(prWithDraft(true))
		ready := widget.draftString(prWithDraft(false))

		assert.Equal(t,
			tview.TaggedStringWidth(drafted),
			tview.TaggedStringWidth(ready),
			"icon set %q: draft and ready markers differ in width", name)
	}
}
