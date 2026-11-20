-- Users get a friendly display name (shown on the dashboard profile and in
-- approval context). Null = "no display name yet" — UI falls back to email.
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(100);
