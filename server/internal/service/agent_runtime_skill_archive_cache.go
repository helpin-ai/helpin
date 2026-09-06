package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"sync"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
)

type runtimeSkillArchiveBuilder func(agentcontract.SkillDefinition) ([]byte, string, string, error)

type runtimeBuiltInSkillArchive struct {
	fingerprint [sha256.Size]byte
	data        []byte
	checksum    string
	filename    string
}

// Only product-owned built-ins enter this cache. Keep one version per catalog
// key; workspace skill lookups and their access/archive checks remain live.
type runtimeBuiltInSkillArchiveCache struct {
	mu    sync.Mutex
	byKey map[string]runtimeBuiltInSkillArchive
	build runtimeSkillArchiveBuilder
}

func newRuntimeBuiltInSkillArchiveCache(build runtimeSkillArchiveBuilder) *runtimeBuiltInSkillArchiveCache {
	return &runtimeBuiltInSkillArchiveCache{
		byKey: make(map[string]runtimeBuiltInSkillArchive),
		build: build,
	}
}

func (c *runtimeBuiltInSkillArchiveCache) get(definition agentcontract.SkillDefinition) ([]byte, string, string, error) {
	if c == nil {
		return agentcontract.BuildSkillArchive(definition)
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		// Cache identity must never prevent the ordinary archive build.
		return c.build(definition)
	}
	fingerprint := sha256.Sum256(encoded)
	// Hold the lock through a miss so concurrent runtime metadata/package
	// requests build an unchanged definition once. Archive generation is local.
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.byKey[definition.Key]; ok && entry.fingerprint == fingerprint {
		return bytes.Clone(entry.data), entry.checksum, entry.filename, nil
	}
	data, checksum, filename, err := c.build(definition)
	if err != nil {
		return nil, "", "", err
	}
	c.byKey[definition.Key] = runtimeBuiltInSkillArchive{
		fingerprint: fingerprint,
		data:        data,
		checksum:    checksum,
		filename:    filename,
	}
	return bytes.Clone(data), checksum, filename, nil
}
