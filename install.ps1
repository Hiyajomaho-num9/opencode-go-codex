$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot

function Say {
  param([string]$Message)
  Write-Host $Message
}

function Need-Command {
  param([string]$Name)
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Missing required command: $Name"
  }
}

function Read-ApiKey {
  if ($env:OPENCODE_GO_API_KEY) {
    return $env:OPENCODE_GO_API_KEY
  }

  $secure = Read-Host "OpenCode Go API key" -AsSecureString
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try {
    $key = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
  }

  if (-not $key) {
    throw "API key is required."
  }
  return $key
}

function Set-UserEnv {
  param([string]$Name, [string]$Value)
  [Environment]::SetEnvironmentVariable($Name, $Value, "User")
  Set-Item -Path "Env:$Name" -Value $Value
}

function Install-Skill {
  param([string]$CodexHome, [string]$McpScript)

  $skillDir = Join-Path $CodexHome "skills\web-search-mcp"
  Say "Installing web-search fallback skill: $skillDir"
  New-Item -ItemType Directory -Force -Path $skillDir | Out-Null

  $skill = @"
---
name: web-search-mcp
description: Use when native Codex web_search is unavailable with the OpenCode Go custom provider. Calls the local web_search MCP-compatible script through shell JSON-RPC.
---

Use this PowerShell JSON-RPC command to search the web:

````powershell
'{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"web_search","arguments":{"query":"QUERY HERE","max_results":5}}}' | python "$McpScript"
````

Retry once after a short sleep if the backend returns a transient network or rate-limit error.
"@

  Set-Content -Path (Join-Path $skillDir "SKILL.md") -Value $skill -Encoding UTF8
}

$root = $PSScriptRoot
$binary = Join-Path $root "opencode-go-codex.exe"
$codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME ".codex" }
$config = Join-Path $codexHome "config.toml"
$modelCatalog = Join-Path $root "examples\models\deepseek-only.json"
$mcpScript = Join-Path $root "tools\web_search_mcp.py"

Say "OpenCode Go Codex Windows installer"
Say "Project: $root"
Say "This will use opencode-go-codex.exe, write user environment variables, update Codex profiles, and install the web-search fallback skill."

if (-not (Test-Path $binary)) {
  Need-Command go
  Say "Building Go adapter: $binary"
  go build -o $binary .\cmd\opencode-go-codex
} else {
  Say "Using existing adapter binary: $binary"
}

$apiKey = Read-ApiKey

Say "Writing user environment variables"
Set-UserEnv "OPENCODE_GO_API_KEY" $apiKey
Set-UserEnv "OPENCODE_GO_CODEX_HOST" "127.0.0.1"
Set-UserEnv "OPENCODE_GO_CODEX_PORT" "8768"
Set-UserEnv "OPENCODE_GO_BASE_URL" "https://opencode.ai/zen/go/v1/chat/completions"
Set-UserEnv "OPENCODE_GO_MODEL" "deepseek-v4-pro"
Set-UserEnv "OPENCODE_GO_COMPACT_MODEL" "deepseek-v4-flash"
Set-UserEnv "OPENCODE_GO_VISION_MODEL" "kimi-k2.6"
Set-UserEnv "OPENCODE_GO_REASONING_EFFORT" "max"
Set-UserEnv "OPENCODE_GO_THINKING" "enabled"
Set-UserEnv "OPENCODE_GO_TIMEOUT" "900"
Set-UserEnv "OPENCODE_GO_DEBUG_ROUTING" "1"

Say "Updating Codex config: $config"
New-Item -ItemType Directory -Force -Path $codexHome | Out-Null
if (Test-Path $config) {
  $backup = "$config.bak.$(Get-Date -Format yyyyMMddHHmmss)"
  Copy-Item $config $backup
  Say "Backup written: $backup"
} else {
  New-Item -ItemType File -Force -Path $config | Out-Null
}

& $binary install-config $config $modelCatalog $mcpScript

Install-Skill -CodexHome $codexHome -McpScript $mcpScript

Say "Install complete."
Say "Start the adapter with: .\start.ps1"
Say "Use Codex with: codex -p deepseek-v4-pro"
Say "Use Codex with: codex -p deepseek-v4-flash"
