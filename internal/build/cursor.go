package build

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type cursorAPI struct {
	client   *http.Client
	endpoint string
}

func newCursor(deps Deps) *cursorAPI {
	client := deps.HTTP
	if client == nil {
		client = &http.Client{Timeout: time.Minute}
	}
	return &cursorAPI{client: client, endpoint: endpoint(deps)}
}

type agentRequest struct {
	Prompt       agentPrompt `json:"prompt"`
	Repos        []agentRepo `json:"repos"`
	AutoCreatePR bool        `json:"autoCreatePR"`
	Name         string      `json:"name"`
	Model        *agentModel `json:"model,omitempty"`
}

type agentPrompt struct {
	Text string `json:"text"`
}

type agentRepo struct {
	URL         string `json:"url"`
	StartingRef string `json:"startingRef"`
}

type agentModel struct {
	ID string `json:"id"`
}

type agentResponse struct {
	Agent struct {
		URL string `json:"url"`
	} `json:"agent"`
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	if e.Status > 0 {
		return formatFailure(e.Status, e.Message)
	}
	return e.Message
}

func (c *cursorAPI) create(ctx context.Context, key string, payload agentRequest) (string, error) {
	key = strings.TrimSpace(key)
	body, err := encodeJSON(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	// The key is sent only in this header. Do not copy it into errors or logs.
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gh-gcf")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", &apiError{Message: redact(err.Error(), key)}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", &apiError{Status: resp.StatusCode, Message: redact(err.Error(), key)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &apiError{Status: resp.StatusCode, Message: cursorErrorMessage(respBody)}
	}
	var parsed agentResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil || !validAgentURL(parsed.Agent.URL) {
		return "", &apiError{Status: resp.StatusCode, Message: "response did not include an agent URL"}
	}
	return parsed.Agent.URL, nil
}

// cursorErrorMessage reads either the wrapped Error schema or the flat 401 body.
func cursorErrorMessage(body []byte) string {
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &wrapped) == nil && wrapped.Error.Message != "" {
		return wrapped.Error.Message
	}
	var flat struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &flat) == nil && flat.Message != "" {
		return flat.Message
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "request failed"
	}
	return truncate(text, maxErrorLen)
}

func validAgentURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func encodeJSONIndent(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
