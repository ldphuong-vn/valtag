# valt CLI

Secret vault CLI for the valtag platform.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/valt-dev/valt/main/scripts/install-valt-cli.sh | sh
```

## Quick start

```bash
valt setup                      # configure + login + pick project
valt mcp install --ide claude   # install MCP server for Claude Desktop
valt list                       # list accessible secrets
valt get DB_PASSWORD            # get a secret value
valt run -- node server.js      # inject secrets + run command
valt request DB_PASSWORD --reason "fixing prod" --duration 2h
valt status <request-id>
```
