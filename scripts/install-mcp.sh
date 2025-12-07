#!/usr/bin/env bash
# Install Kora as an MCP tool for Claude Code
#
# This script creates a project-scoped MCP configuration that allows Claude Code
# to invoke kora digest as a tool. The configuration uses the kora binary from
# ~/.local/bin and outputs JSON for structured consumption.
#
# Usage: ./scripts/install-mcp.sh

set -euo pipefail

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Configuration
readonly KORA_BINARY="$HOME/.local/bin/kora"
readonly PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
readonly MCP_CONFIG="$PROJECT_ROOT/.mcp.json"

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $*" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

# Check if kora binary exists
check_kora_binary() {
    if [[ ! -x "$KORA_BINARY" ]]; then
        log_error "Kora binary not found at $KORA_BINARY"
        log_info "Please run: make install"
        return 1
    fi
    log_info "Found kora binary at $KORA_BINARY"
}

# Create MCP configuration
create_mcp_config() {
    local config_exists=false

    if [[ -f "$MCP_CONFIG" ]]; then
        config_exists=true
        log_warn "MCP configuration already exists at $MCP_CONFIG"

        # Check if kora-digest is already configured
        if grep -q '"kora-digest"' "$MCP_CONFIG" 2>/dev/null; then
            log_info "kora-digest already configured, updating..."
        fi
    fi

    # Create the MCP configuration
    # If file exists, we'll merge; if not, create new
    if [[ "$config_exists" == "false" ]]; then
        cat > "$MCP_CONFIG" <<'EOF'
{
  "mcpServers": {
    "kora-digest": {
      "command": "/bin/sh",
      "args": [
        "-c",
        "~/.local/bin/kora digest --format json --since ${HOURS:-16}h"
      ],
      "description": "Get prioritized digest of GitHub PRs, issues, and Slack messages from the last N hours (default: 16). Set HOURS env var to customize time window.",
      "env": {
        "HOURS": "16"
      }
    }
  }
}
EOF
        log_info "Created new MCP configuration at $MCP_CONFIG"
    else
        # Merge with existing config using jq if available, otherwise warn
        if command -v jq &>/dev/null; then
            local temp_file
            temp_file=$(mktemp)

            # Create the kora-digest entry
            local kora_entry
            kora_entry=$(cat <<'EOF'
{
  "kora-digest": {
    "command": "/bin/sh",
    "args": [
      "-c",
      "~/.local/bin/kora digest --format json --since ${HOURS:-16}h"
    ],
    "description": "Get prioritized digest of GitHub PRs, issues, and Slack messages from the last N hours (default: 16). Set HOURS env var to customize time window.",
    "env": {
      "HOURS": "16"
    }
  }
}
EOF
)

            # Merge configurations
            jq --argjson kora "$kora_entry" \
               '.mcpServers += $kora' \
               "$MCP_CONFIG" > "$temp_file" && mv "$temp_file" "$MCP_CONFIG"

            log_info "Updated MCP configuration with kora-digest"
        else
            log_warn "jq not found. Please manually add kora-digest to $MCP_CONFIG"
            log_info "Configuration snippet:"
            cat <<'EOF'

  "kora-digest": {
    "command": "/bin/sh",
    "args": [
      "-c",
      "~/.local/bin/kora digest --format json --since ${HOURS:-16}h"
    ],
    "description": "Get prioritized digest of GitHub PRs, issues, and Slack messages from the last N hours (default: 16). Set HOURS env var to customize time window.",
    "env": {
      "HOURS": "16"
    }
  }
EOF
            return 1
        fi
    fi
}

# Print success message with usage examples
print_success() {
    log_info "Kora MCP tool installed successfully!"
    echo ""
    echo "Claude Code can now invoke kora as a tool. Try asking:"
    echo ""
    echo "  • \"What's in my digest?\""
    echo "  • \"Show me my GitHub PRs from the last 8 hours\""
    echo "  • \"What Slack messages do I need to respond to?\""
    echo ""
    echo "The tool will use kora digest with --format json output."
    echo ""
    echo "Configuration location: $MCP_CONFIG"
    echo "Binary location: $KORA_BINARY"
    echo ""
    echo "To customize the time window, edit the HOURS env var in $MCP_CONFIG"
    echo "or Claude Code will use the default (16 hours)."
}

# Main execution
main() {
    log_info "Installing Kora as MCP tool for Claude Code..."

    # Check prerequisites
    if ! check_kora_binary; then
        exit 1
    fi

    # Create MCP configuration
    if ! create_mcp_config; then
        exit 1
    fi

    # Print success message
    print_success
}

# Run main function
main
