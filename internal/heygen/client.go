package heygen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/varmiguemunoz/content-automation/internal/config"
)

const baseURL = "https://api.heygen.com"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		apiKey: cfg.HeyGenAPIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type AvatarGroup struct {
	AvatarGroupID string `json:"avatar_group_id"`
	AvatarName    string `json:"avatar_name"`
	Avatars       []struct {
		AvatarID string `json:"avatar_id"`
		Gender   string `json:"gender"`
	} `json:"avatars"`
}

type ListAvatarGroupsResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		AvatarGroupList []AvatarGroup `json:"avatar_group_list"`
	} `json:"data"`
}

type VideoCreateRequest struct {
	VideoInputs []VideoInput `json:"video_inputs"`
	AspectRatio string       `json:"aspect_ratio"`
	Test        bool         `json:"test"`
}

type VideoInput struct {
	Character Character `json:"character"`
	Voice     Voice     `json:"voice"`
	Background Background `json:"background"`
}

type Character struct {
	Type     string `json:"type"`
	AvatarID string `json:"avatar_id"`
	AvatarStyle string `json:"avatar_style"`
}

type Voice struct {
	Type    string `json:"type"`
	VoiceID string `json:"voice_id"`
	InputText string `json:"input_text"`
	Speed   float64 `json:"speed"`
}

type Background struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type VideoCreateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		VideoID string `json:"video_id"`
	} `json:"data"`
}

type VideoStatusResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		VideoID  string `json:"video_id"`
		Status   string `json:"status"`
		VideoURL string `json:"video_url"`
		Error    string `json:"error"`
	} `json:"data"`
}

func (c *Client) ListAvatarGroups() ([]AvatarGroup, error) {
	req, err := http.NewRequest("GET", baseURL+"/v2/avatars", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("HeyGen API key inválida o sin permisos")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result ListAvatarGroupsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return result.Data.AvatarGroupList, nil
}

func (c *Client) CreateVideo(avatarID, voiceID, script string) (string, error) {
	payload := VideoCreateRequest{
		VideoInputs: []VideoInput{
			{
				Character: Character{
					Type:        "avatar",
					AvatarID:    avatarID,
					AvatarStyle: "normal",
				},
				Voice: Voice{
					Type:      "text",
					VoiceID:   voiceID,
					InputText: script,
					Speed:     1.0,
				},
				Background: Background{
					Type:  "color",
					Value: "#FAFAFA",
				},
			},
		},
		AspectRatio: "9:16",
		Test:        false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var videoID string
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		req, err := http.NewRequest("POST", baseURL+"/v2/video/generate", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("X-Api-Key", c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request: %w", err)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 401 {
			return "", fmt.Errorf("HeyGen API key inválida")
		}
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("rate limit de HeyGen")
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("error de servidor HeyGen: %d", resp.StatusCode)
			continue
		}

		var result VideoCreateResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			lastErr = fmt.Errorf("parsing response: %w", err)
			continue
		}

		if result.Data.VideoID == "" {
			lastErr = fmt.Errorf("HeyGen no devolvió video_id: %s", string(respBody))
			continue
		}

		videoID = result.Data.VideoID
		break
	}

	if videoID == "" {
		return "", lastErr
	}
	return videoID, nil
}

func (c *Client) GetVideoStatus(videoID string) (status, videoURL string, err error) {
	req, err := http.NewRequest("GET", baseURL+"/v1/video_status.get?video_id="+videoID, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result VideoStatusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parsing: %w", err)
	}

	return result.Data.Status, result.Data.VideoURL, nil
}
