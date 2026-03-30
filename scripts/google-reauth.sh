#!/usr/bin/env bash
#
# google-reauth.sh — Re-authenticate Google Calendar OAuth2 tokens.
#
# Automatically detects the config file from the systemd service unit,
# extracts credential/token paths, and walks you through the process.
#
# Usage:
#   google-reauth.sh              # auto-detect everything, get auth URL
#   google-reauth.sh <auth-code>  # complete the exchange with a code
#   google-reauth.sh -c <config>  # override config file path
#
set -euo pipefail

SERVICE_NAME="calendar-notifier"
CONFIG_FILE=""
AUTH_CODE=""
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

usage() {
    cat <<EOF
Usage: $(basename "$0") [-c config.yaml] [auth-code]

Re-authenticate Google Calendar OAuth2 tokens.

Options:
  -c <path>    Path to config.yaml (default: auto-detect from systemd)

Without an auth code, prints the authorization URL.
With an auth code, exchanges it for a new token.
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -c) CONFIG_FILE="$2"; shift 2 ;;
        -h|--help) usage ;;
        -*) echo "Unknown option: $1" >&2; usage ;;
        *) AUTH_CODE="$1"; shift ;;
    esac
done

# --- Auto-detect config file from systemd ---
if [[ -z "$CONFIG_FILE" ]]; then
    echo "Detecting config file from systemd service..."
    if ! command -v systemctl &>/dev/null; then
        echo "Error: systemctl not found. Specify the config file with -c." >&2
        exit 1
    fi

    UNIT_CONTENT=$(systemctl cat "$SERVICE_NAME" 2>/dev/null) || {
        echo "Error: Could not read systemd unit for '$SERVICE_NAME'." >&2
        echo "       Is the service installed? Specify config with -c." >&2
        exit 1
    }

    # Extract config path from ExecStart line: look for -config <path>
    CONFIG_FILE=$(echo "$UNIT_CONTENT" | grep -oP '(?<=-config\s)\S+' | head -1) || true

    if [[ -z "$CONFIG_FILE" ]]; then
        echo "Error: Could not find -config flag in ExecStart." >&2
        echo "" >&2
        echo "ExecStart line:" >&2
        echo "$UNIT_CONTENT" | grep 'ExecStart' >&2
        echo "" >&2
        echo "Specify the config file manually with -c." >&2
        exit 1
    fi

    echo "Found config: $CONFIG_FILE"
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Error: Config file not found: $CONFIG_FILE" >&2
    exit 1
fi

# --- Extract Google calendar credentials from config ---
if ! command -v yq &>/dev/null; then
    echo "Error: yq is required but not found. Install it: https://github.com/mikefarah/yq" >&2
    exit 1
fi

# Use yq to extract google calendar entries as tab-separated: name, credentials_file, token_file
GOOGLE_ENTRIES=$(yq -r '
    .calendars[] | select(.type == "google") |
    [.name, .credentials_file, (.token_file // (.credentials_file + ".token"))] |
    @tsv
' "$CONFIG_FILE")

if [[ -z "$GOOGLE_ENTRIES" ]]; then
    echo "Error: No Google calendar entries found in $CONFIG_FILE" >&2
    exit 1
fi

ENTRY_COUNT=$(echo "$GOOGLE_ENTRIES" | wc -l | tr -d ' ')

if [[ "$ENTRY_COUNT" -gt 1 ]]; then
    echo ""
    echo "Multiple Google calendars found:"
    i=1
    while IFS=$'\t' read -r name creds token; do
        echo "  $i) $name"
        i=$((i + 1))
    done <<< "$GOOGLE_ENTRIES"
    echo ""
    read -rp "Select calendar [1]: " choice
    choice=${choice:-1}
    ENTRY=$(echo "$GOOGLE_ENTRIES" | sed -n "${choice}p")
else
    ENTRY="$GOOGLE_ENTRIES"
fi

IFS=$'\t' read -r CAL_NAME CREDS_FILE TOKEN_FILE <<< "$ENTRY"

echo ""
echo "Calendar:     $CAL_NAME"
echo "Credentials:  $CREDS_FILE"
echo "Token file:   $TOKEN_FILE"

if [[ ! -f "$CREDS_FILE" ]]; then
    echo "Error: Credentials file not found: $CREDS_FILE" >&2
    exit 1
fi

# --- Run the Go auth helper ---
AUTH_HELPER="$SCRIPT_DIR/examples/google-auth-helper.go"

if [[ ! -f "$AUTH_HELPER" ]]; then
    echo "Error: Auth helper not found: $AUTH_HELPER" >&2
    exit 1
fi

if [[ -n "$AUTH_CODE" ]]; then
    echo ""
    echo "Exchanging authorization code..."
    (cd "$SCRIPT_DIR" && go run "$AUTH_HELPER" "$CREDS_FILE" "$TOKEN_FILE" "$AUTH_CODE")
    echo ""
    echo "Restart the service to pick up the new token:"
    echo "  sudo systemctl restart $SERVICE_NAME"
else
    echo ""
    (cd "$SCRIPT_DIR" && go run "$AUTH_HELPER" "$CREDS_FILE" "$TOKEN_FILE")
    echo ""
    read -rp "Paste the redirect URI from Google: " REDIRECT_URI

    # Extract the authorization code from the redirect URI query string
    AUTH_CODE=$(echo "$REDIRECT_URI" | grep -oE '([?&])code=([^&]+)' | sed 's/.*code=//' | head -1)

    if [[ -z "$AUTH_CODE" ]]; then
        echo "Error: Could not extract authorization code from URI." >&2
        echo "Expected a URL containing a 'code=' parameter." >&2
        exit 1
    fi

    echo ""
    echo "Exchanging authorization code..."
    (cd "$SCRIPT_DIR" && go run "$AUTH_HELPER" "$CREDS_FILE" "$TOKEN_FILE" "$AUTH_CODE")
    echo ""
    echo "Restart the service to pick up the new token:"
    echo "  sudo systemctl restart $SERVICE_NAME"
fi
