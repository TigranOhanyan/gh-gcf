package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/log/v2"
)

const cursorAPIEnv = "CURSOR_API_URL"
const cursorKeyEnv = "CURSOR_API_KEY"
const defaultCursorAPI = "https://api.cursor.com"

const runCreating = "CREATING"
const runRunning = "RUNNING"
const runFinished = "FINISHED"
const runError = "ERROR"
const runCancelled = "CANCELLED"
const runExpired = "EXPIRED"

type cursorBranch struct {
	RepoURL string `json:"repoUrl"`
	PRURL   string `json:"prUrl"`
}

type cursorGit struct {
	Branches []cursorBranch `json:"branches"`
}

type cursorRun struct {
	ID     string    `json:"id"`
	Status string    `json:"status"`
	Git    cursorGit `json:"git"`
}

type cursorClient struct {
	base string
	key  string
	http *http.Client
}

func newCursorClient() (client cursorClient, err error) {
	key := os.Getenv(cursorKeyEnv)
	missing := key == ""
	if missing {
		err = fmt.Errorf("cursor api key is missing")
		log.Error("cursor", "err", err)
		return
	}
	base := os.Getenv(cursorAPIEnv)
	if base == "" {
		base = defaultCursorAPI
	}
	base = strings.TrimRight(base, "/")
	client.base = base
	client.key = key
	client.http = &http.Client{Timeout: 30 * time.Second}
	return
}

// getRun reads one run. A transport failure returns status 0.
func (client cursorClient) getRun(ctx context.Context, agentID string, runID string) (run cursorRun, status int, err error) {
	url := client.base + "/v1/agents/" + agentID + "/runs/" + runID
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error("cursor run", "agent", agentID, "err", err)
		return
	}
	request.SetBasicAuth(client.key, "")

	response, err := client.http.Do(request)
	if err != nil {
		log.Error("cursor run", "agent", agentID, "err", err)
		return
	}
	defer response.Body.Close()
	status = response.StatusCode
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error("cursor run", "agent", agentID, "err", err)
		return
	}
	success := status == http.StatusOK
	if !success {
		err = fmt.Errorf("cursor run returned %d", status)
		log.Error("cursor run", "agent", agentID, "status", status, "body", string(raw), "err", err)
		return
	}
	err = json.Unmarshal(raw, &run)
	if err != nil {
		log.Error("cursor run", "agent", agentID, "err", err)
		return
	}
	missing := run.Status == ""
	if missing {
		err = fmt.Errorf("cursor run status is missing")
		log.Error("cursor run", "agent", agentID, "err", err)
	}
	return
}
