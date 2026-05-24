#!/usr/bin/env bash
# Survey all Claude Code session JSONL files to catalog data shapes.
# Outputs a structured summary of top-level record types, their key signatures,
# and nested content block shapes.
#
# Usage: ./scripts/survey-shapes.sh [directory]
# Default directory: ~/.claude

set -euo pipefail

DIR="${1:-$HOME/.claude}"

if ! command -v jq &>/dev/null; then
  echo "error: jq is required" >&2
  exit 1
fi

echo "Scanning JSONL files in: $DIR"
echo "============================================"

FILES=$(find "$DIR" -name "*.jsonl" -type f)
FILE_COUNT=$(echo "$FILES" | wc -l | tr -d ' ')
echo "Found $FILE_COUNT session files"
echo ""

# --- Section 1: Top-level record types and their key signatures ---
echo "=== TOP-LEVEL RECORD TYPES ==="
echo "Format: type (count) — [keys]"
echo ""

echo "$FILES" | xargs cat | jq -r '
  .type as $t |
  (keys | sort | join(",")) as $keys |
  "\($t)\t\($keys)"
' 2>/dev/null | sort | uniq -c | sort -rn | head -100

echo ""
echo ""

# --- Section 2: Content block types within user/assistant messages ---
echo "=== CONTENT BLOCK TYPES (within message.content arrays) ==="
echo "Format: block_type (count) — [keys]"
echo ""

echo "$FILES" | xargs cat | jq -r '
  select(.type == "user" or .type == "assistant") |
  .message.content |
  if type == "array" then .[]
  elif type == "string" then empty
  else empty
  end |
  .type as $t |
  (keys | sort | join(",")) as $keys |
  "\($t)\t\($keys)"
' 2>/dev/null | sort | uniq -c | sort -rn | head -100

echo ""
echo ""

# --- Section 3: tool_result content shapes (the nested content field) ---
echo "=== TOOL_RESULT CONTENT SHAPES ==="
echo "Format: content_type — [keys] (count)"
echo ""

echo "$FILES" | xargs cat | jq -r '
  select(.type == "user" or .type == "assistant") |
  .message.content |
  if type == "array" then .[]
  else empty
  end |
  select(.type == "tool_result") |
  .content |
  if type == "array" then
    .[] | "array_item:\(.type)\t" + (keys | sort | join(","))
  elif type == "string" then
    "plain_string\t-"
  elif type == "object" then
    "object\t" + (keys | sort | join(","))
  elif . == null then
    "null\t-"
  else
    "other:\(type)\t-"
  end
' 2>/dev/null | sort | uniq -c | sort -rn | head -50

echo ""
echo ""

# --- Section 4: tool_use input key patterns (per tool name) ---
echo "=== TOOL_USE INPUT SHAPES (top 50 tool/key combos) ==="
echo "Format: tool_name — [input_keys] (count)"
echo ""

echo "$FILES" | xargs cat | jq -r '
  select(.type == "user" or .type == "assistant") |
  .message.content |
  if type == "array" then .[]
  else empty
  end |
  select(.type == "tool_use") |
  .name as $name |
  (.input | keys | sort | join(",")) as $keys |
  "\($name)\t\($keys)"
' 2>/dev/null | sort | uniq -c | sort -rn | head -50

echo ""
echo ""

# --- Section 5: Image source shapes ---
echo "=== IMAGE SOURCE SHAPES ==="
echo ""

echo "$FILES" | xargs cat | jq -r '
  select(.type == "user" or .type == "assistant") |
  .message.content |
  if type == "array" then .[]
  else empty
  end |
  select(.type == "image") |
  .source |
  (keys | sort | join(",")) as $keys |
  "\(.type)\t\($keys)"
' 2>/dev/null | sort | uniq -c | sort -rn | head -20

echo ""
echo ""

# --- Section 6: Envelope-level field value summaries ---
echo "=== ENVELOPE FIELD VALUE DISTRIBUTIONS ==="
echo ""

echo "-- origin.kind values:"
echo "$FILES" | xargs cat | jq -r '
  select(.origin != null and .origin.kind != null) |
  .origin.kind
' 2>/dev/null | sort | uniq -c | sort -rn | head -20

echo ""
echo "-- userType values:"
echo "$FILES" | xargs cat | jq -r '
  select(.userType != null) | .userType
' 2>/dev/null | sort | uniq -c | sort -rn | head -20

echo ""
echo "-- message.role values:"
echo "$FILES" | xargs cat | jq -r '
  select(.message != null and .message.role != null) | .message.role
' 2>/dev/null | sort | uniq -c | sort -rn | head -20

echo ""
echo ""

# --- Section 7: Unknown/unexpected content block types ---
echo "=== CONTENT BLOCK TYPES NOT IN RELIC PARSER ==="
echo "(Excluding: text, tool_use, tool_result, thinking, redacted_thinking, image)"
echo ""

echo "$FILES" | xargs cat | jq -r '
  select(.type == "user" or .type == "assistant") |
  .message.content |
  if type == "array" then .[]
  else empty
  end |
  select(.type != "text" and .type != "tool_use" and .type != "tool_result"
    and .type != "thinking" and .type != "redacted_thinking" and .type != "image") |
  .type as $t |
  (keys | sort | join(",")) as $keys |
  "\($t)\t\($keys)"
' 2>/dev/null | sort | uniq -c | sort -rn | head -30

echo ""
echo ""

# --- Section 8: Top-level types NOT handled by relic ---
echo "=== TOP-LEVEL TYPES NOT HANDLED BY RELIC ==="
echo "(Excluding: user, assistant, custom-title, system, result, attachment,"
echo " permission-mode, last-prompt, file-history-snapshot, agent-name)"
echo ""

echo "$FILES" | xargs cat | jq -r '
  select(.type != "user" and .type != "assistant" and .type != "custom-title"
    and .type != "system" and .type != "result" and .type != "attachment"
    and .type != "permission-mode" and .type != "last-prompt"
    and .type != "file-history-snapshot" and .type != "agent-name") |
  .type as $t |
  (keys | sort | join(",")) as $keys |
  "\($t)\t\($keys)"
' 2>/dev/null | sort | uniq -c | sort -rn | head -30

echo ""
echo "Done."
