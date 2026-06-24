package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/util"
)

// DedupIndex is satisfied by index.Store for migration deduplication.
type DedupIndex interface {
	FindMigration(providerID, originDigest string) (sessionID, storagePath string, ok bool)
}

// FindExistingMigration scans JSONL storage for agenthop_migration metadata matching origin digest.
func FindExistingMigration(storagePath, originDigest string) (string, bool) {
	if storagePath == "" || originDigest == "" {
		return "", false
	}
	if strings.HasSuffix(storagePath, ".jsonl") {
		if path, ok := scanJSONLForDigest(storagePath, originDigest); ok {
			return path, true
		}
	}
	dir := storagePath
	if strings.HasSuffix(storagePath, ".jsonl") {
		dir = filepath.Dir(storagePath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if p == storagePath {
			continue
		}
		if path, ok := scanJSONLForDigest(p, originDigest); ok {
			return path, true
		}
	}
	return "", false
}

func scanJSONLForDigest(path, originDigest string) (string, bool) {
	var found string
	_ = util.ReadJSONLLines(path, 0, func(line []byte) error {
		if migrationDigest(line) == originDigest {
			found = path
		}
		return nil
	})
	return found, found != ""
}

func migrationDigest(line []byte) string {
	var row map[string]any
	if json.Unmarshal(line, &row) != nil {
		return ""
	}
	if t, _ := row["type"].(string); t == model.MigrationType {
		if data, ok := row["data"].(map[string]any); ok {
			if d, _ := data["originDigest"].(string); d != "" {
				return d
			}
		}
	}
	if data, ok := row["data"].(map[string]any); ok {
		if t, _ := data["type"].(string); t == model.MigrationType {
			if d, _ := data["originDigest"].(string); d != "" {
				return d
			}
		}
	}
	return ""
}

// FindDuplicate searches the index and target provider storage for an existing migration of conv.
func FindDuplicate(idx DedupIndex, dst provider.Provider, conv *model.Conversation) (*provider.WriteResult, bool) {
	digest := model.OriginDigest(conv)
	if digest == "" {
		return nil, false
	}
	if idx != nil {
		if sid, path, ok := idx.FindMigration(dst.ID(), digest); ok {
			return &provider.WriteResult{
				SessionID:     sid,
				StoragePath:   path,
				AlreadyExists: true,
			}, true
		}
	}
	for _, ps := range dst.DefaultPaths() {
		root := ps.Path
		if root == "" {
			continue
		}
		if path, ok := walkForDigest(root, digest); ok {
			return writeResultFromPath(path), true
		}
	}
	return nil, false
}

func walkForDigest(root, digest string) (string, bool) {
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if p, ok := scanJSONLForDigest(path, digest); ok {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found, found != ""
}

func writeResultFromPath(path string) *provider.WriteResult {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	base := filepath.Base(path)
	if strings.HasPrefix(base, "rollout-") {
		parts := strings.Split(strings.TrimSuffix(base, ".jsonl"), "-")
		if len(parts) >= 2 {
			id = parts[len(parts)-1]
		}
	}
	_ = util.ReadJSONLLines(path, 3, func(line []byte) error {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			return nil
		}
		if sid, _ := row["session_id"].(string); sid != "" {
			id = sid
		}
		if sid, _ := row["sessionId"].(string); sid != "" {
			id = sid
		}
		return nil
	})
	return &provider.WriteResult{
		SessionID:     id,
		StoragePath:   path,
		AlreadyExists: true,
	}
}
