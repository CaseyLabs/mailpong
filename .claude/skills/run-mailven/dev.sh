#!/usr/bin/env bash
# Launch / wait for / stop Mailven's `make dev` session for agent-driven UI checks.
# Usage (from the repo root): .claude/skills/run-mailven/dev.sh start|wait|status|stop
set -u

URL=http://localhost:34115
LOG=${MAILVEN_DEV_LOG:-${TMPDIR:-/tmp}/mailven-dev.log}
SIDFILE=${TMPDIR:-/tmp}/mailven-dev.sid
ROOT=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)

up() { [ "$(curl -s -o /dev/null -w '%{http_code}' "$URL")" = 200 ]; }
# True unless dev.sh started a session that has since exited.
alive() { [ ! -f "$SIDFILE" ] || pgrep -s "$(cat "$SIDFILE")" >/dev/null; }

case "${1:-}" in
  start)
    if up; then echo "already up at $URL"; exit 0; fi
    # Run in a new session so stop can kill wails, vite (own process group)
    # and the app binary together; the inner bash's PID is the session id.
    cd "$ROOT" || exit 1
    setsid bash -c 'echo $$ >"$1"; exec make dev' _ "$SIDFILE" >"$LOG" 2>&1 </dev/null &
    sleep 1
    echo "started (session $(cat "$SIDFILE")), log: $LOG"
    ;;
  wait)
    for _ in $(seq 1 90); do
      if up; then echo "up at $URL"; exit 0; fi
      if ! alive; then echo "make dev exited; tail of $LOG:" >&2; tail -20 "$LOG" >&2; exit 1; fi
      sleep 2
    done
    echo "not up after 180s; tail of $LOG:" >&2
    tail -20 "$LOG" >&2
    exit 1
    ;;
  status)
    if up; then echo "up at $URL"; else echo down; fi
    ;;
  stop)
    if [ -f "$SIDFILE" ]; then
      pkill -TERM -s "$(cat "$SIDFILE")"
      rm -f "$SIDFILE"
    fi
    sleep 3
    # `make dev` regenerates the runtime bindings with different file modes.
    # Revert only when nothing but modes changed, so real edits survive.
    rt=frontend/wailsjs/runtime
    if ! git -C "$ROOT" diff --quiet -- "$rt"; then
      if git -C "$ROOT" -c core.fileMode=false diff --quiet -- "$rt"; then
        git -C "$ROOT" checkout -- "$rt"
      else
        echo "left $rt alone: it has content changes" >&2
      fi
    fi
    if up; then echo "still up at $URL" >&2; exit 1; fi
    echo stopped
    ;;
  *)
    echo "usage: $0 start|wait|status|stop" >&2
    exit 2
    ;;
esac
