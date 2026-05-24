package meta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/varmiguemunoz/content-automation/internal/config"
)

const graphAPIBase = "https://graph.facebook.com/v19.0"

type Client struct {
	pageID          string
	pageAccessToken string
	igUserID        string
	httpClient      *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		pageID:          cfg.MetaPageID,
		pageAccessToken: cfg.MetaPageAccessToken,
		igUserID:        cfg.MetaIGUserID,
		httpClient:      &http.Client{Timeout: 60 * time.Second},
	}
}

type graphResponse struct {
	ID    string `json:"id"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (c *Client) PostInstagramReel(videoURL, caption string) (string, error) {
	if c.igUserID == "" || c.pageAccessToken == "" {
		return "", fmt.Errorf("META_IG_USER_ID y META_PAGE_ACCESS_TOKEN son requeridos")
	}

	containerID, err := c.createIGMediaContainer(videoURL, caption)
	if err != nil {
		return "", fmt.Errorf("creando container de Instagram: %w", err)
	}

	for i := 0; i < 12; i++ {
		time.Sleep(10 * time.Second)
		ready, err := c.checkIGContainerStatus(containerID)
		if err != nil {
			return "", err
		}
		if ready {
			break
		}
		if i == 11 {
			return "", fmt.Errorf("timeout esperando que el container de Instagram esté listo")
		}
	}

	postID, err := c.publishIGContainer(containerID)
	if err != nil {
		return "", fmt.Errorf("publicando en Instagram: %w", err)
	}

	return postID, nil
}

func (c *Client) createIGMediaContainer(videoURL, caption string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/reels", graphAPIBase, c.igUserID)

	params := url.Values{}
	params.Set("video_url", videoURL)
	params.Set("caption", caption)
	params.Set("media_type", "REELS")
	params.Set("access_token", c.pageAccessToken)

	resp, err := c.httpClient.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return c.parseIDResponse(resp)
}

func (c *Client) checkIGContainerStatus(containerID string) (bool, error) {
	endpoint := fmt.Sprintf("%s/%s?fields=status_code&access_token=%s", graphAPIBase, containerID, c.pageAccessToken)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		StatusCode string `json:"status_code"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	if result.Error != nil {
		return false, fmt.Errorf("error de Instagram: %s", result.Error.Message)
	}
	return result.StatusCode == "FINISHED", nil
}

func (c *Client) publishIGContainer(containerID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/media_publish", graphAPIBase, c.igUserID)

	params := url.Values{}
	params.Set("creation_id", containerID)
	params.Set("access_token", c.pageAccessToken)

	resp, err := c.httpClient.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return c.parseIDResponse(resp)
}

func (c *Client) PostFacebookVideo(videoURL, description string) (string, error) {
	if c.pageID == "" || c.pageAccessToken == "" {
		return "", fmt.Errorf("META_PAGE_ID y META_PAGE_ACCESS_TOKEN son requeridos")
	}

	endpoint := fmt.Sprintf("%s/%s/videos", graphAPIBase, c.pageID)

	params := url.Values{}
	params.Set("file_url", videoURL)
	params.Set("description", description)
	params.Set("access_token", c.pageAccessToken)

	resp, err := c.httpClient.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return c.parseIDResponse(resp)
}

func (c *Client) parseIDResponse(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result graphResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w — body: %s", err, string(body))
	}

	if result.Error != nil {
		return "", fmt.Errorf("Meta API error %d: %s", result.Error.Code, result.Error.Message)
	}

	if result.ID == "" {
		return "", fmt.Errorf("Meta API no devolvió ID — body: %s", string(body))
	}

	return result.ID, nil
}
