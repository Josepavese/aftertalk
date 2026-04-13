#!/usr/bin/env bash
# Step: Generate config.yaml using --dump-defaults (SSOT: binary owns defaults).
# Requires: AFTERTALK_BIN, AFTERTALK_CONFIG, PYTHON, WHISPER_MODEL,
#           WHISPER_LANGUAGE, OLLAMA_MODEL, AFTERTALK_DATA (from installer env)

step_config() {
  header "Config"
  local config_file="$AFTERTALK_CONFIG/config.yaml"

  if [[ -f "$config_file" ]]; then
    info "Config exists, skipping (delete $config_file to regenerate)"
    return
  fi

  # Generate base config from the binary — single source of truth for defaults.
  "$AFTERTALK_BIN/aftertalk-server" --dump-defaults > "$config_file"

  # Patch site-specific values using Python (already a dependency).
  local api_key jwt_secret turn_secret
  api_key=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)
  jwt_secret=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48)
  turn_secret=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48)

  "$PYTHON" - <<PYEOF
import sys
try:
    import yaml
except ImportError:
    sys.exit(0)  # PyYAML unavailable — keep raw defaults

with open("$config_file") as f:
    cfg = yaml.safe_load(f) or {}

cfg.setdefault('database', {})['path'] = '$AFTERTALK_DATA/aftertalk.db'
cfg.setdefault('api', {})['key'] = '$api_key'
cfg.setdefault('jwt', {})['secret'] = '$jwt_secret'
mode = '${INSTALL_MODE:-local-ai}'
if mode == 'local-ai':
    cfg.setdefault('stt', {})['provider'] = 'whisper-local'
    cfg['stt'].setdefault('whisper_local', {}).update({
        'url':             'http://localhost:9001',
        'model':           '${WHISPER_MODEL}',
        'language':        '${WHISPER_LANGUAGE}',
        'response_format': 'verbose_json',
        'endpoint':        '/inference',
    })
    cfg.setdefault('llm', {})['provider'] = 'ollama'
    cfg['llm'].setdefault('ollama', {}).update({
        'base_url': 'http://localhost:11434',
        'model':    '${OLLAMA_MODEL}',
    })
elif mode == 'cloud':
    cfg.setdefault('stt', {})['provider'] = 'google'  # operator fills credentials
    cfg.setdefault('llm', {})['provider'] = 'openai'  # operator fills API key
else:  # offline / stub
    cfg.setdefault('stt', {})['provider'] = 'stub'
    cfg.setdefault('llm', {})['provider'] = 'stub'
cfg.setdefault('webrtc', {}).setdefault('turn', {})['auth_secret'] = '$turn_secret'

with open("$config_file", 'w') as f:
    yaml.dump(cfg, f, default_flow_style=False, allow_unicode=True)
PYEOF

  success "Config: $config_file"
}
