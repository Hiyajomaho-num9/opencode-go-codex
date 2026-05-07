$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot

if (-not $env:OPENCODE_GO_API_KEY -and -not $env:OPENAI_API_KEY) {
  Write-Error "Set OPENCODE_GO_API_KEY first."
}

if (Test-Path ".\opencode-go-codex.exe") {
  & ".\opencode-go-codex.exe" @args
  exit $LASTEXITCODE
}

go run .\cmd\opencode-go-codex @args
