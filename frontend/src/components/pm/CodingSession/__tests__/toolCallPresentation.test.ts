import { describe, expect, it } from 'vitest';

import { describeToolCall, readFilesContentForToolCall, repositorySelectorForToolCall } from '../toolCallPresentation';
import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';

function buildToolCall(overrides: Partial<CodingSessionLiveToolCall> & Pick<CodingSessionLiveToolCall, 'tool_name' | 'args_text'>): CodingSessionLiveToolCall {
  return {
    tool_call_id: overrides.tool_call_id ?? 'tool-1',
    tool_name: overrides.tool_name,
    args_text: overrides.args_text,
    status: overrides.status ?? 'completed',
    parent_message_id: overrides.parent_message_id,
    duration_ms: overrides.duration_ms,
    started_at: overrides.started_at,
    completed_at: overrides.completed_at,
    result: overrides.result,
  };
}

describe('describeToolCall', () => {
  it('formats read_file_range with path and line range', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'read_file_range',
      args_text: JSON.stringify({
        path: 'frontend/src/components/ContentStudioShareModal/ContentStudioShareModal.tsx',
        start_line: 440,
        end_line: 468,
      }),
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Read frontend/src/components/ContentStudioShareModal/ContentStudioShareModal.tsx:440-468',
      secondaryLabel: 'Read File Range',
      chips: ['29 lines'],
    });
  });

  it('formats ripgrep with pattern and path', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'ripgrep',
      args_text: JSON.stringify({
        pattern: 'openShareModal',
        path: 'frontend/src',
      }),
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Search "openShareModal" in frontend/src',
      secondaryLabel: 'Ripgrep',
      chips: [],
    });
  });

  it('formats canonical batched reads and repository search', () => {
    const read = describeToolCall(buildToolCall({
      tool_name: 'read_files',
      args_text: JSON.stringify({ files: [{ path: 'main.go' }, { path: 'metrics.go' }] }),
    }));
    expect(read).toEqual({
      primaryLabel: 'Read main.go +1 more',
      secondaryLabel: 'Read Files',
      chips: ['2 files'],
    });

    const search = describeToolCall(buildToolCall({
      tool_name: 'repository_search',
      args_text: JSON.stringify({ query: 'prometheus', path: 'rust-capture', repository: 'events-pipeline' }),
    }));
    expect(search).toEqual({
      primaryLabel: 'Search "prometheus" in rust-capture · events-pipeline',
      secondaryLabel: 'Repository Search',
      chips: [],
      repositoryLabel: 'events-pipeline',
    });
  });

  it('uses completed read_files ranges and exact continuation metadata', () => {
    const toolCall = buildToolCall({
      tool_name: 'read_files',
      args_text: JSON.stringify({ files: [{ path: 'main.go', repository: 'service-api' }] }),
      result: {
        content: JSON.stringify({
          count: 1,
          files: [{
            path: 'main.go',
            start_line: 20,
            end_line: 26,
            content: 'File: main.go\n20: package main',
            has_more: true,
            next_start_line: 27,
            continuation_reason: 'output_limit',
          }],
        }),
      },
    });

    expect(describeToolCall(toolCall)).toEqual({
      primaryLabel: 'Read main.go:20-26 · service-api',
      secondaryLabel: 'Read Files',
      chips: ['1 file', 'continues at 27'],
      repositoryLabel: 'service-api',
    });
    expect(readFilesContentForToolCall(toolCall)).toBe('File: main.go\n20: package main');
  });

  it('summarizes multiple completed read_files results without exposing the JSON envelope', () => {
    const toolCall = buildToolCall({
      tool_name: 'read_files',
      args_text: JSON.stringify({ files: [{ path: 'a.ts' }, { path: 'b.ts' }] }),
      result: {
        content: JSON.stringify({
          count: 2,
          files: [
            { path: 'a.ts', start_line: 1, end_line: 4, content: 'File: a.ts\n1: export {}', has_more: false },
            { path: 'b.ts', start_line: 8, end_line: 10, content: 'File: b.ts\n8: export {}', has_more: true, next_start_line: 11, continuation_reason: 'line_limit' },
          ],
        }),
      },
    });

    expect(describeToolCall(toolCall)).toMatchObject({
      primaryLabel: 'Read a.ts:1-4 +1 more',
      chips: ['2 files', 'continues at 11'],
    });
    expect(readFilesContentForToolCall(toolCall)).toBe('File: a.ts\n1: export {}\n\nFile: b.ts\n8: export {}');
  });

  it('falls back to requested paths when a read_files result is absent or malformed', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'read_files',
      args_text: JSON.stringify({ files: [{ path: 'main.go' }] }),
      result: { content: '{not valid json' },
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Read main.go',
      secondaryLabel: 'Read Files',
      chips: ['1 file'],
    });
  });

  it('derives a repository label from nested read_files selectors only when unambiguous', () => {
    const oneRepository = buildToolCall({
      tool_name: 'read_files',
      args_text: JSON.stringify({ files: [{ path: 'a.go', repository: 'api' }, { path: 'b.go', repository: 'api' }] }),
    });
    const multipleRepositories = buildToolCall({
      tool_name: 'read_files',
      args_text: JSON.stringify({ files: [{ path: 'a.go', repository: 'api' }, { path: 'b.go', repository: 'worker' }] }),
    });

    expect(repositorySelectorForToolCall(oneRepository)).toBe('api');
    expect(repositorySelectorForToolCall(multipleRepositories)).toBeNull();
  });

  it('formats run_command with command and cwd', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'run_command',
      args_text: JSON.stringify({
        command: 'rg "openShareModal"',
        cwd: 'frontend/src',
      }),
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Run rg "openShareModal"',
      secondaryLabel: 'Run Command',
      chips: ['frontend/src'],
    });
  });

  describe('web search tools', () => {
    it('formats canonical web_search modes and filters', () => {
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'web_search',
        args_text: JSON.stringify({
          query: 'Kafka fallback patterns',
          mode: 'deep',
          category: 'research paper',
          include_domains: ['kafka.apache.org'],
          max_results: 4,
        }),
      }));

      expect(presentation).toEqual({
        primaryLabel: 'Search "Kafka fallback patterns"',
        secondaryLabel: 'Web Search',
        chips: ['deep', 'research paper', 'kafka.apache.org', '4 results'],
      });
    });

    it('formats web_search_exa with only query', () => {
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'web_search_exa',
        args_text: JSON.stringify({
          query: 'latest Vite release notes',
        }),
      }));

      expect(presentation).toEqual({
        primaryLabel: 'Search "latest Vite release notes"',
        secondaryLabel: 'Web Search Exa',
        chips: [],
      });
    });

    it('formats web_search_exa with category, include_domains, and num_results chips', () => {
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'web_search_exa',
        args_text: JSON.stringify({
          query: 'React 19 compiler docs',
          category: 'news',
          include_domains: ['github.com', 'react.dev', 'vite.dev'],
          num_results: 5,
        }),
      }));

      expect(presentation).toEqual({
        primaryLabel: 'Search "React 19 compiler docs"',
        secondaryLabel: 'Web Search Exa',
        chips: ['news', 'github.com +2', '5 results'],
      });
    });

    it('truncates long web_search_exa queries', () => {
      const query = 'a'.repeat(90);
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'web_search_exa',
        args_text: JSON.stringify({ query }),
      }));

      expect(presentation).toEqual({
        primaryLabel: `Search "${'a'.repeat(79)}…"`,
        secondaryLabel: 'Web Search Exa',
        chips: [],
      });
    });

    it('falls back for web_search_exa with missing or unparseable args', () => {
      const missingQuery = describeToolCall(buildToolCall({
        tool_name: 'web_search_exa',
        args_text: JSON.stringify({ category: 'news' }),
      }));
      const unparseableArgs = describeToolCall(buildToolCall({
        tool_name: 'web_search_exa',
        args_text: '{not json',
      }));

      expect(missingQuery).toEqual({
        primaryLabel: 'Web Search Exa',
        secondaryLabel: 'Web Search Exa',
        chips: [],
      });
      expect(unparseableArgs).toEqual({
        primaryLabel: 'Web Search Exa',
        secondaryLabel: 'Web Search Exa',
        chips: [],
      });
    });

    it('formats web_search_brave with freshness and domain_allowlist chips', () => {
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'web_search_brave',
        args_text: JSON.stringify({
          query: 'PostgreSQL 18 release',
          freshness: 'pw',
          domain_allowlist: ['postgresql.org', 'github.com'],
        }),
      }));

      expect(presentation).toEqual({
        primaryLabel: 'Search "PostgreSQL 18 release"',
        secondaryLabel: 'Web Search Brave',
        chips: ['pw', 'postgresql.org +1'],
      });
    });

    it('formats fetch_url with target URL', () => {
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'fetch_url',
        args_text: JSON.stringify({
          url: 'https://docs.writesonic.com/changelog',
        }),
      }));

      expect(presentation).toEqual({
        primaryLabel: 'Fetch https://docs.writesonic.com/changelog',
        secondaryLabel: 'Fetch Url',
        chips: [],
      });
    });

    it('formats crawl_url with crawl limits', () => {
      const presentation = describeToolCall(buildToolCall({
        tool_name: 'crawl_url',
        args_text: JSON.stringify({
          url: 'https://docs.writesonic.com',
          max_pages: 8,
          max_depth: 2,
        }),
      }));

      expect(presentation).toEqual({
        primaryLabel: 'Crawl https://docs.writesonic.com',
        secondaryLabel: 'Crawl Url',
        chips: ['8 pages', 'depth 2'],
      });
    });
  });

  it('formats read_symbol with the symbol name and file', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'read_symbol',
      args_text: JSON.stringify({ path: 'internal/tools/workspace_read_tools.go', symbol: 'readTextFileWindow' }),
    }));
    expect(presentation.primaryLabel).toBe('Read readTextFileWindow in internal/tools/workspace_read_tools.go');
  });

  it('formats find_symbol with an optional kind chip', () => {
    const withKind = describeToolCall(buildToolCall({
      tool_name: 'find_symbol',
      args_text: JSON.stringify({ name: 'Config', kind: 'type' }),
    }));
    expect(withKind.primaryLabel).toBe('Find Config');
    expect(withKind.chips).toContain('type');

    const withoutKind = describeToolCall(buildToolCall({
      tool_name: 'find_symbol',
      args_text: JSON.stringify({ name: 'Config' }),
    }));
    expect(withoutKind.chips).toHaveLength(0);
  });

  it('formats call graph lookups', () => {
    const callers = describeToolCall(buildToolCall({
      tool_name: 'find_callers',
      args_text: JSON.stringify({ symbol: 'Login' }),
    }));
    expect(callers.primaryLabel).toBe('Find callers of Login');

    const callees = describeToolCall(buildToolCall({
      tool_name: 'find_callees',
      args_text: JSON.stringify({ symbol: 'HandleLogin' }),
    }));
    expect(callees.primaryLabel).toBe('Find calls made by HandleLogin');
  });

  it('formats canonical symbol tracing and skill discovery', () => {
    const trace = describeToolCall(buildToolCall({
      tool_name: 'trace_symbol',
      args_text: JSON.stringify({ symbol: 'kafka_send', direction: 'callers' }),
    }));
    expect(trace).toEqual({
      primaryLabel: 'Trace callers of kafka_send',
      secondaryLabel: 'Trace Symbol',
      chips: ['callers'],
    });

    const skills = describeToolCall(buildToolCall({
      tool_name: 'find_skills',
      args_text: JSON.stringify({ query: 'security review' }),
    }));
    expect(skills.primaryLabel).toBe('Find skills for "security review"');
  });

  it('formats list_symbols as an outline', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'list_symbols',
      args_text: JSON.stringify({ path: 'internal/tools/workspace_tools.go' }),
    }));
    expect(presentation.primaryLabel).toBe('Outline internal/tools/workspace_tools.go');
  });

  it('keeps repository identity in multi-repository symbol and root-list rows', () => {
    const symbol = describeToolCall(buildToolCall({
      tool_name: 'read_symbol',
      args_text: JSON.stringify({
        path: 'rust-capture/src/sinks/kafka_event_sink.rs',
        symbol: 'kafka_send',
        repo_alias: 'events-pipeline',
      }),
    }));
    expect(symbol).toEqual({
      primaryLabel: 'Read kafka_send in rust-capture/src/sinks/kafka_event_sink.rs · events-pipeline',
      secondaryLabel: 'Read Symbol',
      chips: [],
      repositoryLabel: 'events-pipeline',
    });

    const root = describeToolCall(buildToolCall({
      tool_name: 'list_directory',
      args_text: JSON.stringify({ path: '', repo_alias: 'website' }),
    }));
    expect(root).toEqual({
      primaryLabel: 'List repository root · website',
      secondaryLabel: 'List Directory',
      chips: [],
      repositoryLabel: 'website',
    });
  });

});
