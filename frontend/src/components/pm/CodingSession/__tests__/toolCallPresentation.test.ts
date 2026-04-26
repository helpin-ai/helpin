import { describe, expect, it } from 'vitest';

import { describeToolCall } from '../toolCallPresentation';
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
});
