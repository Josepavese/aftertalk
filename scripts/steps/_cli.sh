#!/usr/bin/env bash
# Step: Generate the `aftertalk` CLI wrapper script and symlink it.
# Requires: AFTERTALK_BIN, AFTERTALK_HOME, CLI_LINK (from installer env)

step_cli() {
  header "CLI command: aftertalk"

  local wrapper="$AFTERTALK_BIN/aftertalk"
  cat > "$wrapper" <<'WRAPPER'
#!/usr/bin/env bash
# aftertalk CLI — manages the Aftertalk server stack
AFTERTALK_HOME="${AFTERTALK_HOME:-$HOME/.aftertalk}"
BIN="$AFTERTALK_HOME/bin"
LOGS="$AFTERTALK_HOME/logs"
PIDFILE_AT="$AFTERTALK_HOME/aftertalk.pid"
PIDFILE_WH="$AFTERTALK_HOME/whisper.pid"
CONFIG="$AFTERTALK_HOME/config/config.yaml"
MODELS="$AFTERTALK_HOME/models/whisper"

_is_running() { [[ -f "$1" ]] && kill -0 "$(cat "$1")" 2>/dev/null; }

cmd_start() {
  echo "▶ Starting Aftertalk stack..."

  if _is_running "$PIDFILE_WH"; then
    echo "  whisper already running (PID $(cat "$PIDFILE_WH"))"
  else
    local wpid
    wpid=$(nohup bash -c "WHISPER_MODELS_DIR='$MODELS' WHISPER_MODEL='${WHISPER_MODEL:-base}' \
      WHISPER_LANGUAGE='${WHISPER_LANGUAGE:-}' PORT='9001' \
      python3 '$BIN/whisper_server.py' >> '$LOGS/whisper.log' 2>&1 & echo \$!" 2>/dev/null)
    echo "$wpid" > "$PIDFILE_WH"
    echo "  ✓ whisper starting (PID $wpid)"
  fi

  echo -n "  Waiting for whisper"
  local tries=0
  until curl -sf "http://localhost:9001/" >/dev/null 2>&1; do
    echo -n "."; sleep 1; tries=$(( tries + 1 ))
    if [[ $tries -ge 30 ]]; then echo " timeout!"; break; fi
  done
  echo ""

  if _is_running "$PIDFILE_AT"; then
    echo "  aftertalk already running (PID $(cat "$PIDFILE_AT"))"
  else
    local apid
    apid=$(nohup bash -c "'$BIN/aftertalk-server' --config '$CONFIG' >> '$LOGS/aftertalk.log' 2>&1 & echo \$!" 2>/dev/null)
    echo "$apid" > "$PIDFILE_AT"
    echo "  ✓ aftertalk starting (PID $apid)"
  fi

  sleep 2
  echo ""; echo "  UI  → http://localhost:8080"; echo "  Log → $LOGS/"
}

cmd_stop() {
  echo "⏹ Stopping..."
  for pf in "$PIDFILE_AT" "$PIDFILE_WH"; do
    [[ ! -f "$pf" ]] && continue
    local pid; pid=$(cat "$pf")
    if kill -0 "$pid" 2>/dev/null; then kill "$pid" && echo "  ✓ stopped (PID $pid)"; fi
    rm -f "$pf"
  done
  local p9001 p8080
  p9001=$(lsof -ti:9001 2>/dev/null || true); [[ -n "$p9001" ]] && kill "$p9001" 2>/dev/null && echo "  ✓ cleared port 9001"
  p8080=$(lsof -ti:8080 2>/dev/null || true); [[ -n "$p8080" ]] && kill "$p8080" 2>/dev/null && echo "  ✓ cleared port 8080"
}

cmd_status() {
  for svc in aftertalk whisper; do
    local pf="$AFTERTALK_HOME/${svc}.pid"
    if _is_running "$pf"; then echo "  ✓ $svc   running (PID $(cat "$pf"))"
    else echo "  ✗ $svc   stopped"; fi
  done
  local api_key
  api_key=$(grep -E '^\s*key:' "$CONFIG" | awk '{print $2}' | head -1)
  curl -sf -H "Authorization: Bearer $api_key" http://localhost:8080/v1/health >/dev/null 2>&1 \
    && echo "  API: reachable ✓" || echo "  API: unreachable"
}

cmd_restart() { cmd_stop; sleep 1; cmd_start; }

cmd_update() {
  cmd_stop
  git -C "$AFTERTALK_HOME/src" pull --ff-only
  (cd "$AFTERTALK_HOME/src" && go build -o "$BIN/aftertalk-server" ./cmd/aftertalk/)
  cp "$AFTERTALK_HOME/src/scripts/whisper_server.py" "$BIN/whisper_server.py"
  echo "✓ Updated. Run: aftertalk start"
}

cmd_logs() { exec tail -n 100 -f "$LOGS/${1:-aftertalk}.log"; }

# ── Service install / uninstall (systemd on Linux, launchd on macOS) ──────────
cmd_service() {
  local action="${1:-install}"
  local os
  case "$(uname -s)" in Linux*) os=linux ;; Darwin*) os=macos ;; *) os=unknown ;; esac

  case "$os-$action" in
    linux-install)
      local unit_file="/etc/systemd/system/aftertalk.service"
      cat <<UNIT | sudo tee "$unit_file" >/dev/null
[Unit]
Description=Aftertalk Core Server
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$AFTERTALK_HOME
ExecStart=$BIN/aftertalk-server --config $CONFIG
Restart=on-failure
RestartSec=5s
StandardOutput=append:$LOGS/aftertalk.log
StandardError=append:$LOGS/aftertalk.log
Environment=HOME=$HOME

[Install]
WantedBy=multi-user.target
UNIT
      sudo systemctl daemon-reload
      sudo systemctl enable aftertalk
      echo "✓ systemd unit installed: $unit_file"
      echo "  Start with: sudo systemctl start aftertalk"
      ;;
    linux-uninstall)
      sudo systemctl stop aftertalk 2>/dev/null || true
      sudo systemctl disable aftertalk 2>/dev/null || true
      sudo rm -f /etc/systemd/system/aftertalk.service
      sudo systemctl daemon-reload
      echo "✓ systemd unit removed"
      ;;
    macos-install)
      local plist_dir="$HOME/Library/LaunchAgents"
      local plist_file="$plist_dir/io.aftertalk.server.plist"
      mkdir -p "$plist_dir"
      cat <<PLIST > "$plist_file"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>            <string>io.aftertalk.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>$BIN/aftertalk-server</string>
    <string>--config</string>
    <string>$CONFIG</string>
  </array>
  <key>RunAtLoad</key>        <true/>
  <key>KeepAlive</key>        <true/>
  <key>StandardOutPath</key>  <string>$LOGS/aftertalk.log</string>
  <key>StandardErrorPath</key><string>$LOGS/aftertalk.log</string>
  <key>WorkingDirectory</key> <string>$AFTERTALK_HOME</string>
</dict>
</plist>
PLIST
      launchctl load "$plist_file"
      echo "✓ launchd agent installed: $plist_file"
      echo "  Starts automatically on login."
      ;;
    macos-uninstall)
      local plist_file="$HOME/Library/LaunchAgents/io.aftertalk.server.plist"
      launchctl unload "$plist_file" 2>/dev/null || true
      rm -f "$plist_file"
      echo "✓ launchd agent removed"
      ;;
    *)
      echo "Usage: aftertalk service {install|uninstall}"
      echo "  Registers aftertalk as a system service (auto-start + auto-restart)."
      echo "  Supported: systemd (Linux), launchd (macOS)"
      ;;
  esac
}

case "${1:-start}" in
  start)           cmd_start ;;
  stop)            cmd_stop ;;
  restart)         cmd_restart ;;
  status)          cmd_status ;;
  update)          cmd_update ;;
  logs)            cmd_logs "${2:-aftertalk}" ;;
  service)         cmd_service "${2:-install}" ;;
  *) echo "Usage: aftertalk {start|stop|restart|status|update|logs [aftertalk|whisper]|service [install|uninstall]}" ;;
esac
WRAPPER
  chmod +x "$wrapper"

  # Symlink to /usr/local/bin (or ~/.local/bin as fallback)
  if sudo ln -sf "$wrapper" "$CLI_LINK" 2>/dev/null; then
    success "CLI symlink: $CLI_LINK -> $wrapper"
  else
    local local_bin="$HOME/.local/bin"
    mkdir -p "$local_bin"
    ln -sf "$wrapper" "$local_bin/aftertalk"
    warn "Could not write to /usr/local/bin (no sudo). Installed to $local_bin/aftertalk"
    warn "Make sure $local_bin is in your PATH:"
    warn "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
  fi
}
