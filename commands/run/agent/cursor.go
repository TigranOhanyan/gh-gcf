package agent

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

// CursorPrompt is the text sent to one Cursor agent.
type CursorPrompt struct {
	Text string `json:"text"`
}

// CursorRepo is one repo the agent may edit.
type CursorRepo struct {
	URL string `json:"url"`
}

// CursorCreate is the body that starts one Cursor agent.
// An empty AgentID is omitted. Cursor refuses a body that sends both agentId and envVars.
type CursorCreate struct {
	Prompt       CursorPrompt      `json:"prompt"`
	Repos        []CursorRepo      `json:"repos"`
	AutoCreatePR bool              `json:"autoCreatePR"`
	AgentID      string            `json:"agentId,omitempty"`
	EnvVars      map[string]string `json:"envVars,omitempty"`
}

type cursorRun struct {
	ID string `json:"id"`
}

type cursorAgent struct {
	ID string `json:"id"`
}

type cursorCreateResponse struct {
	Agent cursorAgent `json:"agent"`
	Run   cursorRun   `json:"run"`
}

// CursorClient posts agent creates to the Cursor API.
type CursorClient struct {
	base string
	key  string
	http *http.Client
}

// NewCursorClient reads the Cursor API key and optional base URL.
func NewCursorClient() (client CursorClient, err error) {
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

// Create posts one agent. A transport failure returns status 0.
// 200, 201, and 204 are success. Any other status returns the response body.
// A success body with a missing agent id or a missing run id is an error.
func (client CursorClient) Create(ctx context.Context, body CursorCreate) (agentID string, runID string, responseBody string, status int, err error) {
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
	responseBody = string(raw)
	success := status == http.StatusOK || status == http.StatusCreated || status == http.StatusNoContent
	if !success {
		err = fmt.Errorf("cursor create returned %d", status)
		log.Error("cursor create", "status", status, "body", responseBody, "err", err)
		return
	}
	empty := status == http.StatusNoContent
	if empty {
		return
	}
	var parsed cursorCreateResponse
	err = json.Unmarshal(raw, &parsed)
	if err != nil {
		log.Error("cursor create", "err", err)
		return
	}
	agentID = parsed.Agent.ID
	runID = parsed.Run.ID
	missingRun := runID == ""
	if missingRun {
		err = fmt.Errorf("cursor run id is missing")
		log.Error("cursor create", "err", err)
		return
	}
	missingAgent := agentID == ""
	if missingAgent {
		err = fmt.Errorf("cursor agent id is missing")
		log.Error("cursor create", "err", err)
	}
	return
}
