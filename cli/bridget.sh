#!/usr/bin/env bash
# Bridget CLI
# Works with any terminal emulator — Ghostty, iTerm2, Alacritty, Kitty, Terminal.app, etc.
# Supports multiple concurrent editor sessions with CWD-based bindings.
#
# Usage:
#   bridget sessions              List active editor sessions
#   bridget bind [editor-session] Bind CWD to an editor session (interactive if omitted)
#   bridget unbind                Remove binding for CWD
#   bridget bindings              List all bindings
#   bridget status  [-s ID]       Show full editor state
#   bridget file    [-s ID]       Print active file path
#   bridget selection [-s ID]     Print current text selection
#   bridget open <path> [line]    Tell the editor to open a file [-s ID]
#   bridget reveal <path> <line>  Scroll to a line without changing focus [-s ID]
#   bridget diff <path>           Show git diff for a file in the editor
#   bridget diagnostics [path]    Get LSP diagnostics [-s ID]
#   bridget watch   [-s ID]       Stream editor state changes
#
# Hook commands (for ~/.claude/settings.json):
#   bridget hook:context          UserPromptSubmit — inject editor context into prompt
#   bridget hook:preview          PreToolUse (Edit|Write) — show preview diff in editor
#   bridget hook:edit             PostToolUse (Edit|Write) — close preview diff tab

set -euo pipefail

BRIDGE_DIR="${HOME}/.claude/bridge"
SESSIONS_DIR="${BRIDGE_DIR}/sessions"
BINDINGS_FILE="${BRIDGE_DIR}/bindings.json"
LEGACY_STATE_FILE="${BRIDGE_DIR}/editor-state.json"

STALENESS_MS=300000

ensure_bridge_dir() {
  mkdir -p "$SESSIONS_DIR"
}

check_jq() {
  if ! command -v jq &>/dev/null; then
    echo "Error: jq is required. Install with: brew install jq" >&2
    exit 1
  fi
}

# ── Bindings ──────────────────────────────────────────────────────────────────

read_bindings() {
  if [ -f "$BINDINGS_FILE" ]; then
    cat "$BINDINGS_FILE"
  else
    echo '{}'
  fi
}

write_bindings() {
  local json="$1"
  ensure_bridge_dir
  local tmp="${BINDINGS_FILE}.$$"
  echo "$json" > "$tmp"
  mv "$tmp" "$BINDINGS_FILE"
}

# Look up the editor session bound to a directory (longest prefix match).
# Args: $1 = absolute file path
# Prints the editor session ID if found, empty otherwise.
lookup_binding_for_file() {
  local file_path="$1"
  local bindings
  bindings=$(read_bindings)

  # Find the longest CWD key that is a prefix of file_path
  echo "$bindings" | jq -r --arg fp "$file_path" '
    to_entries
    | map(select($fp | startswith(.key)))
    | sort_by(.key | length)
    | last
    | .value // empty
  ' 2>/dev/null
}

# ── Session listing ───────────────────────────────────────────────────────────

# List all non-stale session files, newest first
list_session_files() {
  local now_ms=$(($(date +%s) * 1000))
  local files=()

  if [ -d "$SESSIONS_DIR" ]; then
    for f in "$SESSIONS_DIR"/*.json; do
      [ -f "$f" ] || continue
      [[ "$f" == *.commands.json ]] && continue
      [[ "$f" == *.commands-result.json ]] && continue

      local ts
      ts=$(jq -r '.timestamp // 0' "$f" 2>/dev/null) || continue
      local age_ms=$((now_ms - ts))
      if [ "$age_ms" -le "$STALENESS_MS" ]; then
        files+=("$f")
      fi
    done
  fi

  if [ ${#files[@]} -gt 0 ]; then
    for f in "${files[@]}"; do
      local ts
      ts=$(jq -r '.timestamp // 0' "$f" 2>/dev/null) || continue
      echo "$ts $f"
    done | sort -rn | awk '{print $2}'
  fi
}

# ── Session resolution ────────────────────────────────────────────────────────

# Resolve by explicit session ID (exact or partial match).
resolve_state_file() {
  local target_session="${1:-}"

  if [ -n "$target_session" ]; then
    local f="$SESSIONS_DIR/${target_session}.json"
    if [ -f "$f" ]; then
      echo "$f"
      return 0
    fi
    local matches=()
    for candidate in "$SESSIONS_DIR"/*.json; do
      [ -f "$candidate" ] || continue
      [[ "$candidate" == *.commands.json ]] && continue
      [[ "$candidate" == *.commands-result.json ]] && continue
      local bn
      bn=$(basename "$candidate" .json)
      if [[ "$bn" == *"$target_session"* ]]; then
        matches+=("$candidate")
      fi
    done
    if [ ${#matches[@]} -eq 1 ]; then
      echo "${matches[0]}"
      return 0
    elif [ ${#matches[@]} -gt 1 ]; then
      echo "Error: Ambiguous session '$target_session'. Matches:" >&2
      for m in "${matches[@]}"; do
        echo "  $(basename "$m" .json)" >&2
      done
      exit 1
    fi
    echo "Error: No session found matching '$target_session'" >&2
    exit 1
  fi

  # Default: most recent session
  local newest
  newest=$(list_session_files | head -1)
  if [ -n "$newest" ]; then
    echo "$newest"
    return 0
  fi

  # Fallback to legacy file
  if [ -f "$LEGACY_STATE_FILE" ]; then
    local ts
    ts=$(jq -r '.timestamp // 0' "$LEGACY_STATE_FILE" 2>/dev/null) || true
    local now_ms=$(($(date +%s) * 1000))
    local age_ms=$((now_ms - ts))
    if [ "$age_ms" -le "$STALENESS_MS" ]; then
      echo "$LEGACY_STATE_FILE"
      return 0
    fi
  fi

  echo "Error: No active editor sessions. Is the VS Code/Cursor extension running?" >&2
  exit 1
}

# Resolve the best editor session for a given file path.
# Priority: 1) binding  2) editor workspace match  3) most recent session
# Args: $1 = absolute file path
# Prints the state file path.
resolve_for_file() {
  local file_path="$1"

  # 1) Check bindings
  local bound_session
  bound_session=$(lookup_binding_for_file "$file_path")
  if [ -n "$bound_session" ]; then
    # Verify session is still alive
    local f="$SESSIONS_DIR/${bound_session}.json"
    if [ -f "$f" ]; then
      local ts
      ts=$(jq -r '.timestamp // 0' "$f" 2>/dev/null) || true
      local now_ms=$(($(date +%s) * 1000))
      local age_ms=$((now_ms - ts))
      if [ "$age_ms" -le "$STALENESS_MS" ]; then
        echo "$f"
        return 0
      fi
    fi
    # Partial match fallback (PID may have changed but IDE name is the same)
    local partial
    partial=$(resolve_state_file "$bound_session" 2>/dev/null) || true
    if [ -n "$partial" ]; then
      echo "$partial"
      return 0
    fi
  fi

  # 2) Auto-match by editor workspace folders
  local now_ms=$(($(date +%s) * 1000))
  local best_file=""
  local best_len=0
  if [ -d "$SESSIONS_DIR" ]; then
    for f in "$SESSIONS_DIR"/*.json; do
      [ -f "$f" ] || continue
      [[ "$f" == *.commands.json ]] && continue
      [[ "$f" == *.commands-result.json ]] && continue

      local ts
      ts=$(jq -r '.timestamp // 0' "$f" 2>/dev/null) || continue
      local age_ms=$((now_ms - ts))
      [ "$age_ms" -gt "$STALENESS_MS" ] && continue

      # Check each workspace folder
      local folders
      folders=$(jq -r '.workspace.folders[]? // empty' "$f" 2>/dev/null)
      while IFS= read -r folder; do
        [ -z "$folder" ] && continue
        # Ensure folder ends with / for prefix matching
        local folder_slash="${folder%/}/"
        if [[ "$file_path" == "$folder_slash"* ]] || [[ "$file_path" == "$folder" ]]; then
          local flen=${#folder}
          if [ "$flen" -gt "$best_len" ]; then
            best_len=$flen
            best_file="$f"
          fi
        fi
      done <<< "$folders"
    done
  fi

  if [ -n "$best_file" ]; then
    echo "$best_file"
    return 0
  fi

  # 3) Fall back to most recent session
  resolve_state_file ""
}

# ── Command sending ──────────────────────────────────────────────────────────

get_session_id() {
  local f="$1"
  jq -r '.sessionId // empty' "$f" 2>/dev/null || basename "$f" .json
}

send_command() {
  local state_file="$1"
  local cmd_type="$2"
  local args="$3"
  local cmd_id="cmd-$$-$(date +%s)"

  ensure_bridge_dir

  local session_id
  session_id=$(get_session_id "$state_file")

  local cmd_file="$SESSIONS_DIR/${session_id}.commands.json"
  local result_file="$SESSIONS_DIR/${session_id}.commands-result.json"

  if [ "$state_file" = "$LEGACY_STATE_FILE" ]; then
    cmd_file="${BRIDGE_DIR}/commands.json"
    result_file="${BRIDGE_DIR}/command-results.json"
  fi

  # Write directly (not atomic mv) so the file inode stays the same.
  # On macOS, fs.watch uses kqueue which tracks by inode — atomic mv
  # replaces the inode and kills the watcher. Direct writes trigger
  # reliable "change" events. safeReadJson handles any partial reads.
  cat > "$cmd_file" <<EOF
{
  "version": 1,
  "id": "${cmd_id}",
  "timestamp": $(date +%s)000,
  "command": "${cmd_type}",
  "args": ${args}
}
EOF

  local i=0
  while [ $i -lt 30 ]; do
    if [ -f "$result_file" ]; then
      local result_id
      result_id=$(jq -r '.id // ""' "$result_file" 2>/dev/null)
      if [ "$result_id" = "$cmd_id" ]; then
        local status
        status=$(jq -r '.status' "$result_file" 2>/dev/null)
        if [ "$status" = "ok" ]; then
          jq -r '.result.message' "$result_file"
          local has_data
          has_data=$(jq 'has("result") and (.result | has("data"))' "$result_file" 2>/dev/null)
          if [ "$has_data" = "true" ]; then
            jq '.result.data' "$result_file"
          fi
        else
          jq -r '.result.message' "$result_file" >&2
          exit 1
        fi
        return 0
      fi
    fi
    sleep 0.1
    i=$((i + 1))
  done
  echo "Timeout waiting for editor response" >&2
  exit 1
}

# ── Argument parsing ─────────────────────────────────────────────────────────

SESSION_FILTER=""
ARGS=()

while [ $# -gt 0 ]; do
  case "$1" in
    -s|--session)
      SESSION_FILTER="$2"
      shift 2
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

set -- "${ARGS[@]+"${ARGS[@]}"}"

# ── Commands ─────────────────────────────────────────────────────────────────

case "${1:-help}" in

  # ── sessions ──
  sessions)
    check_jq
    files=$(list_session_files)
    if [ -z "$files" ]; then
      if [ -f "$LEGACY_STATE_FILE" ]; then
        local_ts=$(jq -r '.timestamp // 0' "$LEGACY_STATE_FILE" 2>/dev/null)
        now_ms=$(($(date +%s) * 1000))
        age_ms=$((now_ms - local_ts))
        if [ "$age_ms" -le "$STALENESS_MS" ]; then
          ide=$(jq -r '.ideName // "Editor"' "$LEGACY_STATE_FILE" 2>/dev/null)
          ws=$(jq -r '.workspace.name // ""' "$LEGACY_STATE_FILE" 2>/dev/null)
          file=$(jq -r '.activeFile.relativePath // "(no file)"' "$LEGACY_STATE_FILE" 2>/dev/null)
          echo "(legacy)  ${ide} — ${ws} — ${file}"
          exit 0
        fi
      fi
      echo "No active editor sessions."
      exit 0
    fi
    echo "$files" | while read -r f; do
      sid=$(jq -r '.sessionId // empty' "$f" 2>/dev/null)
      [ -z "$sid" ] && sid=$(basename "$f" .json)
      ide=$(jq -r '.ideName // "Editor"' "$f" 2>/dev/null)
      ws=$(jq -r '.workspace.name // ""' "$f" 2>/dev/null)
      file=$(jq -r '.activeFile.relativePath // "(no file)"' "$f" 2>/dev/null)
      printf "%-20s  %s — %s — %s\n" "$sid" "$ide" "$ws" "$file"
    done
    ;;

  # ── bind ──
  bind)
    check_jq
    shift
    target_session="${1:-}"
    bind_cwd="$(pwd)"

    if [ -z "$target_session" ]; then
      # Interactive: list sessions and prompt
      files=$(list_session_files)
      if [ -z "$files" ]; then
        echo "No active editor sessions to bind to." >&2
        exit 1
      fi

      echo "Active editor sessions:"
      echo ""
      idx=0
      declare -a session_ids=()
      while IFS= read -r f; do
        idx=$((idx + 1))
        sid=$(jq -r '.sessionId // empty' "$f" 2>/dev/null)
        [ -z "$sid" ] && sid=$(basename "$f" .json)
        session_ids+=("$sid")
        ide=$(jq -r '.ideName // "Editor"' "$f" 2>/dev/null)
        ws=$(jq -r '.workspace.name // ""' "$f" 2>/dev/null)
        file=$(jq -r '.activeFile.relativePath // "(no file)"' "$f" 2>/dev/null)
        printf "  %d) %-20s  %s — %s — %s\n" "$idx" "$sid" "$ide" "$ws" "$file"
      done <<< "$files"
      echo ""

      read -rp "Select session number (1-${idx}): " choice
      if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "$idx" ]; then
        echo "Invalid selection." >&2
        exit 1
      fi
      target_session="${session_ids[$((choice - 1))]}"
    fi

    # Verify target session exists
    resolve_state_file "$target_session" >/dev/null

    # Update bindings
    bindings=$(read_bindings)
    bindings=$(echo "$bindings" | jq --arg cwd "$bind_cwd" --arg sid "$target_session" '. + {($cwd): $sid}')
    write_bindings "$bindings"

    echo "Bound ${bind_cwd} → ${target_session}"
    ;;

  # ── unbind ──
  unbind)
    check_jq
    bind_cwd="$(pwd)"
    bindings=$(read_bindings)

    existing=$(echo "$bindings" | jq -r --arg cwd "$bind_cwd" '.[$cwd] // empty')
    if [ -z "$existing" ]; then
      echo "No binding for ${bind_cwd}"
      exit 0
    fi

    bindings=$(echo "$bindings" | jq --arg cwd "$bind_cwd" 'del(.[$cwd])')
    write_bindings "$bindings"
    echo "Removed binding ${bind_cwd} → ${existing}"
    ;;

  # ── bindings ──
  bindings)
    check_jq
    bindings=$(read_bindings)
    count=$(echo "$bindings" | jq 'length')

    if [ "$count" -eq 0 ]; then
      echo "No bindings configured. Use 'bridget bind' to set one up."
      exit 0
    fi

    echo "$bindings" | jq -r 'to_entries[] | "  \(.key) → \(.value)"'
    ;;

  # ── status ──
  status)
    check_jq
    state_file=$(resolve_state_file "$SESSION_FILTER")
    jq '.' "$state_file"
    ;;

  # ── file ──
  file)
    check_jq
    state_file=$(resolve_state_file "$SESSION_FILTER")
    jq -r '.activeFile.path // "No active file"' "$state_file"
    ;;

  # ── selection ──
  selection)
    check_jq
    state_file=$(resolve_state_file "$SESSION_FILTER")
    sel_text=$(jq -r '.selection.text // ""' "$state_file")
    if [ -z "$sel_text" ]; then
      echo "(no selection)"
    else
      echo "$sel_text"
    fi
    ;;

  # ── open ──
  open)
    check_jq
    shift
    if [ $# -lt 1 ]; then
      echo "Usage: bridget open <path> [line] [column] [-s session]" >&2
      exit 1
    fi
    state_file=$(resolve_state_file "$SESSION_FILTER")
    file_path="$1"
    line="${2:-}"
    column="${3:-}"

    args="{\"path\": \"${file_path}\""
    [ -n "$line" ] && args="${args}, \"line\": ${line}"
    [ -n "$column" ] && args="${args}, \"column\": ${column}"
    args="${args}, \"preview\": false}"

    send_command "$state_file" "openFile" "$args"
    ;;

  # ── reveal ──
  reveal)
    check_jq
    shift
    if [ $# -lt 2 ]; then
      echo "Usage: bridget reveal <path> <line> [-s session]" >&2
      exit 1
    fi
    state_file=$(resolve_state_file "$SESSION_FILTER")
    send_command "$state_file" "revealLine" "{\"path\": \"$1\", \"line\": $2}"
    ;;

  # ── diff ──
  diff)
    check_jq
    shift
    if [ $# -lt 1 ]; then
      echo "Usage: bridget diff <path> [-s session]" >&2
      exit 1
    fi
    file_path="$1"
    if [ -n "$SESSION_FILTER" ]; then
      state_file=$(resolve_state_file "$SESSION_FILTER")
    else
      state_file=$(resolve_for_file "$file_path")
    fi
    send_command "$state_file" "showDiff" "{\"path\": \"$1\"}"
    ;;

  # ── preview ──
  preview)
    check_jq
    # Reads JSON from stdin: {file_path, tool_name, old_string?, new_string?, content?}
    # Sends a previewEdit command. Fire-and-forget (no response polling).
    preview_input=$(cat)
    file_path=$(echo "$preview_input" | jq -r '.file_path // empty')
    [ -z "$file_path" ] && exit 0

    if [ -n "$SESSION_FILTER" ]; then
      state_file=$(resolve_state_file "$SESSION_FILTER")
    else
      state_file=$(resolve_for_file "$file_path")
    fi

    session_id=$(get_session_id "$state_file")
    cmd_file="$SESSIONS_DIR/${session_id}.commands.json"
    cmd_id="cmd-$$-$(date +%s)"

    # Use jq to safely encode multiline strings into the command JSON
    echo "$preview_input" | jq \
      --arg id "$cmd_id" \
      '{version: 1, id: $id, timestamp: (now * 1000 | floor), command: "previewEdit", args: .}' \
      > "$cmd_file"
    ;;

  # ── close-preview ──
  close-preview)
    check_jq
    shift
    file_path="${1:-}"
    if [ -n "$SESSION_FILTER" ]; then
      state_file=$(resolve_state_file "$SESSION_FILTER")
    elif [ -n "$file_path" ]; then
      state_file=$(resolve_for_file "$file_path")
    else
      state_file=$(resolve_state_file "")
    fi

    session_id=$(get_session_id "$state_file")
    cmd_file="$SESSIONS_DIR/${session_id}.commands.json"
    cmd_id="cmd-$$-$(date +%s)"

    cat > "$cmd_file" <<EOF
{
  "version": 1,
  "id": "${cmd_id}",
  "timestamp": $(date +%s)000,
  "command": "closePreview",
  "args": {}
}
EOF
    ;;

  # ── diagnostics ──
  diagnostics)
    check_jq
    shift
    state_file=$(resolve_state_file "$SESSION_FILTER")
    if [ $# -gt 0 ]; then
      send_command "$state_file" "getDiagnostics" "{\"path\": \"$1\"}"
    else
      send_command "$state_file" "getDiagnostics" "{}"
    fi
    ;;

  # ── watch ──
  watch)
    check_jq
    state_file=$(resolve_state_file "$SESSION_FILTER")
    echo "Watching editor state... (Ctrl-C to stop)"
    if command -v fswatch &>/dev/null; then
      jq -c '{file: .activeFile.relativePath, line: .activeFile.cursorLine, selection: (.selection.text // "" | .[0:80])}' "$state_file" 2>/dev/null
      fswatch -o "$state_file" | while read -r _; do
        jq -c '{file: .activeFile.relativePath, line: .activeFile.cursorLine, selection: (.selection.text // "" | .[0:80])}' "$state_file" 2>/dev/null
      done
    else
      echo "Install fswatch for live watching: brew install fswatch" >&2
      echo "Falling back to polling (1s interval)..."
      prev=""
      while true; do
        curr=$(jq -c '{file: .activeFile.relativePath, line: .activeFile.cursorLine}' "$state_file" 2>/dev/null)
        if [ "$curr" != "$prev" ]; then
          echo "$curr"
          prev="$curr"
        fi
        sleep 1
      done
    fi
    ;;

  # ── hook:context ──
  hook:context)
    # UserPromptSubmit hook: prints editor context summary for Claude.
    [ -t 0 ] && exit 0  # not called from a Claude hook — stdin is a terminal
    command -v jq &>/dev/null || exit 0
    payload=$(cat)
    event=$(echo "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)
    [ "$event" = "UserPromptSubmit" ] || exit 0
    # Skip when launched from the VS Code/Cursor extension.
    [ -n "${CLAUDE_CODE_SSE_PORT:-}" ] && exit 0

    now_ms=$(($(date +%s) * 1000))
    active_files=()
    active_sids=()

    if [ -d "$SESSIONS_DIR" ]; then
      for f in "$SESSIONS_DIR"/*.json; do
        [ -f "$f" ] || continue
        [[ "$f" == *.commands.json ]] && continue
        [[ "$f" == *.commands-result.json ]] && continue
        timestamp=$(jq -r '.timestamp // 0' "$f" 2>/dev/null) || continue
        age_ms=$((now_ms - timestamp))
        [ "$age_ms" -gt "$STALENESS_MS" ] && continue
        sid=$(jq -r '.sessionId // empty' "$f" 2>/dev/null)
        [ -z "$sid" ] && sid=$(basename "$f" .json)
        active_files+=("$f")
        active_sids+=("$sid")
      done
    fi

    cwd="$(pwd)"
    has_binding=false
    if [ -f "$BINDINGS_FILE" ]; then
      existing=$(jq -r --arg cwd "$cwd" '.[$cwd] // empty' "$BINDINGS_FILE" 2>/dev/null)
      [ -n "$existing" ] && has_binding=true
    fi

    if [ "$has_binding" = false ] && [ ${#active_files[@]} -gt 0 ]; then
      if [ ${#active_files[@]} -eq 1 ]; then
        bind_sid="${active_sids[0]}"
        bindings=$([ -f "$BINDINGS_FILE" ] && cat "$BINDINGS_FILE" || echo '{}')
        bindings=$(echo "$bindings" | jq --arg cwd "$cwd" --arg sid "$bind_sid" '. + {($cwd): $sid}')
        mkdir -p "$(dirname "$BINDINGS_FILE")"
        echo "$bindings" > "$BINDINGS_FILE"
        echo "[Editor Bridge] Auto-bound ${cwd} → ${bind_sid}"
      else
        workspace_matches=()
        for i in "${!active_files[@]}"; do
          f="${active_files[$i]}"
          folders=$(jq -r '.workspace.folders[]? // empty' "$f" 2>/dev/null)
          while IFS= read -r folder; do
            [ -z "$folder" ] && continue
            folder_slash="${folder%/}/"
            if [[ "$cwd" == "$folder_slash"* ]] || [[ "$cwd" == "$folder" ]]; then
              workspace_matches+=("${active_sids[$i]}")
              break
            fi
          done <<< "$folders"
        done
        unique_matches=($(printf '%s\n' "${workspace_matches[@]}" | sort -u))
        if [ ${#unique_matches[@]} -eq 1 ]; then
          bind_sid="${unique_matches[0]}"
          bindings=$([ -f "$BINDINGS_FILE" ] && cat "$BINDINGS_FILE" || echo '{}')
          bindings=$(echo "$bindings" | jq --arg cwd "$cwd" --arg sid "$bind_sid" '. + {($cwd): $sid}')
          echo "$bindings" > "$BINDINGS_FILE"
          echo "[Editor Bridge] Auto-bound ${cwd} → ${bind_sid} (workspace match)"
        elif [ ${#unique_matches[@]} -gt 1 ]; then
          echo "[Editor Bridge] Multiple editors match this workspace. Run: bridget bind"
        fi
      fi
    fi

    printed=0
    for i in "${!active_files[@]}"; do
      f="${active_files[$i]}"
      sid="${active_sids[$i]}"
      ide=$(jq -r '.ideName // "Editor"' "$f" 2>/dev/null)
      abs_path=$(jq -r '.activeFile.path // empty' "$f" 2>/dev/null)
      file=$(jq -r '.activeFile.relativePath // empty' "$f" 2>/dev/null)
      lang=$(jq -r '.activeFile.languageId // empty' "$f" 2>/dev/null)
      line=$(jq -r '.activeFile.cursorLine // empty' "$f" 2>/dev/null)
      sel_start=$(jq -r '.selection.startLine // empty' "$f" 2>/dev/null)
      sel_end=$(jq -r '.selection.endLine // empty' "$f" 2>/dev/null)
      sel_text=$(jq -r '.selection.text // empty' "$f" 2>/dev/null)
      [ -z "$file" ] && continue
      echo "[Editor Bridge] ${ide} (${sid}) — ${abs_path} (${lang}), cursor line ${line}"
      if [ -n "$sel_text" ]; then
        sel_len=${#sel_text}
        echo "  Selection: lines ${sel_start}-${sel_end} (${sel_len} chars)"
        if [ "$sel_len" -le 500 ]; then
          echo "  ---"
          echo "$sel_text"
          echo "  ---"
        else
          echo "  (use editor_get_selection tool to read full selection)"
        fi
      fi
      printed=$((printed + 1))
    done

    if [ "$printed" -eq 0 ] && [ -f "$LEGACY_STATE_FILE" ]; then
      timestamp=$(jq -r '.timestamp // 0' "$LEGACY_STATE_FILE" 2>/dev/null)
      age_ms=$((now_ms - timestamp))
      [ "$age_ms" -gt "$STALENESS_MS" ] && exit 0
      ide=$(jq -r '.ideName // "Editor"' "$LEGACY_STATE_FILE" 2>/dev/null)
      abs_path=$(jq -r '.activeFile.path // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      file=$(jq -r '.activeFile.relativePath // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      lang=$(jq -r '.activeFile.languageId // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      line=$(jq -r '.activeFile.cursorLine // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      sel_start=$(jq -r '.selection.startLine // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      sel_end=$(jq -r '.selection.endLine // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      sel_text=$(jq -r '.selection.text // empty' "$LEGACY_STATE_FILE" 2>/dev/null)
      [ -z "$file" ] && exit 0
      echo "[Editor Bridge] ${ide} — ${abs_path} (${lang}), cursor line ${line}"
      if [ -n "$sel_text" ]; then
        sel_len=${#sel_text}
        echo "  Selection: lines ${sel_start}-${sel_end} (${sel_len} chars)"
        if [ "$sel_len" -le 500 ]; then
          echo "  ---"
          echo "$sel_text"
          echo "  ---"
        else
          echo "  (use editor_get_selection tool to read full selection)"
        fi
      fi
    fi
    ;;

  # ── hook:preview ──
  hook:preview)
    # PreToolUse hook: shows a preview diff in the editor before edits are applied.
    [ -t 0 ] && exit 0  # not called from a Claude hook — stdin is a terminal
    command -v jq &>/dev/null || exit 0
    payload=$(cat)
    event=$(echo "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)
    [ "$event" = "PreToolUse" ] || exit 0
    [ -n "${CLAUDE_CODE_SSE_PORT:-}" ] && exit 0
    tool_name=$(echo "$payload" | jq -r '.tool_name // empty')
    case "$tool_name" in
      Edit|Write) ;;
      *) exit 0 ;;
    esac
    file_path=$(echo "$payload" | jq -r '.tool_input.file_path // empty')
    [ -z "$file_path" ] && exit 0
    echo "$payload" | jq '.tool_input + {tool_name: .tool_name}' | "$0" preview &
    exit 0
    ;;

  # ── hook:edit ──
  hook:edit)
    # PostToolUse hook: closes the preview diff tab after an edit is accepted/rejected.
    [ -t 0 ] && exit 0  # not called from a Claude hook — stdin is a terminal
    command -v jq &>/dev/null || exit 0
    payload=$(cat)
    event=$(echo "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)
    [ "$event" = "PostToolUse" ] || exit 0
    [ -n "${CLAUDE_CODE_SSE_PORT:-}" ] && exit 0
    tool_name=$(echo "$payload" | jq -r '.tool_name // empty')
    case "$tool_name" in
      Edit|Write) ;;
      *) exit 0 ;;
    esac
    file_path=$(echo "$payload" | jq -r '.tool_input.file_path // empty')
    [ -z "$file_path" ] && exit 0
    "$0" close-preview "$file_path" >/dev/null 2>&1 &
    exit 0
    ;;

  # ── help ──
  help|--help|-h)
    cat <<HELP
Bridget

Usage: bridget <command> [args] [-s session]

Global flags:
  -s, --session ID    Target a specific editor session (partial match OK)

Session management:
  sessions              List all active editor sessions
  bind [session]        Bind CWD to an editor session (interactive if omitted)
  unbind                Remove binding for CWD
  bindings              List all CWD → editor bindings

Editor commands:
  status                Show full editor state (JSON)
  file                  Print active file path
  selection             Print current text selection
  open <path> [line]    Open a file in the editor
  reveal <path> <line>  Scroll to a line (no focus change)
  diff <path>           Show git diff for a file in the editor
  diagnostics [path]    Get LSP diagnostics
  watch                 Stream editor state changes

Hook commands (for use in ~/.claude/settings.json):
  hook:context          UserPromptSubmit: inject editor context into prompt
  hook:preview          PreToolUse: show preview diff before edit is applied
  hook:edit             PostToolUse: close preview diff after edit completes

  help                  Show this help

Examples:
  bridget sessions
  bridget bind                         # interactive session picker
  bridget bind Cursor-5800             # bind CWD to specific session
  bridget bindings                     # list all bindings
  bridget diff src/main.ts             # auto-routes via binding
  bridget file -s Cursor               # explicit session override
  bridget open src/main.ts 42
HELP
    ;;

  *)
    echo "Unknown command: $1 (try 'bridget help')" >&2
    exit 1
    ;;
esac
