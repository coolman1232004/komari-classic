package update

import (
	"net/http"
	"testing"
)

type rejectNetwork struct{ t *testing.T }

func (r rejectNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Fatal("pinned agent must not contact an update service")
	return nil, nil
}

func TestPinnedAgentDoesNotFetchUpdates(t *testing.T) {
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: rejectNetwork{t}}
	defer func() { http.DefaultClient = previous }()
	DoUpdateWorks()
	if err := CheckAndUpdate(); err != nil {
		t.Fatal(err)
	}
}
