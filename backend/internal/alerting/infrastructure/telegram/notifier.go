package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

const (
	defaultBaseURL = "https://api.telegram.org"
	maxErrorBody   = 4 << 10
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Notifier struct {
	botToken string
	chatID   string
	baseURL  string
	client   httpClient
}

type sendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func NewNotifier(botToken, chatID string, timeout time.Duration) (*Notifier, error) {
	botToken = strings.TrimSpace(botToken)
	chatID = strings.TrimSpace(chatID)

	if botToken == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}
	if chatID == "" {
		return nil, fmt.Errorf("telegram chat ID is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Notifier{
		botToken: botToken,
		chatID:   chatID,
		baseURL:  defaultBaseURL,
		client:   &http.Client{Timeout: timeout},
	}, nil
}

func (n *Notifier) Notify(ctx context.Context, alert sharedDomain.AlertEvent) error {
	payload := sendMessageRequest{
		ChatID: n.chatID,
		Text:   formatAlert(alert),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram message: %w", err)
	}

	endpoint := fmt.Sprintf(
		"%s/bot%s/sendMessage",
		strings.TrimRight(n.baseURL, "/"),
		n.botToken,
	)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}

	var result apiResponse
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return fmt.Errorf(
				"decode telegram response: status=%d: %w",
				resp.StatusCode,
				err,
			)
		}
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices ||
		!result.OK {
		description := strings.TrimSpace(result.Description)
		if description == "" {
			description = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf(
			"telegram rejected message: status=%d description=%q",
			resp.StatusCode,
			description,
		)
	}

	return nil
}

func formatAlert(alert sharedDomain.AlertEvent) string {
	return fmt.Sprintf(
		"🚨 %s ALERT\n\nApplication: %s\nMessage: %s\nTrace ID: %s\nEvent ID: %s\nTime: %s",
		alert.Level,
		alert.ApplicationName,
		alert.Message,
		alert.TraceID,
		alert.EventID,
		alert.Timestamp.Format(time.RFC3339),
	)
}
