package builder

import (
	"bytes"
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

type cursorPrompt struct {
	Text string `json:"text"`
}

type cursorRepo struct {
	URL string `json:"url"`
}

type cursorCreate struct {
	Prompt       cursorPrompt `json:"prompt"`
	Repos        []cursorRepo `json:"repos"`
	AutoCreatePR bool         `json:"autoCreatePR"`
	AgentID      string       `json:"agentId"`
}

type cursorRun struct {
	ID string `json:"id"`
}

type cursorCreateResponse struct {
	Run cursorRun `json:"run"`
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

// create posts one agent. A transport failure returns status 0.
// A response status outside 2xx returns that status and an error.
func (client cursorClient) create(ctx context.Context, body cursorCreate) (runID string, status int, err error) {
	payload, err := json.Marshal(body)
	if err != nil {
		log.Error("cursor create", "err", err)
		return
	}
	url := client.base + "/v1/agents"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.Error("cursor create", "err", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(client.key, "")

	response, err := client.http.Do(request)
	if err != nil {
		log.Error("cursor create", "err", err)
		return
	}
	defer response.Body.Close()
	status = response.StatusCode
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error("cursor create", "err", err)
		return
	}
	accepted := status >= 200 && status < 300
	if !accepted {
		err = fmt.Errorf("cursor create returned %d", status)
		log.Error("cursor create", "status", status, "err", err)
		return
	}
	var parsed cursorCreateResponse
	err = json.Unmarshal(raw, &parsed)
	if err != nil {
		log.Error("cursor create", "err", err)
		return
	}
	runID = parsed.Run.ID
	missing := runID == ""
	if missing {
		err = fmt.Errorf("cursor run id is missing")
		log.Error("cursor create", "err", err)
	}
	return
}
