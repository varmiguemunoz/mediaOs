package db

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	conn.SetMaxOpenConns(1)

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", e.Name(), err)
		}
		if _, err := d.conn.Exec(string(data)); err != nil {
			return fmt.Errorf("executing migration %s: %w", e.Name(), err)
		}
	}
	return nil
}

type Avatar struct {
	ID             int64
	Name           string
	HeyGenAvatarID string
	HeyGenVoiceID  string
	Style          string
	BestFor        string
	Active         bool
	LastUsedAt     *time.Time
}

type ContentPlan struct {
	ID         int64
	Date       string
	Pillar     string
	AvatarID   *int64
	Topic      string
	Angle      string
	Format     string
	Difficulty string
	CTA        string
	Status     string
	CreatedAt  time.Time
}

type GeneratedScript struct {
	ID                       int64
	ContentPlanID            int64
	Title                    string
	Script                   string
	CaptionInstagram         string
	CaptionFacebook          string
	CaptionLinkedIn          string
	CaptionTikTok            string
	Hashtags                 string
	CTA                      string
	EstimatedDurationSeconds int
	WordCount                int
	CreatedAt                time.Time
}

type VideoRenderJob struct {
	ID            int64
	ContentPlanID int64
	HeyGenVideoID string
	Status        string
	VideoURL      string
	ErrorMessage  string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

type PublicationLog struct {
	ID             int64
	ContentPlanID  int64
	Platform       string
	PlatformPostID string
	Status         string
	ErrorMessage   string
	PublishedAt    *time.Time
	CreatedAt      time.Time
}

func (d *DB) UpsertAvatar(a Avatar) error {
	_, err := d.conn.Exec(`
		INSERT INTO avatars (name, heygen_avatar_id, heygen_voice_id, style, best_for, active)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(heygen_avatar_id) DO UPDATE SET
			name=excluded.name,
			heygen_voice_id=excluded.heygen_voice_id,
			style=excluded.style,
			best_for=excluded.best_for,
			active=excluded.active
	`, a.Name, a.HeyGenAvatarID, a.HeyGenVoiceID, a.Style, a.BestFor, a.Active)
	return err
}

func (d *DB) ListActiveAvatars() ([]Avatar, error) {
	rows, err := d.conn.Query(`
		SELECT id, name, heygen_avatar_id, heygen_voice_id, style, best_for, active, last_used_at
		FROM avatars WHERE active = 1
		ORDER BY last_used_at ASC NULLS FIRST
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAvatars(rows)
}

func (d *DB) GetLeastUsedAvatar() (*Avatar, error) {
	row := d.conn.QueryRow(`
		SELECT id, name, heygen_avatar_id, heygen_voice_id, style, best_for, active, last_used_at
		FROM avatars WHERE active = 1
		ORDER BY last_used_at ASC NULLS FIRST
		LIMIT 1
	`)
	return scanAvatar(row)
}

func (d *DB) UpdateAvatarLastUsed(id int64) error {
	_, err := d.conn.Exec(`UPDATE avatars SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (d *DB) InsertContentPlan(p ContentPlan) (int64, error) {
	res, err := d.conn.Exec(`
		INSERT INTO content_plans (date, pillar, topic, angle, format, difficulty, cta, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'planned')
	`, p.Date, p.Pillar, p.Topic, p.Angle, p.Format, p.Difficulty, p.CTA)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) SetContentPlanAvatar(planID, avatarID int64) error {
	_, err := d.conn.Exec(`UPDATE content_plans SET avatar_id = ? WHERE id = ?`, avatarID, planID)
	return err
}

func (d *DB) UpdateContentPlanStatus(id int64, status string) error {
	_, err := d.conn.Exec(`UPDATE content_plans SET status = ? WHERE id = ?`, status, id)
	return err
}

func (d *DB) GetTodayPlan(date string) (*ContentPlan, error) {
	row := d.conn.QueryRow(`
		SELECT id, date, pillar, avatar_id, topic, angle, format, difficulty, cta, status, created_at
		FROM content_plans
		WHERE date = ? AND status = 'planned' AND avatar_id IS NOT NULL
		LIMIT 1
	`, date)
	return scanContentPlan(row)
}

func (d *DB) ListPendingPlans() ([]ContentPlan, error) {
	rows, err := d.conn.Query(`
		SELECT id, date, pillar, avatar_id, topic, angle, format, difficulty, cta, status, created_at
		FROM content_plans WHERE status = 'planned'
		ORDER BY date ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContentPlans(rows)
}

func (d *DB) InsertGeneratedScript(s GeneratedScript) (int64, error) {
	res, err := d.conn.Exec(`
		INSERT INTO generated_scripts
			(content_plan_id, title, script, caption_instagram, caption_facebook, caption_linkedin, caption_tiktok, hashtags, cta, estimated_duration_seconds, word_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ContentPlanID, s.Title, s.Script, s.CaptionInstagram, s.CaptionFacebook, s.CaptionLinkedIn, s.CaptionTikTok, s.Hashtags, s.CTA, s.EstimatedDurationSeconds, s.WordCount)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetScriptByPlanID(planID int64) (*GeneratedScript, error) {
	row := d.conn.QueryRow(`
		SELECT id, content_plan_id, title, script, caption_instagram, caption_facebook, caption_linkedin, caption_tiktok, hashtags, cta, estimated_duration_seconds, word_count, created_at
		FROM generated_scripts WHERE content_plan_id = ? ORDER BY created_at DESC LIMIT 1
	`, planID)
	return scanGeneratedScript(row)
}

func (d *DB) InsertVideoRenderJob(j VideoRenderJob) (int64, error) {
	res, err := d.conn.Exec(`
		INSERT INTO video_render_jobs (content_plan_id, heygen_video_id, status)
		VALUES (?, ?, 'video_requested')
	`, j.ContentPlanID, j.HeyGenVideoID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) ListPendingVideoJobs() ([]VideoRenderJob, error) {
	rows, err := d.conn.Query(`
		SELECT id, content_plan_id, heygen_video_id, status, video_url, error_message, created_at, completed_at
		FROM video_render_jobs WHERE status = 'video_requested'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVideoJobs(rows)
}

func (d *DB) UpdateVideoRenderJob(id int64, status, videoURL, errMsg string) error {
	var completedAt interface{}
	if status == "rendered" || status == "failed" {
		completedAt = "CURRENT_TIMESTAMP"
	}
	if completedAt != nil {
		_, err := d.conn.Exec(`
			UPDATE video_render_jobs SET status=?, video_url=?, error_message=?, completed_at=CURRENT_TIMESTAMP WHERE id=?
		`, status, videoURL, errMsg, id)
		return err
	}
	_, err := d.conn.Exec(`
		UPDATE video_render_jobs SET status=?, video_url=?, error_message=? WHERE id=?
	`, status, videoURL, errMsg, id)
	return err
}

func (d *DB) GetAvatarByID(id int64) (*Avatar, error) {
	row := d.conn.QueryRow(`
		SELECT id, name, heygen_avatar_id, heygen_voice_id, style, best_for, active, last_used_at
		FROM avatars WHERE id = ?
	`, id)
	return scanAvatar(row)
}

func (d *DB) InsertPublicationLog(l PublicationLog) error {
	_, err := d.conn.Exec(`
		INSERT INTO publication_logs (content_plan_id, platform, platform_post_id, status, error_message, published_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, l.ContentPlanID, l.Platform, l.PlatformPostID, l.Status, l.ErrorMessage, l.PublishedAt)
	return err
}

func (d *DB) GetPlanByID(id int64) (*ContentPlan, error) {
	row := d.conn.QueryRow(`
		SELECT id, date, pillar, avatar_id, topic, angle, format, difficulty, cta, status, created_at
		FROM content_plans WHERE id = ?
	`, id)
	return scanContentPlan(row)
}

func (d *DB) GetVideoJobByPlanID(planID int64) (*VideoRenderJob, error) {
	row := d.conn.QueryRow(`
		SELECT id, content_plan_id, heygen_video_id, status, video_url, error_message, created_at, completed_at
		FROM video_render_jobs WHERE content_plan_id = ? ORDER BY created_at DESC LIMIT 1
	`, planID)
	return scanVideoJob(row)
}

func scanAvatars(rows *sql.Rows) ([]Avatar, error) {
	var result []Avatar
	for rows.Next() {
		var a Avatar
		var lastUsed sql.NullTime
		err := rows.Scan(&a.ID, &a.Name, &a.HeyGenAvatarID, &a.HeyGenVoiceID, &a.Style, &a.BestFor, &a.Active, &lastUsed)
		if err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			a.LastUsedAt = &lastUsed.Time
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func scanAvatar(row *sql.Row) (*Avatar, error) {
	var a Avatar
	var lastUsed sql.NullTime
	err := row.Scan(&a.ID, &a.Name, &a.HeyGenAvatarID, &a.HeyGenVoiceID, &a.Style, &a.BestFor, &a.Active, &lastUsed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		a.LastUsedAt = &lastUsed.Time
	}
	return &a, nil
}

func scanContentPlan(row *sql.Row) (*ContentPlan, error) {
	var p ContentPlan
	var avatarID sql.NullInt64
	err := row.Scan(&p.ID, &p.Date, &p.Pillar, &avatarID, &p.Topic, &p.Angle, &p.Format, &p.Difficulty, &p.CTA, &p.Status, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if avatarID.Valid {
		p.AvatarID = &avatarID.Int64
	}
	return &p, nil
}

func scanContentPlans(rows *sql.Rows) ([]ContentPlan, error) {
	var result []ContentPlan
	for rows.Next() {
		var p ContentPlan
		var avatarID sql.NullInt64
		err := rows.Scan(&p.ID, &p.Date, &p.Pillar, &avatarID, &p.Topic, &p.Angle, &p.Format, &p.Difficulty, &p.CTA, &p.Status, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		if avatarID.Valid {
			p.AvatarID = &avatarID.Int64
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func scanGeneratedScript(row *sql.Row) (*GeneratedScript, error) {
	var s GeneratedScript
	err := row.Scan(&s.ID, &s.ContentPlanID, &s.Title, &s.Script, &s.CaptionInstagram, &s.CaptionFacebook, &s.CaptionLinkedIn, &s.CaptionTikTok, &s.Hashtags, &s.CTA, &s.EstimatedDurationSeconds, &s.WordCount, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanVideoJobs(rows *sql.Rows) ([]VideoRenderJob, error) {
	var result []VideoRenderJob
	for rows.Next() {
		j, err := scanVideoJobFromRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *j)
	}
	return result, rows.Err()
}

func scanVideoJobFromRows(rows *sql.Rows) (*VideoRenderJob, error) {
	var j VideoRenderJob
	var completedAt sql.NullTime
	err := rows.Scan(&j.ID, &j.ContentPlanID, &j.HeyGenVideoID, &j.Status, &j.VideoURL, &j.ErrorMessage, &j.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}

func scanVideoJob(row *sql.Row) (*VideoRenderJob, error) {
	var j VideoRenderJob
	var completedAt sql.NullTime
	err := row.Scan(&j.ID, &j.ContentPlanID, &j.HeyGenVideoID, &j.Status, &j.VideoURL, &j.ErrorMessage, &j.CreatedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}
