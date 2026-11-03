# valtag CLI

Secret vault CLI for the valtag platform.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/valtag-dev/valtag/main/scripts/install-valt-cli.sh | sh
```

## Quick start

```bash
valtag setup                      # configure + login + pick project
valtag mcp install --ide claude   # install MCP server for Claude Desktop
valtag list                       # list accessible secrets
valtag get DB_PASSWORD            # get a secret value
valtag run -- node server.js      # inject secrets + run command
valtag request DB_PASSWORD --reason "fixing prod" --duration 2h
valtag status <request-id>
```
