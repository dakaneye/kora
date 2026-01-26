-- Kora Memory Store Schema
-- SQLite 3.35.0+ (FTS5 support required)
-- Location: ~/.kora/data/kora.db

-- ============================================================================
-- Schema Versioning
-- ============================================================================

CREATE TABLE _meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO _meta (key, value) VALUES ('schema_version', '2');


-- ============================================================================
-- Core Tables
-- ============================================================================

-- Goals: User objectives and work priorities
CREATE TABLE goals (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT DEFAULT 'active', -- "active", "completed", "on_hold"
    priority INTEGER DEFAULT 3,   -- 1-5, lower is higher priority
    target_date TEXT,              -- RFC3339 when you want it done
    tags TEXT,                     -- JSON array: ["q4", "priority", "project:auth"]
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_goals_status ON goals(status) WHERE is_deleted = 0;
CREATE INDEX idx_goals_priority ON goals(priority) WHERE is_deleted = 0;


-- Commitments: What you promised to do and to whom
CREATE TABLE commitments (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    to_whom TEXT,                  -- "alice", "platform-team", or null for self
    status TEXT DEFAULT 'active',  -- "active", "in_progress", "completed"
    due_date TEXT NOT NULL,        -- RFC3339 deadline
    tags TEXT,                     -- JSON array
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_commitments_status ON commitments(status) WHERE is_deleted = 0;
CREATE INDEX idx_commitments_due_date ON commitments(due_date) WHERE is_deleted = 0;


-- Accomplishments: What you shipped, resolved, and achieved
CREATE TABLE accomplishments (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    impact TEXT,                   -- "Reduced build time from 15m to 8m", "Unblocked 3 teams"
    source_url TEXT,               -- GitHub PR, commit, issue link
    accomplished_at TEXT NOT NULL, -- RFC3339 when it happened
    tags TEXT,                     -- JSON array: ["shipped", "security", "product"]
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_accomplishments_date ON accomplishments(accomplished_at);


-- Context: Knowledge about people, projects, and areas
CREATE TABLE context (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,     -- "person", "project", "repo", "team"
    entity_id TEXT NOT NULL,       -- username, project name, repo name, etc
    title TEXT NOT NULL,
    body TEXT NOT NULL,            -- What Claude should know (markdown ok)
    urgency TEXT,                  -- "critical", "high", "normal"
    source_url TEXT,               -- Where this came from (GitHub, etc)
    tags TEXT,                     -- JSON array
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_context_entity ON context(entity_type, entity_id) WHERE is_deleted = 0;
CREATE INDEX idx_context_urgency ON context(urgency) WHERE is_deleted = 0;


-- Standups: Daily standup reports generated and saved
CREATE TABLE standups (
    id TEXT PRIMARY KEY,
    standup_text TEXT NOT NULL,        -- Full standup content
    date TEXT NOT NULL,                 -- Date standup covers (YYYY-MM-DD)
    format TEXT DEFAULT 'markdown',     -- "terminal", "markdown", "slack"
    status TEXT DEFAULT 'draft',        -- "draft", "sent"
    sources_used TEXT,                  -- JSON: {"kora_digest": true, "memory": true}
    sent_at TEXT,                       -- RFC3339 when marked as sent
    referenced_accomplishments TEXT,    -- JSON array of accomplishment IDs
    referenced_goals TEXT,              -- JSON array of goal IDs
    referenced_commitments TEXT,        -- JSON array of commitment IDs
    tags TEXT,                          -- JSON array
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_standups_date ON standups(date) WHERE is_deleted = 0;
CREATE INDEX idx_standups_status ON standups(status) WHERE is_deleted = 0;
CREATE INDEX idx_standups_created ON standups(created_at) WHERE is_deleted = 0;


-- ============================================================================
-- Full-Text Search (FTS5)
-- ============================================================================

-- Virtual table for full-text search across all memory
-- Note: We store content (not contentless) to enable trigger-based updates
CREATE VIRTUAL TABLE memory_search USING fts5(
    content,  -- "goal", "commitment", "accomplishment", "context"
    title,
    body,
    tags
);


-- ============================================================================
-- Triggers: Automatic updated_at on modification
-- ============================================================================

-- Update goals.updated_at when row is modified
CREATE TRIGGER goals_update_timestamp
AFTER UPDATE ON goals
BEGIN
    UPDATE goals SET updated_at = datetime('now')
    WHERE id = NEW.id;
END;

-- Update commitments.updated_at when row is modified
CREATE TRIGGER commitments_update_timestamp
AFTER UPDATE ON commitments
BEGIN
    UPDATE commitments SET updated_at = datetime('now')
    WHERE id = NEW.id;
END;

-- Update accomplishments.updated_at when row is modified
CREATE TRIGGER accomplishments_update_timestamp
AFTER UPDATE ON accomplishments
BEGIN
    UPDATE accomplishments SET updated_at = datetime('now')
    WHERE id = NEW.id;
END;

-- Update context.updated_at when row is modified
CREATE TRIGGER context_update_timestamp
AFTER UPDATE ON context
BEGIN
    UPDATE context SET updated_at = datetime('now')
    WHERE id = NEW.id;
END;


-- ============================================================================
-- Triggers: FTS5 Synchronization
-- ============================================================================

-- Sync goals to full-text search on INSERT
CREATE TRIGGER goals_fts_insert
AFTER INSERT ON goals
BEGIN
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('goal', NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.tags, ''));
END;

-- Sync goals to full-text search on UPDATE (only when not being soft-deleted)
CREATE TRIGGER goals_fts_update
AFTER UPDATE ON goals
WHEN NEW.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'goal' AND title = OLD.title AND body = COALESCE(OLD.description, '')
        LIMIT 1
    );
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('goal', NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.tags, ''));
END;

-- Sync goals to full-text search on DELETE (soft delete)
CREATE TRIGGER goals_fts_delete
AFTER UPDATE ON goals
WHEN NEW.is_deleted = 1 AND OLD.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'goal' AND title = OLD.title AND body = COALESCE(OLD.description, '')
        LIMIT 1
    );
END;


-- Sync commitments to full-text search on INSERT
CREATE TRIGGER commitments_fts_insert
AFTER INSERT ON commitments
BEGIN
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('commitment', NEW.title, COALESCE(NEW.to_whom, ''), COALESCE(NEW.tags, ''));
END;

-- Sync commitments to full-text search on UPDATE (only when not being soft-deleted)
CREATE TRIGGER commitments_fts_update
AFTER UPDATE ON commitments
WHEN NEW.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'commitment' AND title = OLD.title AND body = COALESCE(OLD.to_whom, '')
        LIMIT 1
    );
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('commitment', NEW.title, COALESCE(NEW.to_whom, ''), COALESCE(NEW.tags, ''));
END;

-- Sync commitments to full-text search on DELETE (soft delete)
CREATE TRIGGER commitments_fts_delete
AFTER UPDATE ON commitments
WHEN NEW.is_deleted = 1 AND OLD.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'commitment' AND title = OLD.title AND body = COALESCE(OLD.to_whom, '')
        LIMIT 1
    );
END;


-- Sync accomplishments to full-text search on INSERT
CREATE TRIGGER accomplishments_fts_insert
AFTER INSERT ON accomplishments
BEGIN
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('accomplishment', NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.tags, ''));
END;

-- Sync accomplishments to full-text search on UPDATE (only when not being soft-deleted)
CREATE TRIGGER accomplishments_fts_update
AFTER UPDATE ON accomplishments
WHEN NEW.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'accomplishment' AND title = OLD.title AND body = COALESCE(OLD.description, '')
        LIMIT 1
    );
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('accomplishment', NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.tags, ''));
END;

-- Sync accomplishments to full-text search on DELETE (soft delete)
CREATE TRIGGER accomplishments_fts_delete
AFTER UPDATE ON accomplishments
WHEN NEW.is_deleted = 1 AND OLD.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'accomplishment' AND title = OLD.title AND body = COALESCE(OLD.description, '')
        LIMIT 1
    );
END;


-- Sync context to full-text search on INSERT
CREATE TRIGGER context_fts_insert
AFTER INSERT ON context
BEGIN
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('context', NEW.title, NEW.body, COALESCE(NEW.tags, ''));
END;

-- Sync context to full-text search on UPDATE (only when not being soft-deleted)
CREATE TRIGGER context_fts_update
AFTER UPDATE ON context
WHEN NEW.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'context' AND title = OLD.title AND body = OLD.body
        LIMIT 1
    );
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('context', NEW.title, NEW.body, COALESCE(NEW.tags, ''));
END;

-- Sync context to full-text search on DELETE (soft delete)
CREATE TRIGGER context_fts_delete
AFTER UPDATE ON context
WHEN NEW.is_deleted = 1 AND OLD.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'context' AND title = OLD.title AND body = OLD.body
        LIMIT 1
    );
END;


-- Update standups.updated_at when row is modified
CREATE TRIGGER standups_update_timestamp
AFTER UPDATE ON standups
BEGIN
    UPDATE standups SET updated_at = datetime('now')
    WHERE id = NEW.id;
END;

-- Sync standups to full-text search on INSERT
CREATE TRIGGER standups_fts_insert
AFTER INSERT ON standups
BEGIN
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('standup', NEW.date, NEW.standup_text, COALESCE(NEW.tags, ''));
END;

-- Sync standups to full-text search on UPDATE (only when not being soft-deleted)
CREATE TRIGGER standups_fts_update
AFTER UPDATE ON standups
WHEN NEW.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'standup' AND title = OLD.date AND body = OLD.standup_text
        LIMIT 1
    );
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('standup', NEW.date, NEW.standup_text, COALESCE(NEW.tags, ''));
END;

-- Sync standups to full-text search on DELETE (soft delete)
CREATE TRIGGER standups_fts_delete
AFTER UPDATE ON standups
WHEN NEW.is_deleted = 1 AND OLD.is_deleted = 0
BEGIN
    DELETE FROM memory_search WHERE rowid = (
        SELECT rowid FROM memory_search
        WHERE content = 'standup' AND title = OLD.date AND body = OLD.standup_text
        LIMIT 1
    );
END;
