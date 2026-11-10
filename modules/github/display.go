package github

import (
	"fmt"

	ghb "github.com/google/go-github/v89/github"
)

func (widget *Widget) display() {
	widget.Redraw(widget.content)
}

func (widget *Widget) content() (string, string, bool) {
	repo := widget.currentGithubRepo()

	// Choses the correct place to scroll to when changing sources
	if len(widget.View.GetHighlights()) > 0 {
		widget.View.ScrollToHighlight()
	} else {
		widget.View.ScrollToBeginning()
	}

	if repo == nil {
		return widget.CommonSettings().Title, " GitHub repo data is unavailable ", false
	}

	// initial maxItems count
	widget.Items = make([]int, 0)
	widget.SetItemCount(len(repo.CachedMyReviewRequests))

	title := fmt.Sprintf("%s - %s", widget.CommonSettings().Title, widget.title(repo))
	if repo.Err != nil {
		return title, repo.Err.Error(), true
	}

	_, _, width, _ := widget.View.GetRect()
	str := widget.settings.PaginationMarker(len(widget.GithubRepos), widget.Idx, width)
	if widget.settings.showStats {
		str += fmt.Sprintf("\n [%s]Stats[white]\n", widget.settings.Colors.Subheading)
		str += widget.displayStats(repo)
	}
	if widget.settings.showOpenReviewRequests {
		str += fmt.Sprintf("\n [%s]Open Review Requests[white]\n", widget.settings.Colors.Subheading)
		str += widget.displayMyReviewRequests(repo)
	}
	if widget.settings.showMyPullRequests {
		str += fmt.Sprintf("\n [%s]My Pull Requests[white]\n", widget.settings.Colors.Subheading)
		str += widget.displayMyPullRequests(repo)
	}
	for _, customQuery := range widget.settings.customQueries {
		str += fmt.Sprintf("\n [%s]%s[white]\n", widget.settings.Colors.Subheading, customQuery.title)
		str += widget.displayCustomQuery(repo, customQuery.filter)
	}

	return title, str, false
}

func (widget *Widget) displayMyPullRequests(repo *Repo) string {
	prs := repo.CachedMyPullRequests

	prLength := len(prs)

	if prLength == 0 {
		return " [grey]none[white]\n"
	}

	maxItems := widget.GetItemCount()

	str := ""
	for idx, pr := range prs {
		str += fmt.Sprintf(` %s%s%s[green]["%d"]%4d[""][white] %s`, widget.mergeString(pr), widget.reviewString(repo, pr), widget.draftString(pr), maxItems+idx, *pr.Number, *pr.Title)
		str += "\n"
		widget.Items = append(widget.Items, *pr.Number)
	}

	widget.SetItemCount(maxItems + prLength)

	return str
}

func (widget *Widget) displayCustomQuery(repo *Repo, filter string) string {
	res := repo.CachedCustomQueries[filter]

	if res == nil {
		return " [grey]Invalid Query[white]\n"
	}

	issuesLength := len(res.Issues)

	if issuesLength == 0 {
		return " [grey]none[white]\n"
	}

	maxItems := widget.GetItemCount()

	str := ""
	for idx, issue := range res.Issues {
		str += fmt.Sprintf(` [green]["%d"]%4d[""][white] %s`, maxItems+idx, *issue.Number, *issue.Title)
		str += "\n"
		widget.Items = append(widget.Items, *issue.Number)
	}

	widget.SetItemCount(maxItems + issuesLength)

	return str
}

func (widget *Widget) displayMyReviewRequests(repo *Repo) string {
	prs := repo.CachedMyReviewRequests

	if len(prs) == 0 {
		return " [grey]none[white]\n"
	}

	str := ""
	for idx, pr := range prs {
		str += fmt.Sprintf(` %s%s[green]["%d"]%4d[""][white] %s`, widget.reviewString(repo, pr), widget.draftString(pr), idx, *pr.Number, *pr.Title)
		str += "\n"
		widget.Items = append(widget.Items, *pr.Number)
	}

	return str
}

func (widget *Widget) displayStats(repo *Repo) string {
	locPrinter, err := widget.settings.LocalizedPrinter()
	if err != nil {
		return err.Error()
	}

	str := fmt.Sprintf(
		" PRs: %s  Issues: %s  Stars: %s\n",
		locPrinter.Sprintf("%d", repo.PullRequestCount()),
		locPrinter.Sprintf("%d", repo.IssueCount()),
		locPrinter.Sprintf("%d", repo.StarCount()),
	)

	return str
}

func (widget *Widget) title(repo *Repo) string {
	return fmt.Sprintf(
		"[%s]%s - %s[white]",
		widget.settings.Colors.Title,
		repo.Owner,
		repo.Name,
	)
}

var mergeIcons = map[string]string{
	"dirty":    "[red]\u0021[white] ",
	"clean":    "[green]\u2713[white] ",
	"unstable": "[red]\u2717[white] ",
	"blocked":  "[red]\u2717[white] ",
}

// Review status icons. Text icons are single-width so they can never disturb
// column alignment; emoji are wider and render better on terminals with good
// emoji support. Each entry is padded to occupy the same width as its peers.
var reviewIconSets = map[string]map[string]string{
	"text": {
		ReviewApproved:         "[green]\u2713[white] ",
		ReviewChangesRequested: "[red]\u2717[white] ",
		ReviewRequired:         "[yellow]\u25cf[white] ",
		ReviewCommented:        "[blue]\u25cb[white] ",
		ReviewNone:             "  ",
	},
	"emoji": {
		ReviewApproved:         "\u2705 ",
		ReviewChangesRequested: "\u274c ",
		ReviewRequired:         "\u231a ",
		ReviewCommented:        "\U0001f4ac ",
		ReviewNone:             "   ",
	},
}

// reviewString renders a pull request's review status, or an equal-width blank
// so that titles stay aligned down the column.
func (widget *Widget) reviewString(repo *Repo, pr *ghb.PullRequest) string {
	if !widget.settings.showReviewStatus {
		return ""
	}

	icons, ok := reviewIconSets[widget.settings.reviewIcons]
	if !ok {
		icons = reviewIconSets["emoji"]
	}

	state := repo.CachedReviewStates[pr.GetNumber()]
	if icon, ok := icons[state]; ok {
		return icon
	}

	return icons[ReviewNone]
}

// Keys for draftIconSets. GitHub exposes draft as a plain boolean on the pull
// request itself, so unlike review state there is nothing to compute.
const (
	draftYes = "draft"
	draftNo  = "ready"
)

// Draft marker icons, a pair per icon set. Both states carry a glyph, so the
// pair has to be equal width or every column after it would step sideways as
// rows change state; TestDraftMarkerWidthsMatch pins that down. Width here
// means what tview.TaggedStringWidth reports, because that is what tview lays
// out with -- worth stating since the draft emoji needs its variation selector
// (U+FE0F) to measure its full 2, where a raw runewidth call returns 1, and
// since the text glyphs carry colour tags that are markup rather than content.
var draftIconSets = map[string]map[string]string{
	"text": {
		draftYes: "[grey]\u270e[white] ",
		draftNo:  "[grey]\u25c6[white] ",
	},
	"emoji": {
		draftYes: "\U0001f32c\ufe0f ",
		draftNo:  "\U0001f381 ",
	},
}

// draftString renders a pull request's draft state, drafted or ready, at equal
// width either way so that titles stay aligned down the column.
func (widget *Widget) draftString(pr *ghb.PullRequest) string {
	if !widget.settings.showDraftStatus {
		return ""
	}

	icons, ok := draftIconSets[widget.settings.reviewIcons]
	if !ok {
		icons = draftIconSets["emoji"]
	}

	if pr.GetDraft() {
		return icons[draftYes]
	}

	return icons[draftNo]
}

func (widget *Widget) mergeString(pr *ghb.PullRequest) string {
	if !widget.settings.enableStatus {
		return ""
	}
	if str, ok := mergeIcons[pr.GetMergeableState()]; ok {
		return str
	}
	return "? "
}
