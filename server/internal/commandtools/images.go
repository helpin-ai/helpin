package commandtools

// Image tools create new private image artifacts with OpenAI GPT Image 2.5.
// They never return image bytes to the model: results are artifact references
// the chat renders inline and docs tools can insert.
var imageRuntimeTools = []RuntimeToolMetadata{
	{CommandName: "agents.generate_image", Alias: "generate_image", Category: "Images",
		Description: "Create a new image from a text prompt with GPT Image 2.5, such as an illustration that explains a concept, a hero image, or a visual for a doc or slide. " +
			"Describe the subject, composition, style, colors, and any short text exactly as it should appear; image models can garble long or dense text. " +
			"For precise technical diagrams (flows, sequences, architecture) prefer a Mermaid code block in a document instead. " +
			"Returns a private image artifact. Show it to the user with the returned markdown, or add it to a document with insert_document_artifact. You cannot see the image yourself.",
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"prompt"},
			"properties": map[string]any{
				"prompt":     map[string]any{"type": "string", "minLength": 1, "maxLength": 8000, "description": "Detailed description of the image to create."},
				"aspect":     imageAspectSchema(false),
				"quality":    imageQualitySchema(),
				"background": imageBackgroundSchema(),
				"name":       map[string]any{"type": "string", "maxLength": 120, "description": "Short descriptive file name, without extension."},
			},
		}},
	{CommandName: "agents.edit_image", Alias: "edit_image", Category: "Images",
		Description: "Change existing images with GPT Image 2.5: for example restyle or clean up a screenshot, remove clutter, replace sample data, highlight an element, extend a background, or combine images. " +
			"Sources are artifact IDs (for example from browser_screenshot or an earlier generate_image or edit_image) or Ask chat attachment IDs. " +
			"You cannot see the images, so describe the change precisely and say what must stay unchanged (for example \"keep all UI text, layout and colors exactly as they are\"). " +
			"The model redraws the image, so small text and exact pixels may change; for pixel-exact crops capture a browser_screenshot with a selector instead. " +
			"Returns a new private image artifact; the sources are not modified.",
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"images", "prompt"},
			"properties": map[string]any{
				"images": map[string]any{"type": "array", "minItems": 1, "maxItems": 4, "uniqueItems": true,
					"items":       map[string]any{"type": "string", "minLength": 1},
					"description": "Source image artifact IDs, helpin://artifacts/ references, or Ask attachment IDs. The first image is the one edited; others are references."},
				"prompt":     map[string]any{"type": "string", "minLength": 1, "maxLength": 8000, "description": "The change to make, including what must stay the same."},
				"mask":       map[string]any{"type": "string", "description": "Optional PNG mask artifact or attachment ID, the same size as the first image; transparent pixels mark the area to change."},
				"aspect":     imageAspectSchema(true),
				"quality":    imageQualitySchema(),
				"background": imageBackgroundSchema(),
				"name":       map[string]any{"type": "string", "maxLength": 120, "description": "Short descriptive file name, without extension."},
			},
		}},
}

func imageAspectSchema(edit bool) map[string]any {
	description := "square (1024x1024), landscape (1536x1024), portrait (1024x1536), or wide 16:9 (1792x1008). Defaults to auto."
	if edit {
		description = "Output shape. Defaults to auto, which follows the first source image. " + description
	}
	return map[string]any{"type": "string", "enum": []string{"auto", "square", "landscape", "portrait", "wide"}, "description": description}
}

func imageQualitySchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"draft", "standard", "final"},
		"description": "draft is fastest for exploring ideas, standard (default) suits most uses, final uses the highest-quality model and takes longer."}
}

func imageBackgroundSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"auto", "transparent", "opaque"},
		"description": "transparent produces a PNG with a transparent background, for icons and overlays."}
}
