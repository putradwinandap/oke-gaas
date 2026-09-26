package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/putradwinandap/oke-gaas/internal/access"
	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/rule"
)

const requestOperationTimeout = 10 * time.Second

type Dependencies struct {
	Projects  *project.ProvisionService
	Players   *player.Service
	Rules     *rule.Service
	Progress  *progression.Service
	States    progression.Repository
	Access    *access.Service
	AdminKey  string
}

// New creates the HTTP delivery adapter for the Oke Gaas API.
func New(dependencies ...Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: apiErrorHandler})
	app.Use(recoverer.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	if len(dependencies) == 0 {
		return app
	}
	deps := dependencies[0]

	app.Post("/v1/projects", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		if !validBearer(c.Get("Authorization"), deps.AdminKey) {
			return writeError(c, fiber.StatusUnauthorized, "unauthorized", "invalid admin api key")
		}
		var request struct {
			Name string `json:"name"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		result, err := deps.Projects.Provision(ctx, request.Name)
		if err != nil {
			if errors.Is(err, project.ErrInvalidName) || errors.Is(err, project.ErrNameTooLong) {
				return writeError(c, fiber.StatusBadRequest, "invalid_project_name", err.Error())
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not create project")
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"project": fiber.Map{
				"id":         result.Project.ID(),
				"name":       result.Project.Name(),
				"created_at": result.Project.CreatedAt(),
			},
			"api_key": result.APIKey,
		})
	})

	app.Post("/v1/projects/:projectId/players", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if err := authenticateProject(ctx, c, deps.Access, projectID); err != nil {
			return err
		}
		var request struct {
			ExternalID string `json:"external_id"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		value, err := deps.Players.Register(ctx, projectID, request.ExternalID)
		if err != nil {
			switch {
			case errors.Is(err, player.ErrInvalidExternalID), errors.Is(err, player.ErrExternalIDTooLong):
				return writeError(c, fiber.StatusBadRequest, "invalid_external_id", err.Error())
			case errors.Is(err, player.ErrExternalIDTaken):
				return writeError(c, fiber.StatusConflict, "player_exists", err.Error())
			case errors.Is(err, project.ErrNotFound):
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not register player")
			}
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"id":          value.ID(),
			"project_id":  value.ProjectID(),
			"external_id": value.ExternalID(),
			"created_at":  value.CreatedAt(),
		})
	})

	app.Post("/v1/projects/:projectId/rules", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if err := authenticateProject(ctx, c, deps.Access, projectID); err != nil {
			return err
		}
		var request struct {
			EventType string `json:"event_type"`
			XP        int64  `json:"xp"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		value, err := deps.Rules.CreateExactXP(ctx, projectID, request.EventType, request.XP)
		if err != nil {
			switch {
			case errors.Is(err, rule.ErrInvalidEventType), errors.Is(err, rule.ErrEventTypeTooLong), errors.Is(err, rule.ErrInvalidXPAmount):
				return writeError(c, fiber.StatusBadRequest, "invalid_rule", err.Error())
			case errors.Is(err, project.ErrNotFound):
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not create rule")
			}
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"id":         value.ID(),
			"project_id": value.ProjectID(),
			"version":    value.Version(),
			"event_type": value.EventType(),
			"xp":         value.XPAmount(),
		})
	})

	app.Post("/v1/projects/:projectId/events", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if err := authenticateProject(ctx, c, deps.Access, projectID); err != nil {
			return err
		}
		var request struct {
			EventID    string         `json:"event_id"`
			PlayerID   string         `json:"player_id"`
			Type       string         `json:"type"`
			OccurredAt time.Time      `json:"occurred_at"`
			Properties map[string]any `json:"properties"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		result, err := deps.Progress.Process(ctx, eventdomain.IngestCommand{
			ID:         request.EventID,
			ProjectID:  projectID,
			PlayerID:   request.PlayerID,
			Type:       request.Type,
			OccurredAt: request.OccurredAt,
			Properties: request.Properties,
		})
		if err != nil {
			switch {
			case errors.Is(err, eventdomain.ErrInvalidID), errors.Is(err, eventdomain.ErrIDTooLong):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidID.Error())
			case errors.Is(err, eventdomain.ErrInvalidPlayerID):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidPlayerID.Error())
			case errors.Is(err, eventdomain.ErrInvalidType), errors.Is(err, eventdomain.ErrTypeTooLong):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidType.Error())
			case errors.Is(err, eventdomain.ErrInvalidOccurredAt):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidOccurredAt.Error())
			case errors.Is(err, eventdomain.ErrInvalidProperties):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidProperties.Error())
			case errors.Is(err, eventdomain.ErrIdentityConflict):
				return writeError(c, fiber.StatusConflict, "event_identity_conflict", eventdomain.ErrIdentityConflict.Error())
			case errors.Is(err, eventdomain.ErrPlayerNotInProject):
				return writeError(c, fiber.StatusNotFound, "player_not_found", "player not found in project")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not process event")
			}
		}

		grants := make([]fiber.Map, 0, len(result.Grants))
		for _, grant := range result.Grants {
			grants = append(grants, fiber.Map{
				"id":           grant.ID(),
				"rule_id":      grant.RuleID(),
				"rule_version": grant.RuleVersion(),
				"reward_type":  grant.Type(),
				"amount":       grant.Amount(),
			})
		}
		status := fiber.StatusCreated
		if result.Duplicate {
			status = fiber.StatusOK
		}
		return c.Status(status).JSON(fiber.Map{
			"event_id":  result.Event.ID(),
			"duplicate": result.Duplicate,
			"grants":    grants,
			"state": fiber.Map{
				"player_id": result.State.PlayerID(),
				"xp":        result.State.XP(),
				"updated_at": result.State.UpdatedAt(),
			},
		})
	})

	app.Get("/v1/projects/:projectId/players/:playerId/state", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if err := authenticateProject(ctx, c, deps.Access, projectID); err != nil {
			return err
		}
		playerID := c.Params("playerId")
		if _, err := deps.Players.Get(ctx, projectID, playerID); err != nil {
			if errors.Is(err, player.ErrNotFound) {
				return writeError(c, fiber.StatusNotFound, "player_not_found", "player not found")
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not load player")
		}
		state, err := deps.States.Get(ctx, projectID, playerID)
		if err != nil {
			if errors.Is(err, progression.ErrStateNotFound) {
				return c.Status(fiber.StatusOK).JSON(fiber.Map{
					"project_id": projectID,
					"player_id":  playerID,
					"xp":         0,
				})
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not load player state")
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"project_id": state.ProjectID(),
			"player_id":  state.PlayerID(),
			"xp":         state.XP(),
			"updated_at": state.UpdatedAt(),
		})
	})

	app.Use(func(c fiber.Ctx) error {
		return writeError(c, fiber.StatusNotFound, "not_found", "endpoint not found")
	})

	return app
}

func requestContext(c fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Context(), requestOperationTimeout)
}

func authenticateProject(ctx context.Context, c fiber.Ctx, service *access.Service, projectID string) error {
	if service == nil {
		return writeError(c, fiber.StatusInternalServerError, "internal_error", "project authentication is unavailable")
	}
	secret, ok := bearer(c.Get("Authorization"))
	if !ok {
		return writeError(c, fiber.StatusUnauthorized, "unauthorized", "project api key is required")
	}
	if err := service.Authenticate(ctx, projectID, secret); err != nil {
		if errors.Is(err, access.ErrUnauthorized) {
			return writeError(c, fiber.StatusUnauthorized, "unauthorized", "invalid project api key")
		}
		return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not authenticate project")
	}
	return nil
}

func validBearer(header, expected string) bool {
	secret, ok := bearer(header)
	if !ok || strings.TrimSpace(expected) == "" {
		return false
	}
	expectedHash := sha256.Sum256([]byte(expected))
	secretHash := sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare(secretHash[:], expectedHash[:]) == 1
}

func bearer(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

func apiErrorHandler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) && fiberErr != nil {
		status = fiberErr.Code
		message = fiberErr.Message
		code = "request_error"
		switch status {
		case fiber.StatusNotFound:
			code = "not_found"
		case fiber.StatusMethodNotAllowed:
			code = "method_not_allowed"
		case fiber.StatusRequestEntityTooLarge:
			code = "request_too_large"
		}
	}

	return writeError(c, status, code, message)
}

func writeError(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": message,
		},
	})
}
