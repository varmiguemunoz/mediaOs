package tiktok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/varmiguemunoz/content-automation/internal/config"
)

const apiBase = "https://open.tiktokapis.com/v2"

type Client struct {
	accessToken string
	openID      string
	httpClient  *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		accessToken: cfg.TikTokAccessToken,
		openID:      cfg.TikTokOpenID,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

type InitUploadRequest struct {
	PostInfo    PostInfo    `json:"post_info"`
	SourceInfo  SourceInfo  `json:"source_info"`
}

type PostInfo struct {
	Title        string `json:"title"`
	PrivacyLevel string `json:"privacy_level"`
	DisableDuet  bool   `json:"disable_duet"`
	DisableStitch bool  `json:"disable_stitch"`
	DisableComment bool `json:"disable_comment"`
	VideoCategory string `json:"video_category"`
}

type SourceInfo struct {
	Source         string `json:"source"`
	VideoURL       string `json:"video_url"`
	ChunkSize      int    `json:"chunk_size"`
	TotalChunkCount int   `json:"total_chunk_count"`
}

type InitUploadResponse struct {
	Data struct {
		PublishID    string `json:"publish_id"`
		UploadURL    string `json:"upload_url"`
	} `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type StatusResponse struct {
	Data struct {
		PublishID string `json:"publish_id"`
		Status    string `json:"status"`
	} `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) PostVideo(videoURL, caption string) (string, error) {
	if c.accessToken == "" {
		return "", fmt.Errorf("TIKTOK_ACCESS_TOKEN es requerido")
	}

	payload := InitUploadRequest{
		PostInfo: PostInfo{
			Title:         caption,
			PrivacyLevel:  "PUBLIC_TO_EVERYONE",
			DisableDuet:   false,
			DisableStitch: false,
			DisableComment: false,
		},
		SourceInfo: SourceInfo{
			Source:          "PULL_FROM_URL",
			VideoURL:        videoURL,
			ChunkSize:       0,
			TotalChunkCount: 0,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiBase+"/post/publish/video/init/", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request a TikTok: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result InitUploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response TikTok: %w — body: %s", err, string(respBody))
	}

	if result.Error.Code != "" && result.Error.Code != "ok" {
		return "", fmt.Errorf("TikTok API error: %s — %s", result.Error.Code, result.Error.Message)
	}

	if result.Data.PublishID == "" {
		return "", fmt.Errorf("TikTok no devolvió publish_id — body: %s", string(respBody))
	}

	return result.Data.PublishID, nil
}
