package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ghb "github.com/google/go-github/v89/github"
	"gotest.tools/assert"
)

// reviewsServer serves the ListReviews endpoint for a single pull request.
// Each entry of reviews is a "login:STATE" pair, oldest first.
func reviewsServer(t *testing.T, prNumber int, reviews []string) *httptest.Server {
	t.Helper()

	body := make([]string, 0, len(reviews))
	for i, r := range reviews {
		login, state, _ := strings.Cut(r, ":")
		body = append(body, fmt.Sprintf(
			`{"id":%d,"state":%q,"user":{"login":%q}}`, i+1, state, login))
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v3/repos/acme/widgets/pulls/%d/reviews", prNumber),
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "[%s]", strings.Join(body, ","))
		})

	return httptest.NewServer(mux)
}

func stateFor(t *testing.T, reviews []string, awaitingReviewers bool) string {
	t.Helper()

	const number = 42
	server := reviewsServer(t, number, reviews)
	defer server.Close()

	repo := NewGithubRepo("widgets", "acme", "token", server.URL+"/api/v3/", server.URL+"/api/v3/")
	pr := &ghb.PullRequest{Number: ghb.Ptr(number)}
	if awaitingReviewers {
		pr.RequestedReviewers = []*ghb.User{{Login: ghb.Ptr("zoe")}}
	}

	return repo.computeReviewStates(context.Background(), []*ghb.PullRequest{pr})[number]
}

func TestComputeReviewStates(t *testing.T) {
	tests := []struct {
		name              string
		reviews           []string
		awaitingReviewers bool
		want              string
	}{
		{"single approval", []string{"ann:APPROVED"}, false, ReviewApproved},
		{"nothing at all", []string{}, false, ReviewNone},
		{"reviewer requested, no verdict yet", []string{}, true, ReviewRequired},
		{"comment only", []string{"ann:COMMENTED"}, false, ReviewCommented},
		{"changes requested", []string{"ann:CHANGES_REQUESTED"}, false, ReviewChangesRequested},
		{"changes requested outranks another approval",
			[]string{"ann:APPROVED", "bob:CHANGES_REQUESTED"}, false, ReviewChangesRequested},
		{"approval outranks a pending reviewer",
			[]string{"ann:APPROVED"}, true, ReviewApproved},
		{"later approval supersedes earlier changes request",
			[]string{"ann:CHANGES_REQUESTED", "ann:APPROVED"}, false, ReviewApproved},
		{"later changes request supersedes earlier approval",
			[]string{"ann:APPROVED", "ann:CHANGES_REQUESTED"}, false, ReviewChangesRequested},
		{"comment after approval leaves it standing",
			[]string{"ann:APPROVED", "ann:COMMENTED"}, false, ReviewApproved},
		{"dismissal clears an approval",
			[]string{"ann:APPROVED", "ann:DISMISSED"}, false, ReviewNone},
		{"dismissal falls back to pending reviewer",
			[]string{"ann:APPROVED", "ann:DISMISSED"}, true, ReviewRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stateFor(t, tt.reviews, tt.awaitingReviewers))
		})
	}
}

func TestReviewIconSetsCoverEveryState(t *testing.T) {
	states := []string{
		ReviewApproved, ReviewChangesRequested, ReviewRequired,
		ReviewCommented, ReviewNone,
	}

	for name, icons := range reviewIconSets {
		assert.Equal(t, len(states), len(icons), "icon set %q has an unexpected number of entries", name)
		for _, state := range states {
			_, ok := icons[state]
			assert.Assert(t, ok, "icon set %q is missing state %q", name, state)
		}
	}
}
