package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetImageTools enables the generate_image and edit_image agent tools.
func (s *InternalCommandService) SetImageTools(images *AgentImageService) *InternalCommandService {
	if s == nil {
		return s
	}
	s.imageService = images
	return s
}

// registerImageToolCommands registers the agent image tools. They are always
// listed so agents learn the capability; without an OpenAI credential they
// return an actionable error naming where to add one.
func (s *InternalCommandService) registerImageToolCommands() {
	s.register(InternalCommandDefinition{
		Name: "agents.generate_image", Module: "agents", Mutating: true,
		Tool: mustCommandToolMetadata("agents.generate_image"), Execute: s.executeGenerateImage,
	})
	s.register(InternalCommandDefinition{
		Name: "agents.edit_image", Module: "agents", Mutating: true,
		Tool: mustCommandToolMetadata("agents.edit_image"), Execute: s.executeEditImage,
	})
}

type imageToolInput struct {
	Prompt     string   `json:"prompt"`
	Images     []string `json:"images,omitempty"`
	Mask       string   `json:"mask,omitempty"`
	Aspect     string   `json:"aspect,omitempty"`
	Quality    string   `json:"quality,omitempty"`
	Background string   `json:"background,omitempty"`
	Name       string   `json:"name,omitempty"`
}

func (s *InternalCommandService) executeGenerateImage(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	return s.executeImageTool(ctx, meta, input, false)
}

func (s *InternalCommandService) executeEditImage(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	return s.executeImageTool(ctx, meta, input, true)
}

func (s *InternalCommandService) executeImageTool(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage, edit bool) (json.RawMessage, error) {
	if s.imageService == nil {
		return nil, ErrAgentImagesUnavailable
	}
	var req imageToolInput
	if err := decodeStrictInternalCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse image tool input: %w", err)
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	request := AgentImageRequest{Prompt: req.Prompt, Images: req.Images, Mask: req.Mask,
		Aspect: req.Aspect, Quality: req.Quality, Background: req.Background, Name: req.Name}
	var result *AgentImageResult
	if edit {
		result, err = s.imageService.Edit(ctx, meta, run, request)
	} else {
		result, err = s.imageService.Generate(ctx, meta, run, request)
	}
	if err != nil {
		return nil, err
	}
	return mustJSON(result), nil
}
