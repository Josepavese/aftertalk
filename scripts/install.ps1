# ============================================================================
# Aftertalk Installer — Windows (PowerShell 5.1+ / PowerShell 7+)
# ============================================================================
# Quick install (run in PowerShell as Administrator):
#   irm https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.ps1 | iex
#
# Environment overrides (set before running):
#   $env:AFTERTALK_HOME     install directory (default: %LOCALAPPDATA%\aftertalk)
#   $env:AFTERTALK_RELEASE  GitHub release to download (default: latest, use "edge" for master builds)
#   $env:WHISPER_MODEL      faster-whisper model (default: base)
#   $env:WHISPER_LANGUAGE   transcription language e.g. "it" (default: auto)
#   $env:OLLAMA_MODEL       LLM model (default: qwen3:4b)
#   $env:SKIP_OLLAMA        set to "1" to skip Ollama
#   $env:SKIP_WHISPER       set to "1" to skip Whisper
# ============================================================================
#Requires -Version 5.1
$ErrorActionPreference = "Stop"

# ── PAL: Platform Layer — Windows specifics ───────────────────────────────
$AT_OS   = "windows"
$AT_ARCH = if ($env:PROCESSOR_ARCHITECTURE -eq "AMD64") { "amd64" } else { "arm64" }

# ── Configuration ──────────────────────────────────────────────────────────
$AFTERTALK_HOME    = if ($env:AFTERTALK_HOME) { $env:AFTERTALK_HOME } else { Join-Path $env:LOCALAPPDATA "aftertalk" }
$AFTERTALK_RELEASE = if ($env:AFTERTALK_RELEASE) { $env:AFTERTALK_RELEASE } else { "latest" }
$WHISPER_MODEL     = if ($env:WHISPER_MODEL) { $env:WHISPER_MODEL } else { "base" }
$WHISPER_LANGUAGE  = if ($env:WHISPER_LANGUAGE) { $env:WHISPER_LANGUAGE } else { "" }
$OLLAMA_MODEL      = if ($env:OLLAMA_MODEL) { $env:OLLAMA_MODEL } else { "qwen3:4b" }

$BIN_DIR    = Join-Path $AFTERTALK_HOME "bin"
$DATA_DIR   = Join-Path $AFTERTALK_HOME "data"
$LOGS_DIR   = Join-Path $AFTERTALK_HOME "logs"
$CONFIG_DIR = Join-Path $AFTERTALK_HOME "config"
$MODELS_DIR = Join-Path $AFTERTALK_HOME "models\whisper"

function Write-Header { param($msg) Write-Host "`n=== $msg ===" -ForegroundColor Cyan }
function Write-OK     { param($msg) Write-Host "  [OK] $msg" -ForegroundColor Green }
function Write-Info   { param($msg) Write-Host "  [..] $msg" -ForegroundColor Blue }
function Write-Warn   { param($msg) Write-Host "  [!]  $msg" -ForegroundColor Yellow }
function Write-Fail   { param($msg) Write-Host "  [X]  $msg" -ForegroundColor Red; exit 1 }

function Get-ReleaseBaseUrl {
  param([string]$Release)
  if ($Release -eq "latest") {
    return "https://github.com/Josepavese/aftertalk/releases/latest/download"
  }
  return "https://github.com/Josepavese/aftertalk/releases/download/$Release"
}

function Get-ReleaseChecksums {
  param([string]$BaseUrl, [string]$Release)

  $map = @{}
  $tmp = Join-Path $env:TEMP ("aftertalk-checksums-" + [guid]::NewGuid().ToString("N") + ".txt")
  try {
    Invoke-WebRequest "$BaseUrl/checksums.txt" -OutFile $tmp -UseBasicParsing
    foreach ($line in Get-Content $tmp) {
      $parts = $line -split "\s+", 2
      if ($parts.Count -eq 2) {
        $map[$parts[1].Trim()] = $parts[0].Trim().ToLowerInvariant()
      }
    }
  } catch {
    Write-Warn "checksums.txt not available for $Release; downloads will not be hash-verified"
  } finally {
    Remove-Item $tmp -EA SilentlyContinue
  }
  return $map
}

function Verify-ReleaseChecksum {
  param($Checksums, [string]$Asset, [string]$Path)

  if (-not $Checksums.ContainsKey($Asset)) {
    Write-Warn "checksum missing for $Asset; skipping"
    return
  }

  $actual = (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLowerInvariant()
  $expected = $Checksums[$Asset]
  if ($actual -ne $expected) {
    Write-Fail "Checksum mismatch for $Asset`nexpected: $expected`nactual:   $actual"
  }
  Write-OK "Checksum verified: $Asset"
}

Write-Host @"
  ╔═══════════════════════════════════╗
  ║   Aftertalk Installer $AFTERTALK_RELEASE
  ║  AI meeting minutes, local-first  ║
  ╚═══════════════════════════════════╝
"@ -ForegroundColor Green
Write-Info "OS: $AT_OS / $AT_ARCH"
Write-Info "Install home: $AFTERTALK_HOME"

# ── 1. Prerequisites ──────────────────────────────────────────────────────
Write-Header "1. Prerequisites"

# Detect package manager (winget preferred, then choco, then scoop)
$PKG = if (Get-Command winget -EA SilentlyContinue) { "winget" }
       elseif (Get-Command choco -EA SilentlyContinue) { "choco" }
       elseif (Get-Command scoop -EA SilentlyContinue) { "scoop" }
       else { "none" }
Write-Info "Package manager: $PKG"

function Install-Tool {
  param($Name, $WingetId, $ChocoId, $ScoopId)
  Write-Info "Installing $Name..."
  switch ($PKG) {
    "winget" { winget install --id $WingetId --accept-package-agreements --accept-source-agreements -e }
    "choco"  { choco install $ChocoId -y }
    "scoop"  { scoop install $ScoopId }
    default  { Write-Fail "No package manager found. Install $Name manually from its website." }
  }
}

# python3
$PYTHON = Get-Command python -EA SilentlyContinue | Select-Object -ExpandProperty Source
if (-not $PYTHON) {
  Install-Tool "Python 3" "Python.Python.3.11" "python3" "python"
  $PYTHON = Get-Command python -EA SilentlyContinue | Select-Object -ExpandProperty Source
}
$PY_VER = & $PYTHON --version 2>&1
Write-OK "python: $PY_VER"

# pip
& $PYTHON -m pip --version | Out-Null
Write-OK "pip: $(& $PYTHON -m pip --version)"

# ffmpeg
if (-not (Get-Command ffmpeg -EA SilentlyContinue)) {
  Write-Warn "Installing ffmpeg..."
  Install-Tool "ffmpeg" "Gyan.FFmpeg" "ffmpeg" "ffmpeg"
}
Write-OK "ffmpeg: installed"

# ── 2. faster-whisper ─────────────────────────────────────────────────────
if ($env:SKIP_WHISPER -ne "1") {
  Write-Header "2. Whisper (faster-whisper)"
  $fw_check = & $PYTHON -c "import faster_whisper; print(faster_whisper.__version__)" 2>$null
  if (-not $fw_check) {
    Write-Info "Installing faster-whisper..."
    & $PYTHON -m pip install faster-whisper
  }
  Write-OK "faster-whisper: $(& $PYTHON -c 'import faster_whisper; print(faster_whisper.__version__)')"
}

# ── 3. Ollama ─────────────────────────────────────────────────────────────
if ($env:SKIP_OLLAMA -ne "1") {
  Write-Header "3. Ollama LLM"
  if (-not (Get-Command ollama -EA SilentlyContinue)) {
    Write-Info "Installing Ollama..."
    $installer = Join-Path $env:TEMP "OllamaSetup.exe"
    Invoke-WebRequest "https://ollama.com/download/OllamaSetup.exe" -OutFile $installer
    Start-Process $installer -Wait -ArgumentList "/S"
    Remove-Item $installer
    $env:PATH = "$env:LOCALAPPDATA\Programs\Ollama;" + $env:PATH
  }
  Write-OK "ollama: $(ollama --version 2>$null | Select-Object -First 1)"

  $svc = Get-Process ollama -EA SilentlyContinue
  if (-not $svc) {
    Write-Info "Starting Ollama..."
    Start-Process ollama -ArgumentList "serve" -WindowStyle Hidden
    Start-Sleep 3
  }
  $models = & ollama list 2>$null
  if ($models -notmatch [regex]::Escape($OLLAMA_MODEL)) {
    Write-Info "Pulling $OLLAMA_MODEL ..."
    ollama pull $OLLAMA_MODEL
  }
  Write-OK "model: $OLLAMA_MODEL ready"
}

# ── 4. Directories ────────────────────────────────────────────────────────
Write-Header "4. Home: $AFTERTALK_HOME"
foreach ($d in @($BIN_DIR, $DATA_DIR, $LOGS_DIR, $CONFIG_DIR, $MODELS_DIR)) {
  New-Item -ItemType Directory -Path $d -Force | Out-Null
}
Write-OK "Directories created"

# ── 5. Binary download ────────────────────────────────────────────────────
Write-Header "5. Aftertalk binary ($AT_OS/$AT_ARCH, release: $AFTERTALK_RELEASE)"

$BIN_NAME = "aftertalk-${AT_OS}-${AT_ARCH}.exe"
$BASE_URL = Get-ReleaseBaseUrl $AFTERTALK_RELEASE
$BIN_URL = "$BASE_URL/$BIN_NAME"
$WHISPER_URL = "$BASE_URL/whisper_server.py"
$CHECKSUMS = Get-ReleaseChecksums $BASE_URL $AFTERTALK_RELEASE

Write-Info "Downloading aftertalk-server.exe..."
try {
  $SERVER_PATH = Join-Path $BIN_DIR "aftertalk-server.exe"
  Invoke-WebRequest $BIN_URL -OutFile $SERVER_PATH -UseBasicParsing
  Verify-ReleaseChecksum $CHECKSUMS $BIN_NAME $SERVER_PATH
  Write-OK "Binary: $BIN_DIR\aftertalk-server.exe"
  & $SERVER_PATH --version
} catch {
  Write-Fail "Failed to download binary from $BIN_URL`nCheck https://github.com/Josepavese/aftertalk/releases or set AFTERTALK_RELEASE=edge"
}

Write-Info "Downloading whisper_server.py..."
$WHISPER_PATH = Join-Path $BIN_DIR "whisper_server.py"
$WHISPER_FROM_RELEASE = $false
try {
  Invoke-WebRequest $WHISPER_URL -OutFile $WHISPER_PATH -UseBasicParsing
  $WHISPER_FROM_RELEASE = $true
} catch {
  # Fallback to raw source
  Invoke-WebRequest "https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/whisper_server.py" `
    -OutFile $WHISPER_PATH -UseBasicParsing
}
if ($WHISPER_FROM_RELEASE) {
  Verify-ReleaseChecksum $CHECKSUMS "whisper_server.py" $WHISPER_PATH
}
Write-OK "Whisper server: $BIN_DIR\whisper_server.py"

# ── 6. Config ─────────────────────────────────────────────────────────────
Write-Header "6. Configuration"
$CONFIG_FILE = Join-Path $CONFIG_DIR "config.yaml"
if (-not (Test-Path $CONFIG_FILE)) {
  $api_key    = -join ((65..90 + 97..122 + 48..57) | Get-Random -Count 32 | % { [char]$_ })
  $jwt_secret = -join ((65..90 + 97..122 + 48..57) | Get-Random -Count 48 | % { [char]$_ })
  $db_path    = (Join-Path $DATA_DIR "aftertalk.db") -replace "\\", "/"
  $log_path   = (Join-Path $LOGS_DIR "aftertalk.jsonl") -replace "\\", "/"
  @"
database:
  path: $db_path

http:
  host: 0.0.0.0
  port: 8080

logging:
  level: info
  format: json
  output:
    stdout: true
    file:
      enabled: true
      path: $log_path
  rotation:
    max_size_mb: 100
    max_age_days: 30
    max_backups: 20
    compress: true
  retention:
    delete_after_days: 90
    emergency_cutoff_size_mb: 2048
  redaction:
    enabled: true
    fields:
      - api_key
      - token
      - authorization
      - secret
      - password
      - webhook_payload
      - transcript_text
      - minutes
      - raw_provider_payload

api:
  key: $api_key

jwt:
  secret: $jwt_secret
  issuer: aftertalk
  expiration: 2h

stt:
  provider: whisper-local
  whisper_local:
    url: http://localhost:9001
    model: $WHISPER_MODEL
    language: $WHISPER_LANGUAGE
    response_format: verbose_json
    endpoint: /inference

llm:
  provider: ollama
  ollama:
    base_url: http://localhost:11434
    model: $OLLAMA_MODEL

processing:
  chunk_size_ms: 15000
"@ | Set-Content $CONFIG_FILE
  Write-OK "Config: $CONFIG_FILE"
} else {
  Write-Warn "Config exists, skipping"
}

# ── 7. CLI wrapper ────────────────────────────────────────────────────────
Write-Header "7. CLI command"

$START_BAT = Join-Path $BIN_DIR "aftertalk.bat"
@"
@echo off
setlocal
set AFTERTALK_HOME=$AFTERTALK_HOME
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0aftertalk.ps1" %*
"@ | Set-Content $START_BAT

$CLI_PS1 = Join-Path $BIN_DIR "aftertalk.ps1"
@'
param([string]$Command = "start", [string]$Service = "aftertalk")
$ErrorActionPreference = "Stop"
$HOME_DIR = if ($env:AFTERTALK_HOME) { $env:AFTERTALK_HOME } else { Join-Path $env:LOCALAPPDATA "aftertalk" }
$BIN     = Join-Path $HOME_DIR "bin"
$LOGS    = Join-Path $HOME_DIR "logs"
$CONFIG  = Join-Path $HOME_DIR "config\config.yaml"
$MODELS  = Join-Path $HOME_DIR "models\whisper"

function Get-Pid-File { param($name) Join-Path $HOME_DIR "$name.pid" }
function Is-Running { param($name)
  $pf = Get-Pid-File $name
  if (-not (Test-Path $pf)) { return $false }
  $pid_ = Get-Content $pf
  return (Get-Process -Id $pid_ -EA SilentlyContinue) -ne $null
}

function Start-Stack {
  Write-Host "Starting Aftertalk stack..." -ForegroundColor Green
  if (-not (Is-Running "whisper")) {
    $p = Start-Process python -ArgumentList "`"$(Join-Path $BIN 'whisper_server.py')`"" `
           -RedirectStandardOutput "$LOGS\whisper.log" -WindowStyle Hidden -PassThru `
           -Environment @{ WHISPER_MODELS_DIR=$MODELS; PORT="9001" }
    $p.Id | Set-Content (Get-Pid-File "whisper")
    Write-Host "  [OK] whisper-server (PID $($p.Id))"
  } else { Write-Host "  [..] whisper already running" }

  $tries = 0
  do { Start-Sleep 1; $tries++ } until ((Test-NetConnection localhost -Port 9001 -WarningAction SilentlyContinue).TcpTestSucceeded -or $tries -ge 30)

  if (-not (Is-Running "aftertalk")) {
    $p = Start-Process (Join-Path $BIN "aftertalk-server.exe") -ArgumentList "--config `"$CONFIG`"" `
           -RedirectStandardOutput "$LOGS\aftertalk.log" -WindowStyle Hidden -PassThru
    $p.Id | Set-Content (Get-Pid-File "aftertalk")
    Write-Host "  [OK] aftertalk (PID $($p.Id))"
  } else { Write-Host "  [..] aftertalk already running" }

  Write-Host "`n  UI  -> http://localhost:8080" -ForegroundColor Cyan
}

function Stop-Stack {
  foreach ($svc in @("aftertalk","whisper")) {
    if (Is-Running $svc) {
      Stop-Process -Id (Get-Content (Get-Pid-File $svc)) -Force
      Remove-Item (Get-Pid-File $svc) -Force
      Write-Host "  [OK] $svc stopped"
    }
  }
}

function Show-Status {
  foreach ($svc in @("aftertalk","whisper")) {
    if (Is-Running $svc) { Write-Host "  [OK] $svc running" -ForegroundColor Green }
    else { Write-Host "  [X]  $svc stopped" -ForegroundColor Red }
  }
}
function Show-Version { & (Join-Path $BIN "aftertalk-server.exe") --version }

function Get-ReleaseBaseUrl {
  param([string]$Release)
  if ($Release -eq "latest") {
    return "https://github.com/Josepavese/aftertalk/releases/latest/download"
  }
  return "https://github.com/Josepavese/aftertalk/releases/download/$Release"
}

function Get-ReleaseChecksums {
  param([string]$BaseUrl, [string]$Release)
  $map = @{}
  $tmp = Join-Path $env:TEMP ("aftertalk-checksums-" + [guid]::NewGuid().ToString("N") + ".txt")
  try {
    Invoke-WebRequest "$BaseUrl/checksums.txt" -OutFile $tmp -UseBasicParsing
    foreach ($line in Get-Content $tmp) {
      $parts = $line -split "\s+", 2
      if ($parts.Count -eq 2) { $map[$parts[1].Trim()] = $parts[0].Trim().ToLowerInvariant() }
    }
  } catch {
    Write-Host "  [!] checksums.txt not available for $Release; downloads will not be hash-verified" -ForegroundColor Yellow
  } finally {
    Remove-Item $tmp -EA SilentlyContinue
  }
  return $map
}

function Verify-ReleaseChecksum {
  param($Checksums, [string]$Asset, [string]$Path)
  if (-not $Checksums.ContainsKey($Asset)) {
    Write-Host "  [!] checksum missing for $Asset; skipping" -ForegroundColor Yellow
    return
  }
  $actual = (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLowerInvariant()
  $expected = $Checksums[$Asset]
  if ($actual -ne $expected) {
    throw "Checksum mismatch for $Asset`nexpected: $expected`nactual:   $actual"
  }
  Write-Host "  [OK] checksum verified: $Asset" -ForegroundColor Green
}

switch ($Command) {
  "start"   { Start-Stack }
  "stop"    { Stop-Stack }
  "restart" { Stop-Stack; Start-Sleep 1; Start-Stack }
  "status"  { Show-Status }
  "version" { Show-Version }
  "--version" { Show-Version }
  "update"  {
    Stop-Stack
    $rel  = if ($env:AFTERTALK_RELEASE) { $env:AFTERTALK_RELEASE } else { "latest" }
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "AMD64") { "amd64" } else { "arm64" }
    $name = "aftertalk-windows-${arch}.exe"
    $baseUrl = Get-ReleaseBaseUrl $rel
    $binUrl = "$baseUrl/$name"
    $whUrl = "$baseUrl/whisper_server.py"
    $checksums = Get-ReleaseChecksums $baseUrl $rel
    $serverPath = Join-Path $BIN "aftertalk-server.exe"
    $whisperPath = Join-Path $BIN "whisper_server.py"
    Write-Host "Downloading update (release: $rel)..."
    Invoke-WebRequest $binUrl -OutFile $serverPath -UseBasicParsing
    Verify-ReleaseChecksum $checksums $name $serverPath
    & $serverPath --version
    $whisperFromRelease = $false
    try {
      Invoke-WebRequest $whUrl -OutFile $whisperPath -UseBasicParsing
      $whisperFromRelease = $true
    } catch {
      Invoke-WebRequest "https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/whisper_server.py" `
        -OutFile $whisperPath -UseBasicParsing
    }
    if ($whisperFromRelease) {
      Verify-ReleaseChecksum $checksums "whisper_server.py" $whisperPath
    }
    Write-Host "Updated. Run 'aftertalk start'."
  }
  default { Write-Host "Usage: aftertalk {start|stop|restart|status|update|version}" }
}
'@ | Set-Content $CLI_PS1

# Add to user PATH
$user_path = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($user_path -notmatch [regex]::Escape($BIN_DIR)) {
  [Environment]::SetEnvironmentVariable("PATH", "$BIN_DIR;$user_path", "User")
  $env:PATH = "$BIN_DIR;" + $env:PATH
  Write-OK "Added $BIN_DIR to PATH"
}

# ── Done ──────────────────────────────────────────────────────────────────
Write-Host @"

╔══════════════════════════════════════╗
║  Aftertalk installed successfully!   ║
╚══════════════════════════════════════╝

  Start:  aftertalk start
  Stop:   aftertalk stop
  Status: aftertalk status
  Update: aftertalk update

  Config: $CONFIG_FILE
  Home:   $AFTERTALK_HOME

NOTE: Open a new terminal for PATH changes to take effect.
"@ -ForegroundColor Green
