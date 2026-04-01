package docsimport

import "fmt"

// Warning represents a conversion issue that is not a hard failure.
type Warning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ConversionResult holds the output of an HTML-to-Tiptap conversion.
type ConversionResult struct {
	Doc      Node      `json:"doc"`
	Warnings []Warning `json:"warnings,omitempty"`
}

func warnUnsupportedIframe(src string) Warning {
	return Warning{Type: "unsupported_iframe", Message: fmt.Sprintf("iframe converted to link: %s", src)}
}

func warnStrippedElement(tag string) Warning {
	return Warning{Type: "stripped_element", Message: fmt.Sprintf("unsupported element stripped: <%s>", tag)}
}

func warnCalloutGuess(original, mapped string) Warning {
	return Warning{Type: "callout_variant_guess", Message: fmt.Sprintf("callout %q mapped to variant %q", original, mapped)}
}

func warnImageDownloadFailed(src string) Warning {
	return Warning{Type: "image_download_failed", Message: fmt.Sprintf("image download failed, original URL kept: %s", src)}
}

func warnHTMLBlockFallback() Warning {
	return Warning{Type: "html_block_fallback", Message: "unsupported content preserved as HTML block"}
}

func warnHelpScoutNoteBlockNormalized() Warning {
	return Warning{Type: "helpscout_note_block_normalized", Message: "escaped Help Scout note block normalized to callout"}
}

func warnEmptyHeadingRemoved() Warning {
	return Warning{Type: "empty_heading_removed", Message: "empty heading removed during import normalization"}
}

func warnBlankParagraphRemoved() Warning {
	return Warning{Type: "blank_paragraph_removed", Message: "blank paragraph removed during import normalization"}
}

func warnImageURLKept(src string) Warning {
	return Warning{Type: "image_url_kept", Message: fmt.Sprintf("image URL kept without rewrite: %s", src)}
}
