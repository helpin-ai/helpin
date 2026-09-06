package service

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
)

func TestRuntimeBuiltInSkillArchiveCachePreservesCatalogPackages(t *testing.T) {
	cache := newRuntimeBuiltInSkillArchiveCache(agentcontract.BuildSkillArchive)
	for _, definition := range agentcontract.ListBuiltInSkills() {
		t.Run(definition.Key, func(t *testing.T) {
			want, checksum, filename, err := agentcontract.BuildSkillArchive(definition)
			if err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				got, gotChecksum, gotFilename, err := cache.get(definition)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) || gotChecksum != checksum || gotFilename != filename {
					t.Errorf("attempt %d changed archive bytes, checksum or filename", attempt)
				}
			}
		})
	}
}

func TestRuntimeBuiltInSkillArchiveCacheSharesConcurrentBuild(t *testing.T) {
	definition := agentcontract.SkillDefinition{Key: "example", Instructions: "Keep all tools available."}
	want, _, _, err := agentcontract.BuildSkillArchive(definition)
	if err != nil {
		t.Fatal(err)
	}
	var builds atomic.Int32
	cache := newRuntimeBuiltInSkillArchiveCache(func(def agentcontract.SkillDefinition) ([]byte, string, string, error) {
		builds.Add(1)
		return agentcontract.BuildSkillArchive(def)
	})
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, _, _, err := cache.get(definition)
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("concurrent lookup returned different package: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := builds.Load(); got != 1 {
		t.Errorf("built an unchanged skill %d times; want 1", got)
	}
}

func TestRuntimeBuiltInSkillArchiveCacheRefreshesChangedDefinition(t *testing.T) {
	cases := []struct {
		name   string
		change func(*agentcontract.SkillDefinition)
	}{
		{"instructions", func(d *agentcontract.SkillDefinition) { d.Instructions = "Updated instructions." }},
		{"required tools", func(d *agentcontract.SkillDefinition) { d.RequiredTools = []string{"request_approval"} }},
		{"runtimes", func(d *agentcontract.SkillDefinition) { d.SupportedRuntimes = []string{"native_sdk"} }},
		{"interface", func(d *agentcontract.SkillDefinition) { d.Interface.DefaultPrompt = "Updated prompt." }},
		{"policy", func(d *agentcontract.SkillDefinition) {
			allowed := false
			d.Policy.AllowImplicitInvocation = &allowed
		}},
		{"interaction contract", func(d *agentcontract.SkillDefinition) {
			d.Policy.InteractionContracts = []agentcontract.SkillInteractionContract{{
				Kind: "approval_request", Schema: "approval_v1",
				Transports: map[string]agentcontract.SkillInteractionTransport{
					"native_sdk": {Type: "tool_call", ToolName: "request_approval"},
				},
			}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := newRuntimeBuiltInSkillArchiveCache(agentcontract.BuildSkillArchive)
			definition := agentcontract.SkillDefinition{Key: "example", Instructions: "Original instructions."}
			_, oldChecksum, _, err := cache.get(definition)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&definition)
			want, checksum, filename, err := agentcontract.BuildSkillArchive(definition)
			if err != nil {
				t.Fatal(err)
			}
			got, gotChecksum, gotFilename, err := cache.get(definition)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) || gotChecksum != checksum || gotFilename != filename {
				t.Error("returned stale package after definition changed")
			}
			if gotChecksum == oldChecksum {
				t.Error("changed definition retained old checksum")
			}
		})
	}
}

func TestRuntimeBuiltInSkillArchiveCacheIsolatesReturnedBytes(t *testing.T) {
	cache := newRuntimeBuiltInSkillArchiveCache(agentcontract.BuildSkillArchive)
	definition := agentcontract.SkillDefinition{Key: "example", Instructions: "Original instructions."}
	want, _, _, err := agentcontract.BuildSkillArchive(definition)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		got, _, _, err := cache.get(definition)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("attempt %d returned bytes modified by a previous caller", attempt)
		}
		got[0] ^= 0xff
	}
}

func TestRuntimeBuiltInSkillArchiveCacheRetriesBuildErrors(t *testing.T) {
	wantErr := errors.New("temporary build error")
	builds := 0
	cache := newRuntimeBuiltInSkillArchiveCache(func(def agentcontract.SkillDefinition) ([]byte, string, string, error) {
		builds++
		if builds == 1 {
			return nil, "", "", wantErr
		}
		return agentcontract.BuildSkillArchive(def)
	})
	definition := agentcontract.SkillDefinition{Key: "example", Instructions: "Original instructions."}
	if _, _, _, err := cache.get(definition); !errors.Is(err, wantErr) {
		t.Fatalf("expected original build error, got %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, _, _, err := cache.get(definition); err != nil {
			t.Fatalf("retry %d failed: %v", attempt, err)
		}
	}
	if builds != 2 {
		t.Errorf("built %d times; want one failure and one successful build", builds)
	}
}

func BenchmarkRuntimeBuiltInSkillArchive(b *testing.B) {
	definition, ok := agentcontract.GetBuiltInSkill("marketing_context_setup")
	if !ok {
		b.Fatal("missing built-in skill")
	}
	b.Run("uncached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, _, _, err := agentcontract.BuildSkillArchive(definition); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("cached", func(b *testing.B) {
		cache := newRuntimeBuiltInSkillArchiveCache(agentcontract.BuildSkillArchive)
		if _, _, _, err := cache.get(definition); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, _, _, err := cache.get(definition); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkRuntimeBuiltInSkillLookupCycle(b *testing.B) {
	var keys []string
	for _, key := range askAgentAvailableSkills() {
		// Native runtime embeds these three packages itself. The remaining
		// skills use Helpin's by-key, by-ID and package endpoints each turn.
		switch key {
		case "internal_docs_maintenance", "public_help_docs_maintenance", "api_docs_maintenance":
			continue
		}
		keys = append(keys, key)
	}
	for _, mode := range []string{"uncached", "cached_cold", "cached_warm"} {
		b.Run(mode, func(b *testing.B) {
			host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			if mode == "uncached" {
				host.builtInArchives = nil
			}
			if mode == "cached_warm" {
				benchmarkRuntimeSkillLookupCycle(b, host, keys)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "cached_cold" {
					host.builtInArchives = newRuntimeBuiltInSkillArchiveCache(agentcontract.BuildSkillArchive)
				}
				benchmarkRuntimeSkillLookupCycle(b, host, keys)
			}
		})
	}
}

func benchmarkRuntimeSkillLookupCycle(b *testing.B, host *AgentRuntimeHostService, keys []string) {
	b.Helper()
	for _, key := range keys {
		skill, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
			AppID: "helpin", Key: key,
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{
			AppID: "helpin", SkillID: skill.ID,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := host.GetSkillPackageObject(context.Background(), skill.PackageObjectKey); err != nil {
			b.Fatal(err)
		}
	}
}
