package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
)

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

var versionsCache = struct {
	sync.Mutex
	versions  []string
	fetchedAt time.Time
}{}

const (
	ghReleasesURL    = "https://api.github.com/repos/stroppy-io/stroppy/releases?per_page=30"
	versionsCacheTTL = 5 * time.Minute
)

var fallbackVersions = []string{"v3.0.0"}

func (h *Handler) ListStroppyVersions(
	ctx context.Context,
	_ *connect.Request[pb.ListStroppyVersionsRequest],
) (*connect.Response[pb.ListStroppyVersionsResponse], error) {
	versions, err := fetchStroppyVersions(ctx)
	if err != nil || len(versions) == 0 {
		versions = fallbackVersions
	}
	return connect.NewResponse(&pb.ListStroppyVersionsResponse{
		Versions: versions,
	}), nil
}

func fetchStroppyVersions(_ context.Context) ([]string, error) {
	versionsCache.Lock()
	defer versionsCache.Unlock()

	if time.Since(versionsCache.fetchedAt) < versionsCacheTTL && len(versionsCache.versions) > 0 {
		return versionsCache.versions, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", ghReleasesURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned %s", resp.Status)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	var versions []string
	for _, r := range releases {
		if !r.Draft && r.TagName != "" {
			versions = append(versions, r.TagName)
		}
	}

	versionsCache.versions = versions
	versionsCache.fetchedAt = time.Now()
	return versions, nil
}
