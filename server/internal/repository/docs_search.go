package repository

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsSearchRepository handles full-text search for documents.
type DocsSearchRepository struct {
	db *gorm.DB
}

const (
	defaultPublicSearchLimit    = 20
	publicSearchEntryCollection = "collection"
)

// NewDocsSearchRepository creates a new DocsSearchRepository.
func NewDocsSearchRepository(db *gorm.DB) *DocsSearchRepository {
	return &DocsSearchRepository{db: db}
}

// DocsSearchResult is a search result with rank score.
type DocsSearchResult struct {
	model.DocsDocument
	Rank         float64 `json:"rank"`
	MatchBlockID string  `json:"match_block_id"`
	MatchText    string  `json:"match_text"`
}

// Search performs a Postgres full-text search across document titles and content.
func (r *DocsSearchRepository) Search(ctx context.Context, workspaceID, query string, spaceIDs []string, status *string, limit int) ([]DocsSearchResult, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Convert user query to tsquery format.
	tsQuery := toTSQuery(query)

	sql := `
		SELECT d.*, ts_rank(
			setweight(to_tsvector('english', COALESCE(d.title, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE(c.content_text, '')), 'B'),
			to_tsquery('english', ?)
		) AS rank
		FROM docs_documents d
		LEFT JOIN docs_contents c ON c.document_id = d.id
		WHERE d.workspace_id = ?
		  AND d.deleted_at IS NULL
		  AND (
			to_tsvector('english', COALESCE(d.title, '')) ||
			to_tsvector('english', COALESCE(c.content_text, ''))
		  ) @@ to_tsquery('english', ?)
	`
	args := []interface{}{tsQuery, workspaceID, tsQuery}

	if len(spaceIDs) > 0 {
		sql += " AND d.space_id IN (?)"
		args = append(args, spaceIDs)
	}
	if status != nil && *status != "" {
		sql += " AND d.status = ?"
		args = append(args, *status)
	} else {
		// By default, exclude archived documents from search results.
		sql += " AND d.status != 'archived'"
	}

	sql += " ORDER BY rank DESC LIMIT ?"
	args = append(args, limit)

	var results []DocsSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("docs search: %w", err)
	}
	return results, nil
}

// SearchWithContext enriches only the ranked result set, avoiding block scans for
// every document considered by workspace search.
func (r *DocsSearchRepository) SearchWithContext(ctx context.Context, workspaceID, query string, limit int) ([]DocsSearchResult, error) {
	results, err := r.Search(ctx, workspaceID, query, nil, nil, limit)
	if err != nil || len(results) == 0 {
		return results, err
	}
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.ID)
	}
	var matches []struct {
		DocumentID   string
		MatchBlockID string
		MatchText    string
	}
	err = r.db.WithContext(ctx).Raw(`
 SELECT c.document_id, hit.id AS match_block_id,
   ts_headline('english', COALESCE(hit.content_text,c.content_text,''),to_tsquery('english',?), 'MaxWords=35, MinWords=10') AS match_text
 FROM docs_contents c
 LEFT JOIN LATERAL (
   SELECT b.id,b.content_text FROM docs_blocks b
   WHERE b.document_id=c.document_id AND b.workspace_id=? AND b.deleted_at IS NULL
     AND to_tsvector('english',COALESCE(b.content_text,'')) @@ to_tsquery('english',?)
   ORDER BY (b.type='heading') DESC,b.sort_key,b.id LIMIT 1
 ) hit ON true
 WHERE c.document_id IN ?`, toTSQuery(query), workspaceID, toTSQuery(query), ids).Scan(&matches).Error
	if err != nil {
		return nil, fmt.Errorf("read document search matches: %w", err)
	}
	byID := make(map[string]int, len(results))
	for i, result := range results {
		byID[result.ID] = i
	}
	for _, match := range matches {
		if i, ok := byID[match.DocumentID]; ok {
			results[i].MatchBlockID = match.MatchBlockID
			results[i].MatchText = match.MatchText
		}
	}
	return results, nil
}

// PublicSearch searches published help center article translations for a single locale.
func (r *DocsSearchRepository) PublicSearch(ctx context.Context, workspaceID, locale, query, spaceSlug string, limit int) ([]model.PublicSearchResultResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = defaultPublicSearchLimit
	}

	if r.db.Dialector.Name() != "postgres" {
		return r.publicSearchFallback(ctx, workspaceID, locale, query, spaceSlug, limit)
	}
	return r.publicSearchPostgres(ctx, workspaceID, locale, query, spaceSlug, limit)
}

type publicSearchEntryRow struct {
	ID                 string
	Title              string
	Slug               string
	PublicID           string
	Locale             string
	Excerpt            *string
	CollectionID       *string
	CollectionName     *string
	CollectionSlug     *string
	CollectionPublicID *string
	SpaceSlug          string
	SpaceName          string
	EntryType          string
	EntryContent       string
	SectionTitle       *string
	Anchor             *string
	Position           int
	Score              float64
}

func (r *DocsSearchRepository) publicSearchPostgres(ctx context.Context, workspaceID, locale, query, spaceSlug string, limit int) ([]model.PublicSearchResultResponse, error) {
	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return nil, nil
	}

	sql := `
		WITH matched_entries AS (
			SELECT
				se.document_id,
				se.entry_type,
				se.content AS entry_content,
				se.section_title,
				se.anchor,
				se.position,
				(
					CASE
						WHEN lower(se.content) = lower(?) THEN 12
						WHEN lower(se.content) LIKE lower(?) THEN 8
						WHEN se.entry_type = 'title' THEN 4
						ELSE 0
					END
					+ se.rank_weight
					+ ts_rank_cd(se.search_vector, to_tsquery(se.search_config::regconfig, ?)) * 10
					+ GREATEST(word_similarity(?, se.content), similarity(se.content, ?)) * 2
				) AS score
			FROM docs_helpcenter_search_entries se
			WHERE se.workspace_id = ?
				AND se.locale = ?
				AND (
					se.search_vector @@ to_tsquery(se.search_config::regconfig, ?)
					OR word_similarity(?, se.content) >= 0.45
					OR se.content ILIKE ?
				)
		)
		SELECT
			p.document_id AS id,
			p.title AS title,
			p.slug AS slug,
			ha.public_id AS public_id,
			p.locale AS locale,
			p.excerpt AS excerpt,
			p.collection_id AS collection_id,
			ct.name AS collection_name,
			ct.slug AS collection_slug,
			cc.public_id AS collection_public_id,
			st.slug AS space_slug,
			st.name AS space_name,
			me.entry_type AS entry_type,
			me.entry_content AS entry_content,
			me.section_title AS section_title,
			me.anchor AS anchor,
			me.position AS position,
			me.score AS score
		FROM matched_entries me
		JOIN docs_helpcenter_article_publications p
			ON p.document_id = me.document_id
			AND p.locale = ?
		JOIN docs_documents d ON d.id = p.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id
		JOIN docs_helpcenter_space_translations st
			ON st.space_id = p.space_id
			AND st.locale = p.locale
			AND st.status = ?
			AND st.published_at IS NOT NULL
		LEFT JOIN docs_helpcenter_collection_translations ct
			ON ct.collection_id = p.collection_id
			AND ct.locale = p.locale
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
		LEFT JOIN docs_collections cc ON cc.id = p.collection_id AND cc.deleted_at IS NULL
		WHERE p.workspace_id = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
	`
	args := []interface{}{
		query, query + "%", tsQuery, query, query,
		workspaceID, locale, tsQuery, query, "%" + query + "%",
		locale,
		model.DocsHelpcenterTranslationStatusPublished,
		model.DocsHelpcenterTranslationStatusPublished,
		workspaceID,
		model.DocStatusPublished,
	}

	if spaceSlug != "" {
		sql += " AND st.slug = ?"
		args = append(args, spaceSlug)
	}

	sql += " ORDER BY me.score DESC, me.position ASC LIMIT ?"
	args = append(args, limit*10)

	var rows []publicSearchEntryRow
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("docs public search: %w", err)
	}
	collectionRows, err := r.publicSearchCollectionRows(ctx, workspaceID, locale, spaceSlug, query)
	if err != nil {
		return nil, err
	}
	rows = append(rows, collectionRows...)
	publicationRows, err := r.publicSearchPublicationRows(ctx, workspaceID, locale, spaceSlug, query)
	if err != nil {
		return nil, err
	}
	rows = append(rows, publicationRows...)
	matched := make([]publicSearchEntryRow, 0, len(rows))
	for _, row := range rows {
		score, ok := fallbackPublicSearchScore(row, query)
		if !ok {
			continue
		}
		row.Score = score
		matched = append(matched, row)
	}
	sortPublicSearchRows(matched, query)
	rows = matched
	return r.groupPublicSearchRows(ctx, rows, locale, query, limit)
}

func (r *DocsSearchRepository) publicSearchFallback(ctx context.Context, workspaceID, locale, query, spaceSlug string, limit int) ([]model.PublicSearchResultResponse, error) {
	sql := `
		SELECT
			p.document_id AS id,
			p.title AS title,
			p.slug AS slug,
			ha.public_id AS public_id,
			p.locale AS locale,
			p.excerpt AS excerpt,
			p.collection_id AS collection_id,
			ct.name AS collection_name,
			ct.slug AS collection_slug,
			cc.public_id AS collection_public_id,
			st.slug AS space_slug,
			st.name AS space_name,
			se.entry_type AS entry_type,
			se.content AS entry_content,
			se.section_title AS section_title,
			se.anchor AS anchor,
			se.position AS position,
			se.rank_weight AS score
		FROM docs_helpcenter_search_entries se
		JOIN docs_helpcenter_article_publications p
			ON p.document_id = se.document_id
			AND p.locale = se.locale
		JOIN docs_documents d ON d.id = p.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id
		JOIN docs_helpcenter_space_translations st
			ON st.space_id = p.space_id
			AND st.locale = p.locale
			AND st.status = ?
			AND st.published_at IS NOT NULL
		LEFT JOIN docs_helpcenter_collection_translations ct
			ON ct.collection_id = p.collection_id
			AND ct.locale = p.locale
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
		LEFT JOIN docs_collections cc ON cc.id = p.collection_id AND cc.deleted_at IS NULL
		WHERE se.workspace_id = ?
			AND se.locale = ?
			AND p.workspace_id = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
	`
	args := []interface{}{
		model.DocsHelpcenterTranslationStatusPublished,
		model.DocsHelpcenterTranslationStatusPublished,
		workspaceID,
		locale,
		workspaceID,
		model.DocStatusPublished,
	}
	if spaceSlug != "" {
		sql += " AND st.slug = ?"
		args = append(args, spaceSlug)
	}

	var rows []publicSearchEntryRow
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("docs public search: %w", err)
	}
	collectionRows, err := r.publicSearchCollectionRows(ctx, workspaceID, locale, spaceSlug, query)
	if err != nil {
		return nil, err
	}
	rows = append(rows, collectionRows...)
	publicationRows, err := r.publicSearchPublicationRows(ctx, workspaceID, locale, spaceSlug, query)
	if err != nil {
		return nil, err
	}
	rows = append(rows, publicationRows...)

	matched := make([]publicSearchEntryRow, 0, len(rows))
	for _, row := range rows {
		score, ok := fallbackPublicSearchScore(row, query)
		if !ok {
			continue
		}
		row.Score = score
		matched = append(matched, row)
	}
	sortPublicSearchRows(matched, query)
	return r.groupPublicSearchRows(ctx, matched, locale, query, limit)
}

func (r *DocsSearchRepository) publicSearchPublicationRows(ctx context.Context, workspaceID, locale, spaceSlug, query string) ([]publicSearchEntryRow, error) {
	entryContent := `TRIM(COALESCE(p.title, '') || ' ' || COALESCE(p.excerpt, '') || ' ' || COALESCE(p.content_text, ''))`
	sql := `
		SELECT
			p.document_id AS id,
			p.title AS title,
			p.slug AS slug,
			ha.public_id AS public_id,
			p.locale AS locale,
			p.excerpt AS excerpt,
			p.collection_id AS collection_id,
			ct.name AS collection_name,
			ct.slug AS collection_slug,
			cc.public_id AS collection_public_id,
			st.slug AS space_slug,
			st.name AS space_name,
			? AS entry_type,
			` + entryContent + ` AS entry_content,
			NULL AS section_title,
			NULL AS anchor,
			0 AS position,
			3 AS score
		FROM docs_helpcenter_article_publications p
		JOIN docs_documents d ON d.id = p.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id
		JOIN docs_helpcenter_space_translations st
			ON st.space_id = p.space_id
			AND st.locale = p.locale
			AND st.status = ?
			AND st.published_at IS NOT NULL
		LEFT JOIN docs_helpcenter_collection_translations ct
			ON ct.collection_id = p.collection_id
			AND ct.locale = p.locale
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
		LEFT JOIN docs_collections cc ON cc.id = p.collection_id AND cc.deleted_at IS NULL
		WHERE p.workspace_id = ?
			AND p.locale = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
	`
	args := []interface{}{
		model.DocsHelpcenterSearchEntryTypeTitle,
		model.DocsHelpcenterTranslationStatusPublished,
		model.DocsHelpcenterTranslationStatusPublished,
		workspaceID,
		locale,
		model.DocStatusPublished,
	}
	if spaceSlug != "" {
		sql += " AND st.slug = ?"
		args = append(args, spaceSlug)
	}
	if r.db.Dialector.Name() == "postgres" {
		tsQuery := toTSQuery(query)
		if tsQuery == "" {
			return nil, nil
		}
		sql += `
			AND (
				to_tsvector('simple'::regconfig, ` + entryContent + `) @@ to_tsquery('simple'::regconfig, ?)
				OR word_similarity(?, ` + entryContent + `) >= 0.45
				OR ` + entryContent + ` ILIKE ?
			)
		`
		args = append(args, tsQuery, query, "%"+query+"%")
	}

	var rows []publicSearchEntryRow
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("docs public publication search: %w", err)
	}
	return rows, nil
}

func (r *DocsSearchRepository) publicSearchCollectionRows(ctx context.Context, workspaceID, locale, spaceSlug, query string) ([]publicSearchEntryRow, error) {
	entryContent := `TRIM(COALESCE(ct.name, '') || ' ' || COALESCE(ct.slug, ''))`
	sql := `
		SELECT
			p.document_id AS id,
			p.title AS title,
			p.slug AS slug,
			ha.public_id AS public_id,
			p.locale AS locale,
			p.excerpt AS excerpt,
			p.collection_id AS collection_id,
			ct.name AS collection_name,
			ct.slug AS collection_slug,
			cc.public_id AS collection_public_id,
			st.slug AS space_slug,
			st.name AS space_name,
			? AS entry_type,
			` + entryContent + ` AS entry_content,
			NULL AS section_title,
			NULL AS anchor,
			0 AS position,
			5 AS score
		FROM docs_helpcenter_article_publications p
		JOIN docs_documents d ON d.id = p.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id
		JOIN docs_collections cc ON cc.id = p.collection_id AND cc.deleted_at IS NULL
		JOIN docs_helpcenter_collection_translations ct
			ON ct.collection_id = p.collection_id
			AND ct.locale = p.locale
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
		JOIN docs_helpcenter_space_translations st
			ON st.space_id = p.space_id
			AND st.locale = p.locale
			AND st.status = ?
			AND st.published_at IS NOT NULL
		WHERE p.workspace_id = ?
			AND p.locale = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
	`
	args := []interface{}{
		publicSearchEntryCollection,
		model.DocsHelpcenterTranslationStatusPublished,
		model.DocsHelpcenterTranslationStatusPublished,
		workspaceID,
		locale,
		model.DocStatusPublished,
	}
	if spaceSlug != "" {
		sql += " AND st.slug = ?"
		args = append(args, spaceSlug)
	}
	if r.db.Dialector.Name() == "postgres" {
		tsQuery := toTSQuery(query)
		if tsQuery == "" {
			return nil, nil
		}
		sql += `
			AND (
				to_tsvector('simple'::regconfig, ` + entryContent + `) @@ to_tsquery('simple'::regconfig, ?)
				OR word_similarity(?, ` + entryContent + `) >= 0.45
				OR ` + entryContent + ` ILIKE ?
			)
		`
		args = append(args, tsQuery, query, "%"+query+"%")
	}

	var rows []publicSearchEntryRow
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("docs public collection search: %w", err)
	}
	return rows, nil
}

func (r *DocsSearchRepository) groupPublicSearchRows(ctx context.Context, rows []publicSearchEntryRow, locale, query string, limit int) ([]model.PublicSearchResultResponse, error) {
	sortPublicSearchRows(rows, query)
	results := make([]model.PublicSearchResultResponse, 0, limit)
	byID := map[string]int{}
	for _, row := range rows {
		idx, ok := byID[row.ID]
		if !ok {
			if len(results) >= limit {
				continue
			}
			results = append(results, model.PublicSearchResultResponse{
				ID:                 row.ID,
				Title:              row.Title,
				Slug:               row.Slug,
				PublicID:           row.PublicID,
				Locale:             row.Locale,
				Excerpt:            row.Excerpt,
				CollectionID:       row.CollectionID,
				CollectionName:     row.CollectionName,
				CollectionSlug:     row.CollectionSlug,
				CollectionPublicID: row.CollectionPublicID,
				SpaceSlug:          row.SpaceSlug,
				SpaceName:          row.SpaceName,
				Matches:            []model.PublicSearchMatchResponse{},
			})
			idx = len(results) - 1
			byID[row.ID] = idx
		}
		if len(results[idx].Matches) >= 3 {
			continue
		}
		results[idx].Matches = append(results[idx].Matches, model.PublicSearchMatchResponse{
			EntryType:    row.EntryType,
			SectionTitle: row.SectionTitle,
			Anchor:       row.Anchor,
			Snippet:      buildPublicSearchSnippet(row.EntryContent, query),
		})
	}

	// Build a localized ancestor-path string for every result that has
	// an owning collection. The loop is bounded by (limit × depth ≤ 3)
	// and reuses a per-call cache so sibling results under the same
	// collection don't re-query the ancestor chain.
	if len(results) > 0 {
		pathCache := make(map[string]*string, len(results))
		for i := range results {
			if results[i].CollectionID == nil || *results[i].CollectionID == "" {
				continue
			}
			if cached, ok := pathCache[*results[i].CollectionID]; ok {
				results[i].CollectionAncestorPath = cached
				continue
			}
			path, err := r.buildLocalizedCollectionPath(ctx, *results[i].CollectionID, locale)
			if err != nil {
				return nil, err
			}
			pathCache[*results[i].CollectionID] = path
			results[i].CollectionAncestorPath = path
		}
	}

	return results, nil
}

func fallbackPublicSearchScore(row publicSearchEntryRow, query string) (float64, bool) {
	terms := publicSearchTerms(query)
	if len(terms) == 0 {
		return 0, false
	}
	words := publicSearchTerms(row.EntryContent)
	if len(words) == 0 {
		return 0, false
	}
	score := row.Score
	for _, term := range terms {
		matched := false
		for _, word := range words {
			if publicSearchWordMatchesTerm(word, term) {
				score += 4
				matched = true
				break
			}
		}
		if !matched {
			return 0, false
		}
	}
	if row.EntryType == model.DocsHelpcenterSearchEntryTypeTitle {
		score += 4
	}
	if row.EntryType == publicSearchEntryCollection {
		score += 3
	}
	return score, true
}

func sortPublicSearchRows(rows []publicSearchEntryRow, query string) {
	sort.SliceStable(rows, func(i, j int) bool {
		left := publicSearchRowSortScore(rows[i], query)
		right := publicSearchRowSortScore(rows[j], query)
		if left == right {
			return rows[i].Position < rows[j].Position
		}
		return left > right
	})
}

func publicSearchRowSortScore(row publicSearchEntryRow, query string) float64 {
	score := row.Score
	switch row.EntryType {
	case model.DocsHelpcenterSearchEntryTypeTitle:
		score += 6
	case model.DocsHelpcenterSearchEntryTypeHeading:
		score += 3
	case publicSearchEntryCollection:
		score += 5
	}
	snippet := buildPublicSearchSnippet(row.EntryContent, query)
	if strings.Contains(snippet, "<mark>") {
		score += 8
	}
	return score
}

func publicSearchWordMatchesTerm(word, term string) bool {
	if strings.HasPrefix(word, term) {
		return true
	}
	if len([]rune(term)) < 4 || len([]rune(word)) < 4 {
		return false
	}
	if []rune(word)[0] != []rune(term)[0] {
		return false
	}
	distance := levenshteinDistance(term, word)
	return distance <= publicSearchTypoTolerance(term)
}

func publicSearchTerms(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		if len([]rune(field)) < 2 || isPublicSearchStopword(field) {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func isPublicSearchStopword(term string) bool {
	switch term {
	case "a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "has", "have", "how", "in", "into", "is", "it", "of", "on", "or", "that", "the", "this", "to", "what", "when", "where", "which", "with", "why", "you", "your":
		return true
	default:
		return false
	}
}

func publicSearchTypoTolerance(term string) int {
	if len([]rune(term)) <= 6 {
		return 1
	}
	return 2
}

func buildPublicSearchSnippet(content, query string) string {
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return ""
	}
	terms := publicSearchTerms(query)
	start := 0
	for _, term := range terms {
		if idx := publicSearchTermIndex(content, term); idx >= 0 {
			start = idx - 60
			if start < 0 {
				start = 0
			}
			break
		}
	}
	snippet := content[start:]
	if start > 0 {
		snippet = "..." + snippet
	}
	runes := []rune(snippet)
	if len(runes) > 220 {
		snippet = string(runes[:220]) + "..."
	}
	escaped := html.EscapeString(snippet)
	for _, term := range terms {
		escaped = markPublicSearchTerm(escaped, term)
	}
	return escaped
}

func publicSearchTermIndex(content, term string) int {
	re := regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])` + regexp.QuoteMeta(term))
	loc := re.FindStringSubmatchIndex(content)
	if loc == nil {
		return -1
	}
	if loc[2] >= 0 {
		return loc[2]
	}
	return loc[0]
}

func markPublicSearchTerm(content, term string) string {
	re := regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])(` + regexp.QuoteMeta(html.EscapeString(term)) + `)`)
	return re.ReplaceAllString(content, `${1}<mark>${2}</mark>`)
}

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr := make([]int, len(br)+1)
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			curr[j] = minInt(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(br)]
}

func minInt(values ...int) int {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

// buildLocalizedCollectionPath walks a collection's ancestor chain and
// returns a slash-joined breadcrumb string like "Root / Middle / Self"
// using localized names per the requested locale, falling back to the
// source collection name when no translation exists. Returns nil when
// the collection has no ancestors beyond itself (top-level single node).
func (r *DocsSearchRepository) buildLocalizedCollectionPath(ctx context.Context, collectionID, locale string) (*string, error) {
	// Load the target collection + every ancestor in a single query.
	type nameRow struct {
		ID                 string  `gorm:"column:id"`
		ParentCollectionID *string `gorm:"column:parent_collection_id"`
		SourceName         string  `gorm:"column:source_name"`
		LocaleName         *string `gorm:"column:locale_name"`
	}
	const maxDepth = 4 // depth cap 0..2 + safety margin
	chain := make([]nameRow, 0, maxDepth)
	cursor := collectionID
	for i := 0; i < maxDepth; i++ {
		var row nameRow
		q := r.db.WithContext(ctx).
			Table("docs_collections c").
			Select(`
				c.id AS id,
				c.parent_collection_id AS parent_collection_id,
				c.name AS source_name,
				ct.name AS locale_name
			`).
			Joins(`
				LEFT JOIN docs_helpcenter_collection_translations ct
					ON ct.collection_id = c.id
					AND ct.locale = ?
			`, locale).
			Where("c.id = ? AND c.deleted_at IS NULL", cursor)
		if err := q.Scan(&row).Error; err != nil {
			return nil, fmt.Errorf("load collection for search path: %w", err)
		}
		if row.ID == "" {
			break
		}
		chain = append(chain, row)
		if row.ParentCollectionID == nil || *row.ParentCollectionID == "" {
			break
		}
		cursor = *row.ParentCollectionID
	}

	if len(chain) <= 1 {
		// No ancestors -> no breadcrumb context worth rendering.
		return nil, nil
	}

	// chain is walked leaf -> root. Reverse into top-down order.
	parts := make([]string, 0, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		row := chain[i]
		name := row.SourceName
		if row.LocaleName != nil && *row.LocaleName != "" {
			name = *row.LocaleName
		}
		parts = append(parts, name)
	}
	joined := strings.Join(parts, " / ")
	return &joined, nil
}

// toTSQuery converts a user search string to a Postgres tsquery with AND semantics.
func toTSQuery(input string) string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}
	escaped := make([]string, len(words))
	for i, w := range words {
		// Strip non-alphanumeric chars for safety.
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, w)
		if clean != "" {
			escaped[i] = clean + ":*"
		}
	}
	// Filter empty entries.
	var nonEmpty []string
	for _, e := range escaped {
		if e != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}
	return strings.Join(nonEmpty, " & ")
}

// PublicArticleRefsByDocumentIDs resolves published help-center article
// metadata (title, slugs, collection, space) for documents in one locale.
// Documents without a published publication in that locale are omitted — the
// caller may retry with the help center's default locale as fallback.
func (r *DocsSearchRepository) PublicArticleRefsByDocumentIDs(ctx context.Context, workspaceID, locale string, documentIDs []string) ([]model.PublicSearchResultResponse, error) {
	if len(documentIDs) == 0 {
		return []model.PublicSearchResultResponse{}, nil
	}
	sql := `
		SELECT
			p.document_id AS id,
			p.title AS title,
			p.slug AS slug,
			ha.public_id AS public_id,
			p.locale AS locale,
			p.excerpt AS excerpt,
			p.collection_id AS collection_id,
			ct.name AS collection_name,
			ct.slug AS collection_slug,
			cc.public_id AS collection_public_id,
			st.slug AS space_slug,
			st.name AS space_name
		FROM docs_helpcenter_article_publications p
		JOIN docs_documents d ON d.id = p.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id
		JOIN docs_helpcenter_space_translations st
			ON st.space_id = p.space_id
			AND st.locale = p.locale
			AND st.status = ?
			AND st.published_at IS NOT NULL
		LEFT JOIN docs_helpcenter_collection_translations ct
			ON ct.collection_id = p.collection_id
			AND ct.locale = p.locale
			AND ct.status = ?
			AND ct.published_at IS NOT NULL
		LEFT JOIN docs_collections cc ON cc.id = p.collection_id AND cc.deleted_at IS NULL
		WHERE p.workspace_id = ?
			AND p.locale = ?
			AND p.document_id IN ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
	`
	var results []model.PublicSearchResultResponse
	if err := r.db.WithContext(ctx).Raw(sql,
		model.DocsHelpcenterTranslationStatusPublished,
		model.DocsHelpcenterTranslationStatusPublished,
		workspaceID,
		locale,
		documentIDs,
		model.DocStatusPublished,
	).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("resolve public article refs: %w", err)
	}
	for i := range results {
		results[i].RequestedLocale = locale
	}
	return results, nil
}
