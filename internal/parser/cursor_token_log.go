package parser

import (
	"bufio"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.kenn.io/agentsview/internal/pathutil"
)

const cursorHookUsageFileName = "cursor-hook-usage.jsonl"

func trimCursorTokenLogLine(line string) string {
	line = strings.TrimSpace(line)
	return strings.TrimPrefix(line, "\ufeff")
}

func globalCursorHookUsageLogPath() string {
	dataDir := strings.TrimSpace(os.Getenv("AGENTSVIEW_DATA_DIR"))
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		dataDir = filepath.Join(home, ".agentsview")
	}
	return filepath.Join(dataDir, cursorHookUsageFileName)
}

type cursorTokenLogRow struct {
	RecordedAt          string `json:"recorded_at"`
	HookEventName       string `json:"hook_event_name"`
	ConversationID      string `json:"conversation_id"`
	GenerationID        string `json:"generation_id"`
	InputTokens         *int64 `json:"input_tokens"`
	OutputTokens        *int64 `json:"output_tokens"`
	CacheReadTokens     *int64 `json:"cache_read_tokens"`
	CacheWriteTokens    *int64 `json:"cache_write_tokens"`
	ContextTokens       *int64 `json:"context_tokens"`
	ContextWindowSize   *int64 `json:"context_window_size"`
	TranscriptPath      string `json:"transcript_path"`
	AgentTranscriptPath string `json:"agent_transcript_path"`
	WorkspaceRoot       string `json:"workspace_root"`
}

func cursorTokenLogCandidates(_ string) []string {
	global := globalCursorHookUsageLogPath()
	if global == "" {
		return nil
	}
	return []string{filepath.Clean(global)}
}

func cursorTokenLogCompositeMtime(cwd string) (int64, error) {
	var max int64
	for _, path := range cursorTokenLogCandidates(cwd) {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, err
		}
		if ns := info.ModTime().UnixNano(); ns > max {
			max = ns
		}
	}
	return max, nil
}

func readCursorTokenLogRows(path string) ([]cursorTokenLogRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []cursorTokenLogRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := trimCursorTokenLogLine(sc.Text())
		if line == "" {
			continue
		}
		var row cursorTokenLogRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func loadCursorTokenLogRows(cwd string) ([]cursorTokenLogRow, error) {
	var merged []cursorTokenLogRow
	for _, path := range cursorTokenLogCandidates(cwd) {
		rows, err := readCursorTokenLogRows(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read cursor token log %s: %w", path, err)
		}
		merged = append(merged, rows...)
	}
	return merged, nil
}

func cursorTokenLogPathMatches(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return true
	}
	keyA, errA := pathutil.LocalComparisonKey(a)
	keyB, errB := pathutil.LocalComparisonKey(b)
	if errA == nil && errB == nil {
		return keyA == keyB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func cursorTokenLogRowMatchesSession(
	row cursorTokenLogRow,
	agentID, transcriptPath, cwd string,
) bool {
	id := strings.TrimSpace(row.ConversationID)
	if id == "" || !strings.EqualFold(id, agentID) {
		return false
	}
	if row.TranscriptPath != "" && transcriptPath != "" &&
		!cursorTokenLogPathMatches(row.TranscriptPath, transcriptPath) {
		return false
	}
	if row.AgentTranscriptPath != "" && transcriptPath != "" &&
		!cursorTokenLogPathMatches(row.AgentTranscriptPath, transcriptPath) {
		return false
	}
	if row.WorkspaceRoot != "" && cwd != "" &&
		!cursorTokenLogPathMatches(row.WorkspaceRoot, cwd) {
		return false
	}
	return true
}

func cursorTokenLogRowHasUsage(row cursorTokenLogRow) bool {
	return row.InputTokens != nil ||
		row.OutputTokens != nil ||
		row.CacheReadTokens != nil ||
		row.CacheWriteTokens != nil ||
		row.ContextTokens != nil
}

func cursorTokenLogRecordedAt(row cursorTokenLogRow) time.Time {
	s := strings.TrimSpace(row.RecordedAt)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func sortCursorTokenLogRows(rows []cursorTokenLogRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ti := cursorTokenLogRecordedAt(rows[i])
		tj := cursorTokenLogRecordedAt(rows[j])
		if ti.Equal(tj) {
			return i < j
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})
}

func applyCursorHookTokens(msg *ParsedMessage, row cursorTokenLogRow) {
	if msg == nil || msg.Role != RoleAssistant {
		return
	}
	if !cursorTokenLogRowHasUsage(row) {
		return
	}

	input := int64OrZero(row.InputTokens)
	output := int64OrZero(row.OutputTokens)
	cacheRead := int64OrZero(row.CacheReadTokens)
	cacheWrite := int64OrZero(row.CacheWriteTokens)

	tokenUsage := make(map[string]int, 4)
	if row.InputTokens != nil {
		tokenUsage["input_tokens"] = int(input)
	}
	if row.OutputTokens != nil {
		tokenUsage["output_tokens"] = int(output)
	}
	if row.CacheReadTokens != nil {
		tokenUsage["cache_read_input_tokens"] = int(cacheRead)
	}
	if row.CacheWriteTokens != nil {
		tokenUsage["cache_creation_input_tokens"] = int(cacheWrite)
	}
	if len(tokenUsage) == 0 {
		return
	}

	context := int(output)
	if row.ContextTokens != nil {
		context = int(*row.ContextTokens)
	} else if row.InputTokens != nil || row.CacheReadTokens != nil ||
		row.CacheWriteTokens != nil {
		context = int(input + cacheRead + cacheWrite)
	}

	msg.HasContextTokens = row.ContextTokens != nil ||
		row.InputTokens != nil ||
		row.CacheReadTokens != nil ||
		row.CacheWriteTokens != nil
	msg.HasOutputTokens = row.OutputTokens != nil
	msg.tokenPresenceKnown = true
	msg.ContextTokens = context
	msg.OutputTokens = int(output)

	raw, err := json.Marshal(tokenUsage, json.Deterministic(true))
	if err == nil {
		msg.TokenUsage = raw
	}
}

func int64OrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func enrichCursorSessionFromTokenLog(
	transcriptPath, cwd string,
	msgs []ParsedMessage,
) error {
	agentID := cursorRawIDFromTranscriptPath(transcriptPath)
	if !IsValidSessionID(agentID) {
		return nil
	}
	allRows, err := loadCursorTokenLogRows(cwd)
	if err != nil {
		return err
	}
	if len(allRows) == 0 {
		return nil
	}

	var matched []cursorTokenLogRow
	for _, row := range allRows {
		if cursorTokenLogRowMatchesSession(row, agentID, transcriptPath, cwd) {
			matched = append(matched, row)
		}
	}
	if len(matched) == 0 {
		return nil
	}

	var responseRows []cursorTokenLogRow
	var stopRows []cursorTokenLogRow
	for _, row := range matched {
		switch strings.TrimSpace(row.HookEventName) {
		case "afterAgentResponse":
			if cursorTokenLogRowHasUsage(row) {
				responseRows = append(responseRows, row)
			}
		case "stop":
			if cursorTokenLogRowHasUsage(row) {
				stopRows = append(stopRows, row)
			}
		}
	}
	sortCursorTokenLogRows(responseRows)
	sortCursorTokenLogRows(stopRows)

	assistantIdx := make([]int, 0, len(msgs))
	for i := range msgs {
		if msgs[i].Role == RoleAssistant {
			assistantIdx = append(assistantIdx, i)
		}
	}
	for i, row := range responseRows {
		if i >= len(assistantIdx) {
			break
		}
		applyCursorHookTokens(&msgs[assistantIdx[i]], row)
	}

	if len(responseRows) == 0 && len(stopRows) > 0 && len(assistantIdx) > 0 {
		applyCursorHookTokens(
			&msgs[assistantIdx[len(assistantIdx)-1]],
			stopRows[len(stopRows)-1],
		)
	}
	return nil
}

func cursorTokenLogConversationIDs(path string) ([]string, error) {
	rows, err := readCursorTokenLogRows(path)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, row := range rows {
		id := strings.TrimSpace(row.ConversationID)
		if id == "" || !IsValidSessionID(id) {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s cursorSourceSet) sourcesForTokenLogPath(
	_ string,
	logPath string,
) ([]SourceRef, error) {
	if !IsRegularFile(logPath) {
		return nil, nil
	}
	ids, err := cursorTokenLogConversationIDs(logPath)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var out []SourceRef
	seen := make(map[string]struct{})
	for _, root := range s.roots {
		for _, id := range ids {
			path := cursorFindSourceFile(root, id)
			if path == "" {
				continue
			}
			source, ok := s.sourceRef(root, path)
			if !ok {
				continue
			}
			key := source.Key
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, source)
		}
	}
	return out, nil
}

func cursorTokenLogChangedPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.HasSuffix(clean, "/"+cursorHookUsageFileName)
}
