import { describe, expect, it } from 'vitest'
import { htmlToMarkdown } from '@/lib/htmlToMarkdown'

function convert(html: string) {
  const root = document.createElement('div')
  root.innerHTML = html
  return htmlToMarkdown(root, 'https://help.acme.test/articles/export')
}

describe('htmlToMarkdown', () => {
  it('converts an article to Markdown', () => {
    const markdown = convert(`
      <h2>Export a report</h2>
      <p>Open <strong>Reports</strong>, then choose <em>Export</em>. See <a href="/articles/csv">CSV format</a>.</p>
      <ol><li>Pick a range</li><li>Click <code>Export</code><ul><li>CSV</li><li>PDF</li></ul></li></ol>
      <pre><code class="language-bash">curl -X GET /reports
</code></pre>
      <blockquote><p>Exports run in the background.</p></blockquote>
      <table><tr><th>Format</th><th>Size</th></tr><tr><td>CSV</td><td>Small | fast</td></tr></table>
      <img src="/assets/export.png" alt="Export dialog">
    `)
    expect(markdown).toBe([
      '## Export a report',
      '',
      'Open **Reports**, then choose *Export*. See [CSV format](https://help.acme.test/articles/csv).',
      '',
      '1. Pick a range',
      '2. Click `Export`',
      '  - CSV',
      '  - PDF',
      '',
      '```bash',
      'curl -X GET /reports',
      '```',
      '',
      '> Exports run in the background.',
      '',
      '| Format | Size |',
      '| --- | --- |',
      '| CSV | Small \\| fast |',
      '',
      '![Export dialog](https://help.acme.test/assets/export.png)',
      '',
    ].join('\n'))
  })

  it('skips buttons and anchor-only links', () => {
    expect(convert('<h3><a href="#setup">#</a>Setup</h3><p>Text<button>Copy</button></p>')).toBe('### Setup\n\nText\n')
  })
})
