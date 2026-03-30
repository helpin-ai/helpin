package handler

import "strings"

func buildDocsRedirectTargetPath(collectionSlug string, articleSlug *string) string {
	collection := strings.Trim(strings.TrimSpace(collectionSlug), "/")
	article := ""
	if articleSlug != nil {
		article = strings.Trim(strings.TrimSpace(*articleSlug), "/")
	}

	switch {
	case collection != "" && article != "":
		return "/" + collection + "/" + article
	case collection != "":
		return "/" + collection
	case article != "":
		return "/" + article
	default:
		return "/"
	}
}
