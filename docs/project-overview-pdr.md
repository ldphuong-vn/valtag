# valtag - Product Development Requirements

## Product
**valtag** - MCP-native secret vault with human-in-the-loop approval workflow for AI agents.

## Problem
AI agents need access to secrets (API keys, DB credentials) but developers either hardcode them or paste into chat. No approval workflow, no time-limited access, no audit trail.

## Solution
1. Encrypted secret storage (zero-knowledge, AES-256-GCM envelope encryption)
2. MCP Protocol integration for AI agent access requests
3. Human approval workflow with email notifications
4. Temporary, auto-expiring credentials
5. Complete audit trail with hash chain integrity

## Target Users
- Dev teams (5-50 people) using AI coding assistants
- Enterprise security teams controlling AI access

## MVP Scope
- Go backend (auth, vault, workflow, audit, email notify)
- Rust MCP server (5 tools, 3 resources)
- Next.js dashboard (secrets, approvals, audit viewer)
- Client-side encryption
- Single VPS deployment with Docker Compose
