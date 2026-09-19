# Cursor hook: append per-turn token usage to ~/.agentsview/cursor-hook-usage.jsonl.
# Windows: Cursor pipes JSON from a temp file; stdin may include a BOM — extract {...} before parsing.
# stdout: {} for stop/subagentStop; afterAgentResponse/preCompact accept empty output.
$ErrorActionPreference = 'Stop'

function Write-HookStdout {
    [Console]::Out.WriteLine('{}')
}

function Read-HookPayloadRaw {
    $chunks = @($input)
    if ($chunks.Count -gt 0) {
        $fromPipeline = ($chunks | ForEach-Object { "$_" }) -join ''
        if (-not [string]::IsNullOrWhiteSpace($fromPipeline)) {
            return $fromPipeline
        }
    }

    if ($args.Count -gt 0 -and -not [string]::IsNullOrWhiteSpace([string]$args[0])) {
        return [string]$args[0]
    }

    return [Console]::In.ReadToEnd()
}

function Get-JsonObjectSlice {
    param([string]$Text)

    if ([string]::IsNullOrWhiteSpace($Text)) {
        return $null
    }

    $t = $Text.Trim()
    if ($t.Length -ge 1 -and [int][char]$t[0] -eq 0xFEFF) {
        $t = $t.Substring(1).Trim()
    }

    $start = $t.IndexOf('{')
    $end = $t.LastIndexOf('}')
    if ($start -lt 0 -or $end -le $start) {
        return $null
    }

    return $t.Substring($start, $end - $start + 1)
}

function Convert-HookWorkspaceRoot {
    param([string]$Root)

    if ([string]::IsNullOrWhiteSpace($Root)) {
        return $null
    }

    $r = $Root.Trim()
    if ($r -match '^/([a-zA-Z]):/(.*)$') {
        return '{0}:\{1}' -f $Matches[1], ($Matches[2] -replace '/', '\')
    }
    if ($r -match '^/([a-zA-Z]):$') {
        return '{0}:\' -f $Matches[1]
    }

    return $r
}

function Get-PayloadValue {
    param(
        $Payload,
        [string[]]$Names
    )

    if ($null -eq $Payload) {
        return $null
    }

    foreach ($name in $Names) {
        $prop = $Payload.PSObject.Properties[$name]
        if ($null -ne $prop -and $null -ne $prop.Value) {
            return $prop.Value
        }
    }

    return $null
}

function Get-PayloadInt {
    param(
        $Payload,
        [string[]]$Names
    )

    $raw = Get-PayloadValue -Payload $Payload -Names $Names
    if ($null -eq $raw) {
        return $null
    }

    if ($raw -is [int] -or $raw -is [long] -or $raw -is [decimal] -or $raw -is [double]) {
        return [int64]$raw
    }

    $s = [string]$raw
    if ([string]::IsNullOrWhiteSpace($s)) {
        return $null
    }

    $parsed = 0
    if ([int64]::TryParse($s, [ref]$parsed)) {
        return $parsed
    }

    return $null
}

function Get-TokenFieldsFromPayload {
    param($Payload)

    $usage = Get-PayloadValue -Payload $Payload -Names @('usage', 'token_usage', 'tokenUsage')

    function Get-Int([string[]]$Names) {
        $v = Get-PayloadInt -Payload $Payload -Names $Names
        if ($null -ne $v) { return $v }
        if ($null -ne $usage) {
            return Get-PayloadInt -Payload $usage -Names $Names
        }
        return $null
    }

    return [ordered]@{
        input_tokens        = (Get-Int @('input_tokens', 'inputTokens'))
        output_tokens       = (Get-Int @('output_tokens', 'outputTokens'))
        cache_read_tokens   = (Get-Int @('cache_read_tokens', 'cacheReadTokens'))
        cache_write_tokens  = (Get-Int @('cache_write_tokens', 'cacheWriteTokens'))
        context_tokens      = (Get-Int @('context_tokens', 'contextTokens'))
        context_window_size = (Get-Int @('context_window_size', 'contextWindowSize'))
    }
}

function Resolve-AgentsviewDataDir {
    if (-not [string]::IsNullOrWhiteSpace($env:AGENTSVIEW_DATA_DIR)) {
        return $env:AGENTSVIEW_DATA_DIR.Trim()
    }
    return Join-Path $env:USERPROFILE '.agentsview'
}

function Resolve-GlobalHookLogPath {
    $dataDir = Resolve-AgentsviewDataDir
    if (-not (Test-Path -LiteralPath $dataDir)) {
        New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
    }
    return Join-Path $dataDir 'cursor-hook-usage.jsonl'
}

function Append-Utf8NoBomLine {
    param(
        [string]$Path,
        [string]$Line
    )

    $dir = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    $enc = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::AppendAllText($Path, $Line, $enc)
}

try {
    $raw = Read-HookPayloadRaw
    $json = Get-JsonObjectSlice -Text $raw
    if ([string]::IsNullOrWhiteSpace($json)) {
        Write-HookStdout
        exit 0
    }

    $payload = $json | ConvertFrom-Json

    $workspaceRoot = $null
    if ($payload.workspace_roots -and $payload.workspace_roots.Count -gt 0) {
        $workspaceRoot = Convert-HookWorkspaceRoot -Root ([string]$payload.workspace_roots[0])
    }

    $tokens = Get-TokenFieldsFromPayload -Payload $payload

    $row = [ordered]@{
        recorded_at           = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
        hook_event_name       = $payload.hook_event_name
        conversation_id       = $payload.conversation_id
        generation_id         = $payload.generation_id
        model                 = $payload.model
        model_id              = $payload.model_id
        status                = $payload.status
        loop_count            = $payload.loop_count
        subagent_type         = $payload.subagent_type
        input_tokens          = $tokens.input_tokens
        output_tokens         = $tokens.output_tokens
        cache_read_tokens     = $tokens.cache_read_tokens
        cache_write_tokens    = $tokens.cache_write_tokens
        context_tokens        = $tokens.context_tokens
        context_window_size   = $tokens.context_window_size
        transcript_path       = $payload.transcript_path
        agent_transcript_path = $payload.agent_transcript_path
        workspace_root        = $workspaceRoot
        cursor_version        = $payload.cursor_version
    }

    $line = ($row | ConvertTo-Json -Compress) + "`n"
    $logPath = Resolve-GlobalHookLogPath
    Append-Utf8NoBomLine -Path $logPath -Line $line

    if ($env:CURSOR_TOKEN_HOOK_DEBUG -eq '1') {
        $debugDir = Resolve-AgentsviewDataDir
        $debugPath = Join-Path $debugDir 'cursor-hook-usage-debug.jsonl'
        $debugRow = [ordered]@{
            recorded_at     = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
            raw_input_len   = if ($null -eq $raw) { 0 } else { $raw.Length }
            parsed_json_len = $json.Length
            payload         = $payload
        }
        ($debugRow | ConvertTo-Json -Compress -Depth 20) + "`n" | Out-File -FilePath $debugPath -Encoding utf8 -Append
    }
}
catch {
    $errDir = Resolve-AgentsviewDataDir
    if (-not (Test-Path -LiteralPath $errDir)) {
        New-Item -ItemType Directory -Path $errDir -Force | Out-Null
    }
    $errPath = Join-Path $errDir 'cursor-hook-errors.log'
    "$( (Get-Date).ToUniversalTime().ToString('o') ) $($_.Exception.Message)`n" | Out-File -FilePath $errPath -Encoding utf8 -Append
}

Write-HookStdout
exit 0
