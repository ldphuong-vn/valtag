-- token_version invalidates outstanding CLI/MCP JWTs (typ=cli, 30-day) when
-- bumped: revoke-all raises it, so the tokens' ver claim stops matching.
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1;
