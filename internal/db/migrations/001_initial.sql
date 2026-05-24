CREATE TABLE IF NOT EXISTS avatars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    heygen_avatar_id TEXT NOT NULL UNIQUE,
    heygen_voice_id TEXT NOT NULL,
    style TEXT DEFAULT '',
    best_for TEXT DEFAULT '',
    active INTEGER DEFAULT 1,
    last_used_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS content_pillars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    target_audience TEXT DEFAULT '',
    difficulty TEXT DEFAULT 'intermediate'
);

CREATE TABLE IF NOT EXISTS content_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    pillar TEXT NOT NULL,
    avatar_id INTEGER REFERENCES avatars(id),
    topic TEXT NOT NULL,
    angle TEXT DEFAULT '',
    format TEXT DEFAULT '',
    difficulty TEXT DEFAULT 'beginner',
    cta TEXT DEFAULT '',
    status TEXT DEFAULT 'planned',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS generated_scripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    title TEXT NOT NULL,
    script TEXT NOT NULL,
    caption_instagram TEXT DEFAULT '',
    caption_facebook TEXT DEFAULT '',
    caption_linkedin TEXT DEFAULT '',
    caption_tiktok TEXT DEFAULT '',
    hashtags TEXT DEFAULT '',
    cta TEXT DEFAULT '',
    estimated_duration_seconds INTEGER DEFAULT 40,
    word_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS video_render_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    heygen_video_id TEXT NOT NULL,
    status TEXT DEFAULT 'video_requested',
    video_url TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS publication_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    platform TEXT NOT NULL,
    platform_post_id TEXT DEFAULT '',
    status TEXT DEFAULT 'pending',
    error_message TEXT DEFAULT '',
    published_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO content_pillars (name, description, target_audience, difficulty) VALUES
    ('AI para negocios', 'Cómo aplicar inteligencia artificial en negocios reales para automatizar, vender y escalar', 'Founders, dueños de negocio, agencias', 'beginner'),
    ('Automatización WhatsApp', 'Construir agentes y flujos automáticos en WhatsApp para ventas y atención al cliente', 'Agencias, negocios medianos, founders técnicos', 'intermediate'),
    ('Backend real', 'APIs, bases de datos y arquitecturas para productos reales en producción', 'Programadores, tech leads', 'advanced'),
    ('TypeScript avanzado', 'Patrones, tipos avanzados y tooling moderno para desarrollo profesional', 'Desarrolladores JavaScript', 'advanced'),
    ('Arquitectura de software', 'Diseño de sistemas escalables, decisiones técnicas y patrones de arquitectura', 'Ingenieros senior, CTOs', 'advanced'),
    ('Herramientas para founders', 'Software, automatizaciones y workflows para founders técnicos que quieren moverse rápido', 'Founders técnicos, indie hackers', 'beginner');
