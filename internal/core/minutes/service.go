package minutes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowup/aftertalk/internal/ai/llm"
	"github.com/flowup/aftertalk/internal/logging"
	"github.com/google/uuid"
)

type Service struct {
	repo        *MinutesRepository
	llmProvider llm.LLMProvider
}

func NewService(repo *MinutesRepository, provider llm.LLMProvider) *Service {
	return &Service{
		repo:        repo,
		llmProvider: provider,
	}
}

func convertContentItems(items []llm.ContentItem) []ContentItem {
	result := make([]ContentItem, len(items))
	for i, item := range items {
		result[i] = ContentItem{
			Text:      item.Text,
			Timestamp: item.Timestamp,
		}
	}
	return result
}

func convertCitations(citations []llm.Citation) []Citation {
	result := make([]Citation, len(citations))
	for i, c := range citations {
		result[i] = Citation{
			TimestampMs: c.TimestampMs,
			Text:        c.Text,
			Role:        c.Role,
		}
	}
	return result
}

func (s *Service) GenerateMinutes(ctx context.Context, sessionID string, transcriptionText string, roles []string) (*Minutes, error) {
	logging.Infof("Generating minutes for session %s", sessionID)

	minutes := NewMinutes(uuid.New().String(), sessionID)
	minutes.Provider = s.llmProvider.Name()

	if err := s.repo.Create(ctx, minutes); err != nil {
		return nil, fmt.Errorf("failed to create minutes: %w", err)
	}

	prompt := llm.GenerateMinutesPrompt(transcriptionText, roles)
	response, err := s.llmProvider.Generate(ctx, prompt)
	if err != nil {
		minutes.MarkError()
		s.repo.Update(ctx, minutes)
		return nil, fmt.Errorf("failed to generate minutes: %w", err)
	}

	minutesResponse, err := llm.ParseMinutesResponse(response)
	if err != nil {
		minutes.MarkError()
		s.repo.Update(ctx, minutes)
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	minutes.Themes = minutesResponse.Themes
	minutes.ContentsReported = convertContentItems(minutesResponse.ContentsReported)
	minutes.ProfessionalInterventions = convertContentItems(minutesResponse.ProfessionalInterventions)
	minutes.ProgressIssues = Progress{
		Progress: minutesResponse.ProgressIssues.Progress,
		Issues:   minutesResponse.ProgressIssues.Issues,
	}
	minutes.NextSteps = minutesResponse.NextSteps
	minutes.Citations = convertCitations(minutesResponse.Citations)
	minutes.MarkReady()

	if err := s.repo.Update(ctx, minutes); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	logging.Infof("Minutes generated successfully for session %s", sessionID)
	return minutes, nil
}

func (s *Service) GetMinutes(ctx context.Context, sessionID string) (*Minutes, error) {
	minutes, err := s.repo.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	return minutes, nil
}

func (s *Service) GetMinutesByID(ctx context.Context, id string) (*Minutes, error) {
	minutes, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	return minutes, nil
}

func (s *Service) UpdateMinutes(ctx context.Context, id string, updatedMinutes *Minutes, editedBy string) (*Minutes, error) {
	minutes, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	contentJSON, _ := json.Marshal(minutes)
	history := NewMinutesHistory(uuid.New().String(), minutes.ID, minutes.Version, string(contentJSON))
	history.SetEditedBy(editedBy)

	if err := s.repo.CreateHistory(ctx, history); err != nil {
		return nil, fmt.Errorf("failed to create history: %w", err)
	}

	minutes.Themes = updatedMinutes.Themes
	minutes.ContentsReported = updatedMinutes.ContentsReported
	minutes.ProfessionalInterventions = updatedMinutes.ProfessionalInterventions
	minutes.ProgressIssues = updatedMinutes.ProgressIssues
	minutes.NextSteps = updatedMinutes.NextSteps
	minutes.Citations = updatedMinutes.Citations
	minutes.IncrementVersion()

	if err := s.repo.Update(ctx, minutes); err != nil {
		return nil, fmt.Errorf("failed to update minutes: %w", err)
	}

	return minutes, nil
}

func (s *Service) GetMinutesHistory(ctx context.Context, minutesID string) ([]*MinutesHistory, error) {
	history, err := s.repo.GetHistory(ctx, minutesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	return history, nil
}
