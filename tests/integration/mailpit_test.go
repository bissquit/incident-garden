//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// MailpitClient provides access to Mailpit REST API for testing.
type MailpitClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMailpitClient creates a new Mailpit API client.
func NewMailpitClient(host string, port int) *MailpitClient {
	return &MailpitClient{
		baseURL:    fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// MailpitMessage represents an email message in Mailpit.
type MailpitMessage struct {
	ID      string           `json:"ID"`
	From    MailpitAddress   `json:"From"`
	To      []MailpitAddress `json:"To"`
	Cc      []MailpitAddress `json:"Cc"`
	Bcc     []MailpitAddress `json:"Bcc"`
	Subject string           `json:"Subject"`
	Snippet string           `json:"Snippet"`
	Text    string           // populated by GetMessageByID
	HTML    string           // populated by GetMessageByID
}

// MailpitAddress represents an email address.
type MailpitAddress struct {
	Address string `json:"Address"`
	Name    string `json:"Name"`
}

// AllRecipients returns all recipients (To, Cc, Bcc) of a message.
// This is useful when emails are sent using BCC (which is how our sender works).
func (m *MailpitMessage) AllRecipients() []MailpitAddress {
	result := make([]MailpitAddress, 0, len(m.To)+len(m.Cc)+len(m.Bcc))
	result = append(result, m.To...)
	result = append(result, m.Cc...)
	result = append(result, m.Bcc...)
	return result
}

type messagesResponse struct {
	Messages []MailpitMessage `json:"messages"`
	Total    int              `json:"messages_count"`
}

// GetMessages returns all messages in the inbox.
func (c *MailpitClient) GetMessages() ([]MailpitMessage, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/messages")
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages: status %d: %s", resp.StatusCode, body)
	}

	var result messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	return result.Messages, nil
}

// GetMessageByID returns a single message with full body content.
func (c *MailpitClient) GetMessageByID(id string) (*MailpitMessage, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/message/" + id)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get message: status %d", resp.StatusCode)
	}

	var msg MailpitMessage
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}

	// Fetch plain text body
	textResp, err := c.httpClient.Get(c.baseURL + "/api/v1/message/" + id + "/part/0")
	if err == nil {
		defer textResp.Body.Close()
		if textResp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(textResp.Body)
			msg.Text = string(body)
		}
	}

	return &msg, nil
}

// DeleteAllMessages clears the inbox.
func (c *MailpitClient) DeleteAllMessages() error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/v1/messages", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete messages: status %d", resp.StatusCode)
	}
	return nil
}

// WaitForMessages waits until at least count messages are received.
// Returns error on timeout.
func (c *MailpitClient) WaitForMessages(count int, timeout time.Duration) ([]MailpitMessage, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		messages, err := c.GetMessages()
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if len(messages) >= count {
			return messages, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	messages, _ := c.GetMessages()
	if lastErr != nil {
		return messages, fmt.Errorf("timeout waiting for %d messages (got %d): %w", count, len(messages), lastErr)
	}
	return messages, fmt.Errorf("timeout waiting for %d messages, got %d", count, len(messages))
}

// SearchByRecipient searches messages by recipient email address.
func (c *MailpitClient) SearchByRecipient(email string) ([]MailpitMessage, error) {
	query := url.QueryEscape("to:" + email)
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/search?query=" + query)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search messages: status %d", resp.StatusCode)
	}

	var result messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search results: %w", err)
	}
	return result.Messages, nil
}

// MessageCount returns current number of messages in inbox.
func (c *MailpitClient) MessageCount() (int, error) {
	messages, err := c.GetMessages()
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}
