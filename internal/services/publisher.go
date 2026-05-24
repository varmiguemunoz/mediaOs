package services

import (
	"fmt"
	"time"

	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/meta"
	"github.com/varmiguemunoz/content-automation/internal/tiktok"
)

type Publisher struct {
	cfg      *config.Config
	database *db.DB
}

func NewPublisher(cfg *config.Config, database *db.DB) *Publisher {
	return &Publisher{cfg: cfg, database: database}
}

type PublishResult struct {
	Platform string
	PostID   string
	Err      error
}

func (p *Publisher) Publish(plan *db.ContentPlan, script *db.GeneratedScript, videoURL string) error {
	results := []PublishResult{}

	metaClient := meta.New(p.cfg)
	tikTokClient := tiktok.New(p.cfg)

	if p.cfg.MetaIGUserID != "" && p.cfg.MetaPageAccessToken != "" {
		fmt.Print("  📱 Publicando en Instagram... ")
		postID, err := metaClient.PostInstagramReel(videoURL, script.CaptionInstagram)
		results = append(results, PublishResult{Platform: "instagram", PostID: postID, Err: err})
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ (ID: %s)\n", postID)
		}
	}

	if p.cfg.MetaPageID != "" && p.cfg.MetaPageAccessToken != "" {
		fmt.Print("  📘 Publicando en Facebook... ")
		postID, err := metaClient.PostFacebookVideo(videoURL, script.CaptionFacebook)
		results = append(results, PublishResult{Platform: "facebook", PostID: postID, Err: err})
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ (ID: %s)\n", postID)
		}
	}

	if p.cfg.TikTokAccessToken != "" {
		fmt.Print("  🎵 Publicando en TikTok... ")
		postID, err := tikTokClient.PostVideo(videoURL, script.CaptionTikTok)
		results = append(results, PublishResult{Platform: "tiktok", PostID: postID, Err: err})
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ (ID: %s)\n", postID)
		}
	}

	now := time.Now()
	for _, r := range results {
		errMsg := ""
		status := "published"
		if r.Err != nil {
			errMsg = r.Err.Error()
			status = "failed"
		}
		log := db.PublicationLog{
			ContentPlanID:  plan.ID,
			Platform:       r.Platform,
			PlatformPostID: r.PostID,
			Status:         status,
			ErrorMessage:   errMsg,
		}
		if r.Err == nil {
			log.PublishedAt = &now
		}
		if err := p.database.InsertPublicationLog(log); err != nil {
			fmt.Printf("  ⚠️  Error guardando log de %s: %v\n", r.Platform, err)
		}
	}

	successCount := 0
	for _, r := range results {
		if r.Err == nil {
			successCount++
		}
	}

	if successCount == 0 && len(results) > 0 {
		return fmt.Errorf("falló la publicación en todas las plataformas")
	}

	return nil
}
