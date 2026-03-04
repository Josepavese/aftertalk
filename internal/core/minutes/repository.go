package minutes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/core"
)

type MinutesRepository struct {
	*core.BaseRepository
}

func NewMinutesRepository(db *sql.DB) *MinutesRepository {
	return &MinutesRepository{
		BaseRepository: core.NewBaseRepository(db),
	}
}

func (r *MinutesRepository) Create(ctx context.Context, minutes *Minutes) error {
	themesJSON, _ := json.Marshal(minutes.Themes)
	contentsJSON, _ := json.Marshal(minutes.ContentsReported)
	interventionsJSON, _ := json.Marshal(minutes.ProfessionalInterventions)
	progressJSON, _ := json.Marshal(minutes.ProgressIssues)
	nextStepsJSON, _ := json.Marshal(minutes.NextSteps)
	citationsJSON, _ := json.Marshal(minutes.Citations)

	query := `
		INSERT INTO minutes (id, session_id, version, themes, contents_reported, professional_interventions, progress_issues, next_steps, citations, generated_at, delivered_at, status, provider)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var deliveredAt interface{}
	if minutes.DeliveredAt != nil {
		deliveredAt = minutes.DeliveredAt.Format(time.RFC3339)
	}

	_, err := r.ExecContext(ctx, query,
		minutes.ID,
		minutes.SessionID,
		minutes.Version,
		string(themesJSON),
		string(contentsJSON),
		string(interventionsJSON),
		string(progressJSON),
		string(nextStepsJSON),
		string(citationsJSON),
		minutes.GeneratedAt.Format(time.RFC3339),
		deliveredAt,
		string(minutes.Status),
		minutes.Provider,
	)

	if err != nil {
		return fmt.Errorf("failed to create minutes: %w", err)
	}

	return nil
}

func (r *MinutesRepository) GetByID(ctx context.Context, id string) (*Minutes, error) {
	query := `
		SELECT id, session_id, version, themes, contents_reported, professional_interventions, progress_issues, next_steps, citations, generated_at, delivered_at, status, provider
		FROM minutes
		WHERE id = ?
	`

	var m Minutes
	var themes, contents, interventions, progress, nextSteps, citations string
	var generatedAt, deliveredAt sql.NullString

	err := r.QueryRowContext(ctx, query, id).Scan(
		&m.ID,
		&m.SessionID,
		&m.Version,
		&themes,
		&contents,
		&interventions,
		&progress,
		&nextSteps,
		&citations,
		&generatedAt,
		&deliveredAt,
		&m.Status,
		&m.Provider,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("minutes not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	json.Unmarshal([]byte(themes), &m.Themes)
	json.Unmarshal([]byte(contents), &m.ContentsReported)
	json.Unmarshal([]byte(interventions), &m.ProfessionalInterventions)
	json.Unmarshal([]byte(progress), &m.ProgressIssues)
	json.Unmarshal([]byte(nextSteps), &m.NextSteps)
	json.Unmarshal([]byte(citations), &m.Citations)

	if generatedAt.Valid {
		m.GeneratedAt, _ = time.Parse(time.RFC3339, generatedAt.String)
	}
	if deliveredAt.Valid {
		t, _ := time.Parse(time.RFC3339, deliveredAt.String)
		m.DeliveredAt = &t
	}

	return &m, nil
}

func (r *MinutesRepository) GetBySession(ctx context.Context, sessionID string) (*Minutes, error) {
	query := `
		SELECT id, session_id, version, themes, contents_reported, professional_interventions, progress_issues, next_steps, citations, generated_at, delivered_at, status, provider
		FROM minutes
		WHERE session_id = ?
	`

	var m Minutes
	var themes, contents, interventions, progress, nextSteps, citations string
	var generatedAt, deliveredAt sql.NullString

	err := r.QueryRowContext(ctx, query, sessionID).Scan(
		&m.ID,
		&m.SessionID,
		&m.Version,
		&themes,
		&contents,
		&interventions,
		&progress,
		&nextSteps,
		&citations,
		&generatedAt,
		&deliveredAt,
		&m.Status,
		&m.Provider,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("minutes not found for session: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get minutes: %w", err)
	}

	json.Unmarshal([]byte(themes), &m.Themes)
	json.Unmarshal([]byte(contents), &m.ContentsReported)
	json.Unmarshal([]byte(interventions), &m.ProfessionalInterventions)
	json.Unmarshal([]byte(progress), &m.ProgressIssues)
	json.Unmarshal([]byte(nextSteps), &m.NextSteps)
	json.Unmarshal([]byte(citations), &m.Citations)

	if generatedAt.Valid {
		m.GeneratedAt, _ = time.Parse(time.RFC3339, generatedAt.String)
	}
	if deliveredAt.Valid {
		t, _ := time.Parse(time.RFC3339, deliveredAt.String)
		m.DeliveredAt = &t
	}

	return &m, nil
}

func (r *MinutesRepository) Update(ctx context.Context, minutes *Minutes) error {
	themesJSON, _ := json.Marshal(minutes.Themes)
	contentsJSON, _ := json.Marshal(minutes.ContentsReported)
	interventionsJSON, _ := json.Marshal(minutes.ProfessionalInterventions)
	progressJSON, _ := json.Marshal(minutes.ProgressIssues)
	nextStepsJSON, _ := json.Marshal(minutes.NextSteps)
	citationsJSON, _ := json.Marshal(minutes.Citations)

	query := `
		UPDATE minutes
		SET version = ?, themes = ?, contents_reported = ?, professional_interventions = ?, progress_issues = ?, next_steps = ?, citations = ?, delivered_at = ?, status = ?
		WHERE id = ?
	`

	var deliveredAt interface{}
	if minutes.DeliveredAt != nil {
		deliveredAt = minutes.DeliveredAt.Format(time.RFC3339)
	}

	_, err := r.ExecContext(ctx, query,
		minutes.Version,
		string(themesJSON),
		string(contentsJSON),
		string(interventionsJSON),
		string(progressJSON),
		string(nextStepsJSON),
		string(citationsJSON),
		deliveredAt,
		string(minutes.Status),
		minutes.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update minutes: %w", err)
	}

	return nil
}

func (r *MinutesRepository) CreateHistory(ctx context.Context, history *MinutesHistory) error {
	query := `
		INSERT INTO minutes_history (id, minutes_id, version, content, edited_at, edited_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.ExecContext(ctx, query,
		history.ID,
		history.MinutesID,
		history.Version,
		history.Content,
		history.EditedAt.Format(time.RFC3339),
		history.EditedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to create minutes history: %w", err)
	}

	return nil
}

func (r *MinutesRepository) GetHistory(ctx context.Context, minutesID string) ([]*MinutesHistory, error) {
	query := `
		SELECT id, minutes_id, version, content, edited_at, edited_by
		FROM minutes_history
		WHERE minutes_id = ?
		ORDER BY version DESC
	`

	rows, err := r.QueryContext(ctx, query, minutesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get minutes history: %w", err)
	}
	defer rows.Close()

	var history []*MinutesHistory
	for rows.Next() {
		var h MinutesHistory
		var editedAt string
		var editedBy sql.NullString

		err := rows.Scan(
			&h.ID,
			&h.MinutesID,
			&h.Version,
			&h.Content,
			&editedAt,
			&editedBy,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan minutes history: %w", err)
		}

		h.EditedAt, _ = time.Parse(time.RFC3339, editedAt)
		if editedBy.Valid {
			h.EditedBy = editedBy.String
		}

		history = append(history, &h)
	}

	return history, nil
}
