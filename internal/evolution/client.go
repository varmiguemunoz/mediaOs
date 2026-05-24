package evolution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/varmiguemunoz/content-automation/internal/config"
)

type Client struct {
	baseURL    string
	apiKey     string
	instance   string
	httpClient *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		baseURL:  strings.TrimRight(cfg.EvolutionBaseURL, "/"),
		apiKey:   cfg.EvolutionAPIKey,
		instance: cfg.EvolutionInstance,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type SendTextRequest struct {
	Number  string `json:"number"`
	Text    string `json:"text"`
	Options struct {
		Delay    int  `json:"delay"`
		Presence bool `json:"presence"`
	} `json:"options"`
}

type Message struct {
	Key struct {
		RemoteJID string `json:"remoteJid"`
		FromMe    bool   `json:"fromMe"`
		ID        string `json:"id"`
	} `json:"key"`
	Message struct {
		Conversation string `json:"conversation"`
		ExtendedText struct {
			Text string `json:"text"`
		} `json:"extendedTextMessage"`
	} `json:"message"`
	MessageTimestamp int64 `json:"messageTimestamp"`
}

type FindMessagesResponse struct {
	Messages struct {
		Records []Message `json:"records"`
	} `json:"messages"`
}

func (c *Client) SendText(number, text string) error {
	payload := SendTextRequest{}
	payload.Number = number
	payload.Text = text
	payload.Options.Delay = 1000
	payload.Options.Presence = false

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/message/sendText/%s", c.baseURL, c.instance)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("enviando mensaje: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("EvolutionAPI error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *Client) FindRecentMessagesFrom(number string, since time.Time) ([]Message, error) {
	url := fmt.Sprintf("%s/chat/findMessages/%s", c.baseURL, c.instance)

	payload := map[string]interface{}{
		"where": map[string]interface{}{
			"key": map[string]interface{}{
				"remoteJid": number + "@s.whatsapp.net",
				"fromMe":    false,
			},
			"messageTimestamp": map[string]interface{}{
				"$gte": since.Unix(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("buscando mensajes: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("EvolutionAPI error %d: %s", resp.StatusCode, string(respBody))
	}

	var result FindMessagesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing mensajes: %w", err)
	}

	return result.Messages.Records, nil
}

func (c *Client) ExtractApprovalFromMessages(messages []Message) (action string, planID int64) {
	for _, msg := range messages {
		text := msg.Message.Conversation
		if text == "" {
			text = msg.Message.ExtendedText.Text
		}
		text = strings.ToUpper(strings.TrimSpace(text))

		if strings.HasPrefix(text, "APPROVE ") {
			var id int64
			_, err := fmt.Sscanf(text, "APPROVE %d", &id)
			if err == nil {
				return "approve", id
			}
		}
		if strings.HasPrefix(text, "REJECT ") {
			var id int64
			_, err := fmt.Sscanf(text, "REJECT %d", &id)
			if err == nil {
				return "reject", id
			}
		}
	}
	return "", 0
}
