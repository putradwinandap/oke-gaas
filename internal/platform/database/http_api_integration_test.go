package database_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/putradwinandap/oke-gaas/internal/access"
	database "github.com/putradwinandap/oke-gaas/internal/platform/database"
	httpserver "github.com/putradwinandap/oke-gaas/internal/platform/http"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openHTTPIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	db, err := database.OpenPostgres(dsn)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, table := range []string{
		"project_api_keys",
		"event_processing",
		"player_states",
		"reward_grants",
		"rules",
		"events",
		"players",
		"projects",
	} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	for _, path := range []string{
		"../../../migrations/000001_core.up.sql",
		"../../../migrations/000002_events.up.sql",
		"../../../migrations/000003_rules_rewards.up.sql",
		"../../../migrations/000004_player_state.up.sql",
		"../../../migrations/000005_event_processing.up.sql",
		"../../../migrations/000006_project_api_keys.up.sql",
	} {
		sql, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(sql)).Error)
	}

	return db
}

func newHTTPIntegrationApp(db *gorm.DB) *fiber.App {
	projects := database.NewProjectRepository(db)
	players := database.NewPlayerRepository(db)
	rules := database.NewRuleRepository(db)
	return httpserver.New(httpserver.Dependencies{
		Projects: project.NewProvisionService(database.NewProjectProvisionTransactor(db)),
		Players:  player.NewService(projects, players),
		Rules:    rule.NewService(projects, rules),
		Progress: progression.NewService(database.NewProgressionTransactor(db)),
		States:   database.NewPlayerStateRepository(db),
		Access:   access.NewService(database.NewProjectAPIKeyRepository(db)),
		AdminKey: "test-admin-key",
	})
}

func requestJSON(t *testing.T, app *fiber.App, method, path, token string, body any) (*http.Response, map[string]any) {
	t.Helper()

	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	require.NoError(t, resp.Body.Close())
	return resp, decoded
}

func createProjectViaAPI(t *testing.T, app *fiber.App, name string) (string, string) {
	t.Helper()

	resp, body := requestJSON(t, app, http.MethodPost, "/v1/projects", "test-admin-key", map[string]any{
		"name": name,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	projectBody, ok := body["project"].(map[string]any)
	require.True(t, ok)
	projectID, ok := projectBody["id"].(string)
	require.True(t, ok)
	apiKey, ok := body["api_key"].(string)
	require.True(t, ok)
	require.NotEmpty(t, apiKey)
	return projectID, apiKey
}

func TestRESTVerticalSliceProcessesDuplicateEventExactlyOnce(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	projectID, apiKey := createProjectViaAPI(t, app, "Learning")

	resp, playerBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey,
		map[string]any{"external_id": "student-1"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID, ok := playerBody["id"].(string)
	require.True(t, ok)

	resp, _ = requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{"event_type": "lesson_completed", "xp": 100},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	eventBody := map[string]any{
		"event_id":    "evt_lesson_1",
		"player_id":   playerID,
		"type":        "lesson_completed",
		"occurred_at": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
		"properties":  map[string]any{"lesson_id": "lesson_5"},
	}
	resp, first := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, eventBody,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, false, first["duplicate"])
	firstState := first["state"].(map[string]any)
	require.Equal(t, float64(100), firstState["xp"])

	resp, duplicate := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, eventBody,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, duplicate["duplicate"])
	duplicateState := duplicate["state"].(map[string]any)
	require.Equal(t, float64(100), duplicateState["xp"])

	conflictBody := map[string]any{
		"event_id":    "evt_lesson_1",
		"player_id":   playerID,
		"type":        "lesson_completed",
		"occurred_at": eventBody["occurred_at"],
		"properties":  map[string]any{"lesson_id": "lesson_6"},
	}
	resp, conflict := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, conflictBody,
	)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	conflictError := conflict["error"].(map[string]any)
	require.Equal(t, "event_identity_conflict", conflictError["code"])

	resp, state := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/players/%s/state", projectID, playerID), apiKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, float64(100), state["xp"])
}

func TestProjectAPIKeyCannotCrossTenantBoundary(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	_, firstKey := createProjectViaAPI(t, app, "First")
	secondProjectID, _ := createProjectViaAPI(t, app, "Second")

	resp, body := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", secondProjectID), firstKey,
		map[string]any{"external_id": "intruder"},
	)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	errorBody := body["error"].(map[string]any)
	require.Equal(t, "unauthorized", errorBody["code"])
}


func TestProjectCannotUseAnotherProjectsPlayer(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	firstProjectID, firstKey := createProjectViaAPI(t, app, "First")
	resp, firstPlayer := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", firstProjectID), firstKey,
		map[string]any{"external_id": "student-a"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	firstPlayerID := firstPlayer["id"].(string)

	secondProjectID, secondKey := createProjectViaAPI(t, app, "Second")
	resp, body := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", secondProjectID), secondKey,
		map[string]any{
			"event_id":    "evt_cross_tenant",
			"player_id":   firstPlayerID,
			"type":        "lesson_completed",
			"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	errorBody := body["error"].(map[string]any)
	require.Equal(t, "player_not_found", errorBody["code"])
}
