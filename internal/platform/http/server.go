package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/putradwinandap/oke-gaas/internal/access"
	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/counter"
	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/level"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/rule"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const requestOperationTimeout = 10 * time.Second

type Dependencies struct {
	Projects *project.ProvisionService
	Players  *player.Service
	Levels   *level.Service
	Counters *counter.Service
	Badges   *badge.Service
	Rules    *rule.Service
	Progress *progression.Service
	States   progression.Repository
	Access   *access.Service
	AdminKey string
}

// New creates the HTTP delivery adapter for the Oke Gaas API.
func New(dependencies ...Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: apiErrorHandler})
	app.Use(recoverer.New())
	tracer := otel.Tracer(instrumentationName)
	meter := otel.Meter(instrumentationName)
	app.Use(telemetryMiddleware(tracer, meter))
	eventMetrics := newEventProcessingMetrics(meter)

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
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
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

	app.Post("/v1/projects/:projectId/levels", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		var request struct {
			MinXP int64 `json:"min_xp"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		value, err := deps.Levels.Append(ctx, projectID, request.MinXP)
		if err != nil {
			switch {
			case errors.Is(err, level.ErrInvalidMinXP), errors.Is(err, level.ErrThresholdNotIncreasing):
				return writeError(c, fiber.StatusBadRequest, "invalid_level", err.Error())
			case errors.Is(err, project.ErrNotFound):
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not create level threshold")
			}
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"project_id": value.ProjectID(),
			"number":     value.Number(),
			"min_xp":     value.MinXP(),
		})
	})

	app.Get("/v1/projects/:projectId/levels", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		values, err := deps.Levels.List(ctx, projectID)
		if err != nil {
			if errors.Is(err, project.ErrNotFound) {
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not list level thresholds")
		}
		thresholds := make([]fiber.Map, 0, len(values)+1)
		thresholds = append(thresholds, fiber.Map{"number": uint64(1), "min_xp": int64(0)})
		for _, value := range values {
			thresholds = append(thresholds, fiber.Map{"number": value.Number(), "min_xp": value.MinXP()})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"project_id": projectID, "levels": thresholds})
	})

	app.Post("/v1/projects/:projectId/counters", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		var request struct {
			Name       string         `json:"name"`
			EventType  string         `json:"event_type"`
			Conditions map[string]any `json:"conditions"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		value, err := deps.Counters.Create(ctx, projectID, request.Name, request.EventType, request.Conditions)
		if err != nil {
			switch {
			case errors.Is(err, counter.ErrInvalidName), errors.Is(err, counter.ErrNameTooLong),
				errors.Is(err, counter.ErrInvalidEventType), errors.Is(err, counter.ErrEventTypeTooLong),
				errors.Is(err, counter.ErrInvalidConditions):
				return writeError(c, fiber.StatusBadRequest, "invalid_counter", err.Error())
			case errors.Is(err, counter.ErrNameTaken):
				return writeError(c, fiber.StatusConflict, "counter_exists", err.Error())
			case errors.Is(err, project.ErrNotFound):
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not create counter")
			}
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"id":         value.ID(),
			"project_id": value.ProjectID(),
			"name":       value.Name(),
			"event_type": value.EventType(),
			"conditions": value.Conditions(),
			"created_at": value.CreatedAt(),
		})
	})

	app.Get("/v1/projects/:projectId/counters", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		values, err := deps.Counters.List(ctx, projectID)
		if err != nil {
			if errors.Is(err, project.ErrNotFound) {
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not list counters")
		}
		counters := make([]fiber.Map, 0, len(values))
		for _, value := range values {
			counters = append(counters, fiber.Map{
				"id": value.ID(), "project_id": value.ProjectID(), "name": value.Name(),
				"event_type": value.EventType(), "conditions": value.Conditions(), "created_at": value.CreatedAt(),
			})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"project_id": projectID, "counters": counters})
	})

	app.Get("/v1/projects/:projectId/players/:playerId/counters", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		playerID := c.Params("playerId")
		values, err := deps.Counters.PlayerProgress(ctx, projectID, playerID)
		if err != nil {
			if errors.Is(err, player.ErrNotFound) {
				return writeError(c, fiber.StatusNotFound, "player_not_found", "player not found")
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not load player counters")
		}
		counters := make([]fiber.Map, 0, len(values))
		for _, value := range values {
			entry := fiber.Map{
				"counter_id": value.Counter.ID(),
				"name":       value.Counter.Name(),
				"event_type": value.Counter.EventType(),
				"conditions": value.Counter.Conditions(),
				"value":      value.Value,
			}
			if value.UpdatedAt != nil {
				entry["updated_at"] = *value.UpdatedAt
			}
			counters = append(counters, entry)
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"project_id": projectID,
			"player_id":  playerID,
			"counters":   counters,
		})
	})

	app.Post("/v1/projects/:projectId/badges", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		var request struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		value, err := deps.Badges.Create(ctx, projectID, request.Name, request.Description)
		if err != nil {
			switch {
			case errors.Is(err, badge.ErrInvalidName), errors.Is(err, badge.ErrNameTooLong), errors.Is(err, badge.ErrDescriptionTooLong):
				return writeError(c, fiber.StatusBadRequest, "invalid_badge", err.Error())
			case errors.Is(err, badge.ErrAlreadyExists):
				return writeError(c, fiber.StatusConflict, "badge_exists", err.Error())
			case errors.Is(err, project.ErrNotFound):
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not create badge")
			}
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": value.ID(), "project_id": value.ProjectID(), "name": value.Name(), "description": value.Description(), "created_at": value.CreatedAt()})
	})
	app.Get("/v1/projects/:projectId/badges", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		values, err := deps.Badges.List(ctx, projectID)
		if err != nil {
			if errors.Is(err, project.ErrNotFound) {
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not list badges")
		}
		items := make([]fiber.Map, 0, len(values))
		for _, value := range values {
			items = append(items, fiber.Map{"id": value.ID(), "project_id": value.ProjectID(), "name": value.Name(), "description": value.Description(), "created_at": value.CreatedAt()})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"project_id": projectID, "badges": items})
	})
	app.Get("/v1/projects/:projectId/players/:playerId/badges", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		playerID := c.Params("playerId")
		values, err := deps.Badges.PlayerBadges(ctx, projectID, playerID)
		if err != nil {
			if errors.Is(err, player.ErrNotFound) {
				return writeError(c, fiber.StatusNotFound, "player_not_found", "player not found")
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not load player badges")
		}
		items := make([]fiber.Map, 0, len(values))
		for _, value := range values {
			items = append(items, fiber.Map{"badge_id": value.Definition.ID(), "name": value.Definition.Name(), "description": value.Definition.Description(), "event_id": value.Grant.EventID(), "rule_id": value.Grant.RuleID(), "rule_version": value.Grant.RuleVersion(), "granted_at": value.Grant.GrantedAt()})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"project_id": projectID, "player_id": playerID, "badges": items})
	})

	app.Post("/v1/projects/:projectId/rules", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
		}
		var request struct {
			RewardType    string         `json:"reward_type"`
			BadgeID       string         `json:"badge_id"`
			EventType     string         `json:"event_type"`
			XP            int64          `json:"xp"`
			Conditions    map[string]any `json:"conditions"`
			MatchEvery    *uint64        `json:"match_every"`
			OncePerUTCDay bool           `json:"once_per_utc_day"`
		}
		if err := c.Bind().Body(&request); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid_request", "request body is invalid")
		}
		matchEvery := uint64(1)
		if request.MatchEvery != nil {
			matchEvery = *request.MatchEvery
		}
		var value *rule.Rule
		var err error
		if request.RewardType == rule.TypeBadge {
			if request.XP != 0 {
				return writeError(c, fiber.StatusBadRequest, "invalid_rule", "Badge rules must not set xp")
			}
			if matchEvery != 1 || request.OncePerUTCDay {
				return writeError(c, fiber.StatusBadRequest, "invalid_rule", "Badge rules do not support aggregate or daily gates")
			}
			value, err = deps.Rules.CreateBadge(ctx, projectID, request.EventType, request.BadgeID, request.Conditions)
		} else if request.RewardType == "" || request.RewardType == rule.TypeXP {
			if request.BadgeID != "" {
				return writeError(c, fiber.StatusBadRequest, "invalid_rule", "XP rules must not set badge_id")
			}
			value, err = deps.Rules.CreateTimedXP(ctx, projectID, request.EventType, request.XP, request.Conditions, matchEvery, request.OncePerUTCDay)
		} else {
			return writeError(c, fiber.StatusBadRequest, "invalid_rule", "unsupported reward_type")
		}
		if err != nil {
			switch {
			case errors.Is(err, rule.ErrInvalidEventType), errors.Is(err, rule.ErrEventTypeTooLong), errors.Is(err, rule.ErrInvalidXPAmount), errors.Is(err, rule.ErrInvalidBadgeID), errors.Is(err, rule.ErrInvalidConditions), errors.Is(err, rule.ErrInvalidMatchEvery), errors.Is(err, rule.ErrIncompatibleTimeWindow):
				return writeError(c, fiber.StatusBadRequest, "invalid_rule", err.Error())
			case errors.Is(err, badge.ErrNotFound):
				return writeError(c, fiber.StatusBadRequest, "invalid_rule", "badge must belong to this project")
			case errors.Is(err, project.ErrNotFound):
				return writeError(c, fiber.StatusNotFound, "project_not_found", "project not found")
			default:
				return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not create rule")
			}
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"id":               value.ID(),
			"project_id":       value.ProjectID(),
			"version":          value.Version(),
			"event_type":       value.EventType(),
			"xp":               value.XPAmount(),
			"reward_type":      value.RewardType(),
			"badge_id":         value.BadgeID(),
			"conditions":       value.Conditions(),
			"match_every":      value.MatchEvery(),
			"once_per_utc_day": value.OncePerUTCDay(),
		})
	})

	app.Post("/v1/projects/:projectId/events", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
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
		processCtx, span := tracer.Start(ctx, "event.process")
		processStarted := time.Now()
		result, err := deps.Progress.Process(processCtx, eventdomain.IngestCommand{
			ID:         request.EventID,
			ProjectID:  projectID,
			PlayerID:   request.PlayerID,
			Type:       request.Type,
			OccurredAt: request.OccurredAt,
			Properties: request.Properties,
		})
		grantCount := 0
		duplicate := false
		if result != nil {
			grantCount = len(result.Grants)
			duplicate = result.Duplicate
		}
		eventMetrics.record(processCtx, processStarted, err, duplicate, grantCount)
		span.SetAttributes(
			attribute.Bool("event.duplicate", duplicate),
			attribute.Int("reward.grant_count", grantCount),
		)
		if err != nil {
			span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
			span.SetStatus(codes.Error, "event processing failed")
		}
		span.End()
		if err != nil {
			switch {
			case errors.Is(err, eventdomain.ErrInvalidID):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidID.Error())
			case errors.Is(err, eventdomain.ErrIDTooLong):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrIDTooLong.Error())
			case errors.Is(err, eventdomain.ErrInvalidPlayerID):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidPlayerID.Error())
			case errors.Is(err, eventdomain.ErrInvalidType):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrInvalidType.Error())
			case errors.Is(err, eventdomain.ErrTypeTooLong):
				return writeError(c, fiber.StatusBadRequest, "invalid_event", eventdomain.ErrTypeTooLong.Error())
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
			item := fiber.Map{
				"id":           grant.ID(),
				"rule_id":      grant.RuleID(),
				"rule_version": grant.RuleVersion(),
				"reward_type":  grant.Type(),
				"amount":       grant.Amount(),
			}
			if grant.BadgeID() != "" {
				item["badge_id"] = grant.BadgeID()
			}
			grants = append(grants, item)
		}
		currentLevel, err := deps.Levels.Resolve(ctx, projectID, result.State.XP())
		if err != nil {
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not resolve player level")
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
				"player_id":  result.State.PlayerID(),
				"xp":         result.State.XP(),
				"level":      currentLevel,
				"updated_at": result.State.UpdatedAt(),
			},
		})
	})

	app.Get("/v1/projects/:projectId/players/:playerId/state", func(c fiber.Ctx) error {
		ctx, cancel := requestContext(c)
		defer cancel()
		projectID := c.Params("projectId")
		if !authenticateProject(ctx, c, deps.Access, projectID) {
			return nil
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
					"level":      uint64(1),
				})
			}
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not load player state")
		}
		currentLevel, err := deps.Levels.Resolve(ctx, projectID, state.XP())
		if err != nil {
			return writeError(c, fiber.StatusInternalServerError, "internal_error", "could not resolve player level")
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"project_id": state.ProjectID(),
			"player_id":  state.PlayerID(),
			"xp":         state.XP(),
			"level":      currentLevel,
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

func authenticateProject(ctx context.Context, c fiber.Ctx, service *access.Service, projectID string) bool {
	if service == nil {
		_ = writeError(c, fiber.StatusInternalServerError, "internal_error", "project authentication is unavailable")
		return false
	}
	secret, ok := bearer(c.Get("Authorization"))
	if !ok {
		_ = writeError(c, fiber.StatusUnauthorized, "unauthorized", "project api key is required")
		return false
	}
	if err := service.Authenticate(ctx, projectID, secret); err != nil {
		if errors.Is(err, access.ErrUnauthorized) {
			_ = writeError(c, fiber.StatusUnauthorized, "unauthorized", "invalid project api key")
			return false
		}
		_ = writeError(c, fiber.StatusInternalServerError, "internal_error", "could not authenticate project")
		return false
	}
	return true
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
