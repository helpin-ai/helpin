package model

import (
	"regexp"
	"strings"
)

type ApprovalRequest struct {
	Phase   string `json:"phase"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

var (
	specDraftBlockRe       = regexp.MustCompile(`(?is)<spec_draft>\s*([\s\S]*?)\s*</spec_draft>`)
	storyPlanBlockRe       = regexp.MustCompile(`(?is)<story_plan>\s*([\s\S]*?)\s*</story_plan>`)
	approvalRequestBlockRe = regexp.MustCompile(`(?is)<approval_request\b([^>]*?)(?:>([\s\S]*?)</approval_request>|/\s*>)`)
	approvalPhaseAttrRe    = regexp.MustCompile(`(?is)\bphase\s*=\s*['"]([^'"]+)['"]`)
	approvalTitleAttrRe    = regexp.MustCompile(`(?is)\btitle\s*=\s*['"]([^'"]+)['"]`)
	approvalSummaryAttrRe  = regexp.MustCompile(`(?is)\bsummary\s*=\s*['"]([^'"]+)['"]`)
	approvalTitleRe        = regexp.MustCompile(`(?is)<title>\s*([\s\S]*?)\s*</title>`)
	approvalSummaryRe      = regexp.MustCompile(`(?is)<summary>\s*([\s\S]*?)\s*</summary>`)
)

func ExtractLatestSpecDraft(text string) string {
	return extractLatestTaggedBlock(specDraftBlockRe, text)
}

func ExtractLatestStoryPlan(text string) string {
	return extractLatestTaggedBlock(storyPlanBlockRe, text)
}

func ExtractLatestApprovalRequest(text string) *ApprovalRequest {
	matches := approvalRequestBlockRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	match := matches[len(matches)-1]
	attrs := strings.TrimSpace(match[1])
	body := strings.TrimSpace(match[2])
	phaseMatch := approvalPhaseAttrRe.FindStringSubmatch(attrs)
	if len(phaseMatch) < 2 {
		return nil
	}
	phase := strings.TrimSpace(strings.ToLower(phaseMatch[1]))
	if phase == "" {
		return nil
	}

	title := body
	if titleMatch := approvalTitleRe.FindStringSubmatch(body); len(titleMatch) >= 2 {
		title = strings.TrimSpace(titleMatch[1])
	} else if titleAttrMatch := approvalTitleAttrRe.FindStringSubmatch(attrs); len(titleAttrMatch) >= 2 {
		title = strings.TrimSpace(titleAttrMatch[1])
	}
	summary := ""
	if summaryMatch := approvalSummaryRe.FindStringSubmatch(body); len(summaryMatch) >= 2 {
		summary = strings.TrimSpace(summaryMatch[1])
	} else if summaryAttrMatch := approvalSummaryAttrRe.FindStringSubmatch(attrs); len(summaryAttrMatch) >= 2 {
		summary = strings.TrimSpace(summaryAttrMatch[1])
	}

	return &ApprovalRequest{
		Phase:   phase,
		Title:   strings.TrimSpace(title),
		Summary: summary,
	}
}

func extractLatestTaggedBlock(re *regexp.Regexp, text string) string {
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}
