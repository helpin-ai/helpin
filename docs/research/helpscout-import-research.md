# HelpScout Docs Import Research

> Research date: 2026-03-18

## 1. HelpScout Docs API

### Base URL & Authentication
- **Base URL**: `https://docsapi.helpscout.net/v1/`
- **Auth**: HTTP Basic Authentication — API key as username, dummy password (e.g., `X`)
- **HTTPS only**
- Each HelpScout user has their own Docs API key (found in Profile > Authentication > API Keys)
- Requires the "Docs: Create new, edit settings & Collections" permission

```bash
curl --user YOUR_API_KEY:X https://docsapi.helpscout.net/v1/collections
```

### Rate Limits

| Number of Sites | Rate Limit |
|-----------------|------------|
| 1               | 2,000 requests / 10 min |
| 2               | 3,000 requests / 10 min |
| 3+              | 4,000 requests / 10 min |

Response headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
HTTP 429 when exceeded. **Important**: Admin UI usage also counts toward the limit.

### Key Endpoints

#### Sites
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/sites` | List all Docs sites |
| GET | `/v1/sites/{id}` | Get a single site |

#### Collections (= top-level categories/spaces)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/collections` | List all collections (paginated) |
| GET | `/v1/collections/{id}` | Get a single collection (accepts id or number) |
| GET | `/v1/collections/{id}/categories` | List categories within a collection |
| GET | `/v1/collections/{id}/articles` | List articles within a collection |

**Collection Object Fields**:
```json
{
  "id": "string (ObjectId)",
  "siteId": "string",
  "number": 33,
  "slug": "my-collection",
  "visibility": "public",
  "order": 1,
  "name": "My Collection",
  "description": "Description of my collection",
  "publicUrl": "https://my-docs.helpscoutdocs.com/collection/1-test",
  "articleCount": 3,
  "publishedArticleCount": 1,
  "createdBy": 73423,
  "updatedBy": 73423,
  "createdAt": "2013-08-21T14:01:33Z",
  "updatedAt": "2013-08-21T14:01:33Z"
}
```

#### Categories (= sub-sections within a collection)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/collections/{id}/categories` | List categories for a collection |
| GET | `/v1/categories/{id}` | Get a single category |
| GET | `/v1/categories/{id}/articles` | List articles in a category |

**Category Object Fields**:
```json
{
  "id": "string",
  "number": 23,
  "slug": "my-category",
  "visibility": "public",
  "collectionId": "string",
  "order": 1,
  "defaultSort": "custom",
  "name": "My Category",
  "description": null,
  "articleCount": 21,
  "publishedArticleCount": 19,
  "publicUrl": "https://mysite.helpscoutdocs.com/category/133-my-category",
  "createdBy": 73423,
  "updatedBy": 73423,
  "createdAt": "2013-08-21T19:34:13Z",
  "updatedAt": "2013-08-21T19:34:13Z"
}
```

#### Articles
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/collections/{id}/articles` | List articles in collection (returns ArticleRef, no body) |
| GET | `/v1/categories/{id}/articles` | List articles in category (returns ArticleRef, no body) |
| GET | `/v1/articles/{id}` | Get full article (includes `text` body) |
| GET | `/v1/articles/{id}?draft=true` | Get draft version of article (if exists) |
| GET | `/v1/articles/{id}/revisions` | List article revisions |
| GET | `/v1/articles/{id}/revisions/{revId}` | Get a specific revision |
| GET | `/v1/articles/search?query=X&collectionId=Y` | Search articles |

### Pagination

All list endpoints return paginated results:
```json
{
  "articles": {
    "page": 1,
    "pages": 5,
    "count": 237,
    "items": [...]
  }
}
```

- Default page size not documented; the export tool uses `pageSize=100`
- Pass `?page=2` for subsequent pages
- Collections return max 50 records per page (per overview docs)

### Article Object (Full — from GET /v1/articles/{id})

```json
{
  "id": "5215163545667acd25394b5c",
  "number": 121,
  "slug": "my-article",
  "status": "published",
  "hasDraft": false,
  "name": "My Article",
  "text": "This is my <b>article text</b>.",
  "categories": ["5214c77d45667acd25394b52"],
  "related": ["521509f145667acd25394b5b"],
  "collectionId": "5214c77c45667acd25394b51",
  "publicUrl": "https://docs.helpscout.net/article/100-my-article",
  "popularity": 4.3,
  "viewCount": 236,
  "keywords": ["keyword1", "keyword2"],
  "createdBy": 73423,
  "updatedBy": null,
  "createdAt": "2013-08-21T19:34:13Z",
  "updatedAt": null
}
```

**Key fields**:
- `status`: `"published"` or `"notpublished"` (draft/unpublished)
- `hasDraft`: boolean — if `true`, there's a draft with unpublished changes
- `text`: **HTML string** — this is the article body content
- `keywords`: array of strings
- `categories`: array of category IDs (not names/slugs)
- `related`: array of article IDs for cross-references

### ArticleRef Object (from List endpoints — NO body text)

```json
{
  "id": "5215163545667acd25394b5c",
  "number": 121,
  "collectionId": "5214c77c45667acd25394b51",
  "status": "published",
  "hasDraft": false,
  "name": "My Article",
  "publicUrl": "https://docs.helpscout.net/article/100-my-article",
  "popularity": 4.3,
  "viewCount": 237,
  "createdBy": 73423,
  "updatedBy": null,
  "createdAt": "2013-08-21T19:34:13Z",
  "updatedAt": null,
  "lastPublishedAt": "2013-08-21T19:34:13Z"
}
```

**Critical**: List endpoints return `ArticleRef` (no `text` field). You MUST call `GET /v1/articles/{id}` individually for each article to get the body content. This means N+1 API calls.

---

## 2. Article Content Format

### What HelpScout Returns

The `text` field in the Article object contains **HTML**. Key characteristics:

- **Format**: Raw HTML string (not Markdown, not JSON)
- **Example**: `"This is my <b>article text</b>."`
- **Editor behavior**: HelpScout's Docs editor strips HTML classes, inline CSS, and unsupported customizations from content entered via the WYSIWYG editor
- **HTML blocks**: Authors CAN use HTML blocks with inline CSS for custom styling — these are preserved in the `text` field
- **No Markdown version**: The API does not return a Markdown alternative. Only HTML via `text`
- **Images**: Docs auto-resizes images to max 1000px width / 800px height. Images are hosted on HelpScout's CDN (various subdomains of `helpscoutdocs.com` and `helpscout.com`)
- **Lightbox links**: Images may have `class="lightbox"` for click-to-expand behavior

### Content Patterns to Expect

1. **Clean HTML**: Standard `<p>`, `<h1>`-`<h6>`, `<ul>`, `<ol>`, `<li>`, `<a>`, `<img>`, `<strong>`, `<em>`, `<code>`, `<pre>`, `<blockquote>`, `<table>`, `<hr>`
2. **HTML blocks with inline CSS**: `<div style="background: #f0f0f0; padding: 15px;">` etc.
3. **Embedded videos**: `<iframe>` tags for YouTube/Vimeo embeds
4. **Image tags**: `<img src="https://secure.helpscout.net/..." />` or CDN URLs
5. **Internal links**: `<a href="https://docs.yoursite.helpscoutdocs.com/article/123-slug">` — links between articles use the public URL format
6. **Code blocks**: `<pre><code>` blocks, possibly with syntax highlighting classes

---

## 3. How Other Tools Import from HelpScout

### Existing Tools & Approaches

**arikfr/helpscout-docs-export** (Python, open-source):
- The canonical reference implementation for HelpScout export
- Algorithm:
  1. Fetch all collections → `GET /v1/collections`
  2. Fetch all categories for each collection → `GET /v1/collections/{id}/categories`
  3. For each collection, list articles → `GET /v1/collections/{id}/articles?pageSize=100&status=published`
  4. For each ArticleRef, fetch full article → `GET /v1/articles/{id}`
  5. Convert `article.text` (HTML) to Markdown using `html2text` Python library
  6. Prepend YAML frontmatter with metadata (collection slug, categories, keywords, name, publicUrl, slug)
  7. Write as `.md` files organized by collection slug
- Also dumps `categories.json` and `collections.json` for reference

**Help Desk Migration** (commercial service):
- Automated migration between 76+ platforms
- Handles HelpScout to Intercom, Zendesk, Freshdesk etc.
- Maps custom fields and values automatically
- Primarily focused on ticket/conversation migration, not just docs

**Document360** (commercial):
- Offers managed migration service (not self-service API)
- Claims 100+ platform support including HelpScout
- Migration team handles the process manually
- Preserves categories, articles, and SEO rankings

**HelpDocs**:
- Self-service import supporting multiple platforms
- No specific HelpScout connector documented

### Common Pattern Across Tools

All tools follow the same algorithm:
1. Collections → Categories → Article list → Individual article fetch
2. N+1 problem is unavoidable (list gives ArticleRef, need individual GET for body)
3. HTML conversion happens client-side after fetching
4. Metadata (categories, keywords, status) preserved as structured data alongside content

---

## 4. Import Gotchas & Nuances

### Images
- **CDN URLs**: Images hosted on `secure.helpscout.net`, `*.helpscoutdocs.com`, or custom CDN domains
- **Risk**: If HelpScout account is closed, image URLs break. Must download and re-upload images to own storage
- **Auto-resize**: HelpScout caps images at 1000x800px — originals may not be recoverable
- **Recommendation**: Parse all `<img src="...">` tags, download images, upload to S3/MinIO, rewrite URLs in HTML before conversion

### Internal Links (Cross-References)
- Format: `https://docs.yoursite.helpscoutdocs.com/article/{number}-{slug}`
- Must build a mapping table: HelpScout article ID/number/slug → new Helpin article ID
- Post-import pass to rewrite all internal `<a href="...">` links to new URLs
- The `related` field on articles provides explicit cross-references (array of article IDs)

### Draft vs Published
- `status`: `"published"` or `"notpublished"`
- `hasDraft`: boolean indicating unpublished changes exist
- Use `?draft=true` query param on GET article to retrieve draft version
- Decision needed: Import only published? Import drafts separately? Import draft version if it exists?
- **Recommendation**: Import published articles as published. For articles with `hasDraft=true`, optionally fetch and store draft as a separate version

### Article Metadata to Preserve
- `name` → article title
- `slug` → URL slug (SEO preservation)
- `status` → published/draft status
- `keywords` → search keywords (array of strings)
- `categories` → category associations (array of category IDs — need to resolve to names/slugs)
- `related` → cross-reference article IDs
- `publicUrl` → original URL (for redirect mapping)
- `createdAt`, `updatedAt` → timestamps
- `createdBy`, `updatedBy` → author IDs (would need User API to resolve names)
- `viewCount`, `popularity` → engagement metrics

### Custom HTML/CSS
- HTML blocks with inline CSS will survive the API response
- Must decide: strip inline styles (clean import) or preserve them (visual fidelity)
- **Recommendation**: Strip inline styles during conversion. TipTap/ProseMirror doesn't support arbitrary inline CSS

### Embedded Videos
- `<iframe>` tags for YouTube, Vimeo, Loom, etc.
- ProseMirror/TipTap can handle these via a custom `iframe` node extension
- Must whitelist allowed iframe sources for security

### Character Encoding
- API returns UTF-8 JSON
- HTML entities in content (`&amp;`, `&lt;`, `&#8217;` etc.) — handled by HTML parser
- Watch for double-encoding if content passes through multiple serialization steps

### URL Redirects
- Must preserve `publicUrl` mapping for SEO
- If customers had public HelpScout docs, they'll need redirects from old URLs to new ones
- Build redirect map: `helpscoutdocs.com/article/{number}-{slug}` → `helpin.ai/docs/{new-slug}`

### Rate Limiting Strategy
- With 2000 req/10min, importing 500 articles means:
  - 1 (collections) + N (categories per collection) + N (article lists) + 500 (individual articles) = ~510 requests
  - Well within limits for most knowledge bases
  - For large KBs (1000+ articles), add a simple delay between batches
- Monitor `X-RateLimit-Remaining` header and pause when approaching zero

---

## 5. HTML to TipTap/ProseMirror JSON Conversion

### Option A: `@tiptap/html` generateJSON (RECOMMENDED)

**Package**: `@tiptap/html` (server-compatible)
**Import**: `import { generateJSON } from '@tiptap/html'` (NOT from `@tiptap/core` which is browser-only)

```javascript
import { generateJSON } from '@tiptap/html'
import Document from '@tiptap/extension-document'
import Paragraph from '@tiptap/extension-paragraph'
import Text from '@tiptap/extension-text'
import Bold from '@tiptap/extension-bold'
import Italic from '@tiptap/extension-italic'
import Heading from '@tiptap/extension-heading'
import BulletList from '@tiptap/extension-bullet-list'
import OrderedList from '@tiptap/extension-ordered-list'
import ListItem from '@tiptap/extension-list-item'
import CodeBlock from '@tiptap/extension-code-block'
import Blockquote from '@tiptap/extension-blockquote'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import Table from '@tiptap/extension-table'
import TableRow from '@tiptap/extension-table-row'
import TableCell from '@tiptap/extension-table-cell'
import TableHeader from '@tiptap/extension-table-header'
import HardBreak from '@tiptap/extension-hard-break'
import HorizontalRule from '@tiptap/extension-horizontal-rule'
// ... other extensions matching your TipTap editor config

const json = generateJSON(htmlString, [
  Document, Paragraph, Text, Bold, Italic, Heading,
  BulletList, OrderedList, ListItem, CodeBlock, Blockquote,
  Image, Link, Table, TableRow, TableCell, TableHeader,
  HardBreak, HorizontalRule,
])
```

**Pros**:
- Uses the exact same parsing rules as TipTap editor — guaranteed round-trip fidelity
- Extensions list must match what your editor supports (unknown HTML is stripped)
- Server-side via `@tiptap/html` uses a virtual DOM (no browser needed)
- Widely used and maintained by TipTap team

**Cons**:
- Requires Node.js runtime (can't run in Go directly)
- Must keep extension list in sync with frontend editor config
- Known issue (#6939): some versions have server-side compatibility problems — use `@tiptap/html` not `@tiptap/html/server`

**Implementation approach for Helpin**:
- Write a Node.js conversion script/service that takes HTML input and outputs TipTap JSON
- Could be: (a) a standalone CLI script called from Go via `exec.Command`, (b) a small HTTP microservice, or (c) a Temporal activity in Node
- The conversion script should share the same extension list as the frontend editor

### Option B: HTML → Markdown → TipTap (Two-step)

1. Convert HTML to Markdown using Go library (e.g., `jaytaylor/html2text` or `JohannesKaufmann/html-to-markdown`)
2. Parse Markdown to TipTap JSON

**Pros**:
- Stays entirely in Go
- Markdown is a clean intermediate format

**Cons**:
- **Lossy**: HTML → Markdown loses information (tables, inline styles, custom HTML, some formatting)
- No reliable Go library for Markdown → TipTap/ProseMirror JSON
- Two conversion steps = two sources of bugs
- **Not recommended** for production use

### Option C: Go-native ProseMirror Libraries

Available Go packages:
- `cozy/prosemirror-go` — Port of prosemirror-model/transform for collaborative editing servers. **Does NOT include DOM parsing** (HTML → JSON). Only useful for document manipulation, not conversion.
- `karitham/prosemirror`, `nicksrandall/prosemirror-go`, `wenj91/prosemirror-go` — Various Go implementations, mostly focused on JSON → HTML rendering, not the reverse direction.

**Verdict**: No Go library supports HTML → ProseMirror JSON conversion. The DOM parsing logic is fundamentally browser/JS-dependent.

### Option D: Frontend-side Conversion

Use TipTap's built-in `editor.commands.setContent(html)` on the frontend, then extract JSON.

**Pros**:
- Zero backend work needed
- Guaranteed compatibility with editor
- Can preview during import

**Cons**:
- Conversion happens per-article in the browser — slow for bulk imports
- User must keep browser tab open during import
- No batch processing capability

### Recommendation

**Use Option A (`@tiptap/html` generateJSON)** with this architecture:

1. **Go backend** handles: HelpScout API calls, image downloading/re-uploading, metadata mapping
2. **Node.js script** handles: HTML → TipTap JSON conversion (called from Go or as Temporal activity)
3. **Pipeline**:
   ```
   HelpScout API → Go fetcher → Image processor → Node.js HTML→JSON converter → Go article creator
   ```

Alternatively, the entire import could be a **Temporal workflow**:
- Activity 1 (Go): Fetch collections, categories, article list from HelpScout
- Activity 2 (Go): For each article, fetch full content + download images + upload to S3
- Activity 3 (Node or Go shelling to Node): Convert HTML to TipTap JSON
- Activity 4 (Go): Create articles in Helpin DB with converted content

This approach isolates the JS dependency to a single conversion step while keeping the rest in Go.

---

## Sources

- [HelpScout Docs API Overview](https://developer.helpscout.com/docs-api/)
- [HelpScout Docs API - List Articles](https://developer.helpscout.com/docs-api/articles/list/)
- [HelpScout Docs API - Get Article](https://developer.helpscout.com/docs-api/articles/get/)
- [HelpScout Docs API - Article Object](https://developer.helpscout.com/docs-api/objects/article/)
- [HelpScout Docs API - ArticleRef Object](https://developer.helpscout.com/docs-api/objects/article-ref/)
- [HelpScout Docs API - List Collections](https://developer.helpscout.com/docs-api/collections/list/)
- [HelpScout Docs API - Get Collection](https://developer.helpscout.com/docs-api/collections/get/)
- [HelpScout Docs API - Category Object](https://developer.helpscout.com/docs-api/objects/category/)
- [HelpScout Docs API - List Categories](https://developer.helpscout.com/docs-api/categories/list/)
- [HelpScout Docs API - Save Article Draft](https://developer.helpscout.com/docs-api/articles/save-draft/)
- [HelpScout Docs Export Tool (Python)](https://github.com/arikfr/helpscout-docs-export)
- [TipTap HTML Utility](https://tiptap.dev/docs/editor/api/utilities/html)
- [TipTap Export Guide](https://tiptap.dev/docs/guides/output-json-html)
- [@tiptap/html npm package](https://www.npmjs.com/package/@tiptap/html)
- [cozy/prosemirror-go](https://github.com/cozy/prosemirror-go)
- [Document360 Migration Docs](https://docs.document360.com/docs/migrating-documentation-from-another-knowledge-base-platform)
- [HelpScout Docs Editor HTML Usage](https://docs.helpscout.com/article/1501-use-html-in-the-docs-editor)
- [HelpScout Doc Article Styles (GitHub)](https://github.com/helpscout/doc-article-styles)
