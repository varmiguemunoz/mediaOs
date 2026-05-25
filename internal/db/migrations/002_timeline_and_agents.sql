ALTER TABLE generated_scripts ADD COLUMN timeline_json TEXT DEFAULT '';

CREATE TABLE IF NOT EXISTS composition_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    html_path TEXT DEFAULT '',
    video_path TEXT DEFAULT '',
    status TEXT DEFAULT 'composition_pending',
    error_message TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS edit_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    heygen_video_url TEXT DEFAULT '',
    composition_video_path TEXT DEFAULT '',
    final_video_path TEXT DEFAULT '',
    status TEXT DEFAULT 'edit_pending',
    error_message TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
