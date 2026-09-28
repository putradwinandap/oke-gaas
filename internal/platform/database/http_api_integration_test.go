package database_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/putradwinandap/oke-gaas/internal/access"
	"github.com/putradwinandap/oke-gaas/internal/achievement"
	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/counter"
	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/level"
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

	require.NoError(t, db.Exec("DROP FUNCTION IF EXISTS require_counter_aware_event_processing() CASCADE").Error)
	require.NoError(t, db.Exec("DROP FUNCTION IF EXISTS require_badge_aware_event_processing() CASCADE").Error)
	for _, table := range []string{
		"project_api_keys",
		"event_processing",
		"badge_definitions",
		"player_counters",
		"counter_definitions",
		"rule_daily_claims",
		"rule_match_counts",
		"player_states",
		"level_thresholds",
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
		"../../../migrations/000007_rule_conditions.up.sql",
		"../../../migrations/000008_rule_match_counts.up.sql",
		"../../../migrations/000009_rule_daily_claims.up.sql",
		"../../../migrations/000010_level_thresholds.up.sql",
		"../../../migrations/000011_counters.up.sql",
		"../../../migrations/000012_achievements.up.sql",
		"../../../migrations/000013_badge_rewards.up.sql",
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
	counters := database.NewCounterRepository(db)
	playerCounters := database.NewPlayerCounterRepository(db)
	badges := database.NewBadgeRepository(db)
	badgeGrants := database.NewBadgeGrantRepository(db)
	achievements := achievement.NewService(projects, players, counters, database.NewAchievementRepository(db), database.NewAchievementUnlockRepository(db))
	return httpserver.New(httpserver.Dependencies{
		Projects:     project.NewProvisionService(database.NewProjectProvisionTransactor(db)),
		Players:      player.NewService(projects, players),
		Levels:       level.NewService(projects, database.NewLevelRepository(db)),
		Counters:     counter.NewService(projects, players, counters, playerCounters),
		Badges:       badge.NewService(projects, players, badges, badgeGrants),
		Achievements: achievements,
		Rules:        rule.NewServiceWithBadges(projects, rules, badges),
		Progress:     progression.NewService(database.NewProgressionTransactor(db)),
		States:       database.NewPlayerStateRepository(db),
		Access:       access.NewService(database.NewProjectAPIKeyRepository(db)),
		AdminKey:     "test-admin-key",
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
	require.Equal(t, float64(1), firstState["level"])

	resp, duplicate := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, eventBody,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, duplicate["duplicate"])
	duplicateState := duplicate["state"].(map[string]any)
	require.Equal(t, float64(100), duplicateState["xp"])
	require.Equal(t, float64(1), duplicateState["level"])

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
	require.Equal(t, float64(1), state["level"])
}

func TestRESTCountersAreProjectScopedAndTrackAcceptedEvents(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	projectID, apiKey := createProjectViaAPI(t, app, "Counters")
	otherProjectID, otherAPIKey := createProjectViaAPI(t, app, "Other Counters")
	resp, playerBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey,
		map[string]any{"external_id": "student-counter"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)

	resp, created := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/counters", projectID), apiKey,
		map[string]any{
			"name": "completed lessons", "event_type": "lesson_completed",
			"conditions": map[string]any{"course_id": "course_7"},
		},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	counterID := created["id"].(string)

	resp, listed := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/counters", projectID), apiKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, listed["counters"], 1)
	resp, otherCounters := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/counters", otherProjectID), otherAPIKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, otherCounters["counters"])

	resp, initial := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/players/%s/counters", projectID, playerID), apiKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	initialProgress := initial["counters"].([]any)[0].(map[string]any)
	require.Equal(t, float64(0), initialProgress["value"])

	eventBody := map[string]any{
		"event_id": "evt_counter_api", "player_id": playerID, "type": "lesson_completed",
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
		"properties":  map[string]any{"course_id": "course_7"},
	}
	resp, _ = requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, eventBody,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp, _ = requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, eventBody,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, progress := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/players/%s/counters", projectID, playerID), apiKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	progressEntry := progress["counters"].([]any)[0].(map[string]any)
	require.Equal(t, counterID, progressEntry["counter_id"])
	require.Equal(t, float64(1), progressEntry["value"])

	resp, crossProject := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/players/%s/counters", otherProjectID, playerID), otherAPIKey, nil,
	)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, "player_not_found", crossProject["error"].(map[string]any)["code"])
}

func TestRESTBadgeRewardIsAuditableProjectScopedAndUniquePerPlayer(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)
	projectID, apiKey := createProjectViaAPI(t, app, "Badges")
	otherProjectID, otherAPIKey := createProjectViaAPI(t, app, "Other Badges")
	resp, playerBody := requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey, map[string]any{"external_id": "student-badge"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)
	resp, createdBadge := requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/badges", projectID), apiKey, map[string]any{"name": "Early Adopter", "description": "First cohort"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	badgeID := createdBadge["id"].(string)
	resp, listed := requestJSON(t, app, http.MethodGet, fmt.Sprintf("/v1/projects/%s/badges", projectID), apiKey, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, listed["badges"], 1)
	resp, _ = requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/rules", otherProjectID), otherAPIKey, map[string]any{"reward_type": "badge", "badge_id": badgeID, "event_type": "lesson_completed"})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp, _ = requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey, map[string]any{"reward_type": "badge", "badge_id": badgeID, "event_type": "lesson_completed", "conditions": map[string]any{"course": "go"}})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	badgeEventTime := time.Now().UTC().Format(time.RFC3339Nano)
	postEvent := func(id, course string) (*http.Response, map[string]any) {
		return requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, map[string]any{"event_id": id, "player_id": playerID, "type": "lesson_completed", "occurred_at": badgeEventTime, "properties": map[string]any{"course": course}})
	}
	resp, nonmatch := postEvent("evt_badge_no_match", "rust")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Empty(t, nonmatch["grants"])
	resp, first := postEvent("evt_badge_1", "go")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	grants := first["grants"].([]any)
	require.Len(t, grants, 1)
	grant := grants[0].(map[string]any)
	require.Equal(t, "badge", grant["reward_type"])
	require.Equal(t, badgeID, grant["badge_id"])
	require.Equal(t, float64(0), grant["amount"])
	stateResp, state := requestJSON(t, app, http.MethodGet, fmt.Sprintf("/v1/projects/%s/players/%s/state", projectID, playerID), apiKey, nil)
	require.Equal(t, http.StatusOK, stateResp.StatusCode)
	require.Zero(t, state["xp"])
	resp, retry := postEvent("evt_badge_1", "go")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, retry["duplicate"])
	resp, second := postEvent("evt_badge_2", "go")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Empty(t, second["grants"])
	resp, collection := requestJSON(t, app, http.MethodGet, fmt.Sprintf("/v1/projects/%s/players/%s/badges", projectID, playerID), apiKey, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, collection["badges"], 1)
	owned := collection["badges"].([]any)[0].(map[string]any)
	require.Equal(t, badgeID, owned["badge_id"])
	require.Equal(t, "evt_badge_1", owned["event_id"])
	require.Equal(t, float64(1), owned["rule_version"])
	resp, isolated := requestJSON(t, app, http.MethodGet, fmt.Sprintf("/v1/projects/%s/players/%s/badges", otherProjectID, playerID), otherAPIKey, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NotEmpty(t, isolated["error"])
}

func TestConcurrentBadgeEventsCreateOnePlayerOwnershipGrant(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)
	projectID, apiKey := createProjectViaAPI(t, app, "Concurrent Badges")
	resp, playerBody := requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey, map[string]any{"external_id": "student-concurrent-badge"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)
	resp, badgeBody := requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/badges", projectID), apiKey, map[string]any{"name": "Concurrent Badge"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	_, _ = requestJSON(t, app, http.MethodPost, fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey, map[string]any{"reward_type": "badge", "badge_id": badgeBody["id"], "event_type": "lesson_completed"})

	service := progression.NewService(database.NewProgressionTransactor(db))
	const workers = 8
	results := make(chan int, workers)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			result, err := service.Process(context.Background(), eventdomain.IngestCommand{ID: fmt.Sprintf("evt_concurrent_badge_%d", index), ProjectID: projectID, PlayerID: playerID, Type: "lesson_completed", OccurredAt: time.Now().UTC(), Properties: map[string]any{}})
			if err != nil {
				errors <- err
				return
			}
			results <- len(result.Grants)
		}(i)
	}
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	grantCount := 0
	for count := range results {
		grantCount += count
	}
	require.Equal(t, 1, grantCount)
	resp, collection := requestJSON(t, app, http.MethodGet, fmt.Sprintf("/v1/projects/%s/players/%s/badges", projectID, playerID), apiKey, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, collection["badges"], 1)
	var auditRows []struct {
		EventID     string
		RuleID      string
		RuleVersion uint64
		RewardType  string
		BadgeID     string
	}
	require.NoError(t, db.Table("reward_grants").Select("event_id, rule_id, rule_version, reward_type, badge_id").Where("project_id = ? AND player_id = ? AND reward_type = 'badge'", projectID, playerID).Scan(&auditRows).Error)
	require.Len(t, auditRows, 1)
	require.Regexp(t, `^evt_concurrent_badge_[0-7]$`, auditRows[0].EventID)
	require.NotEmpty(t, auditRows[0].RuleID)
	require.Equal(t, uint64(1), auditRows[0].RuleVersion)
	require.Equal(t, "badge", auditRows[0].RewardType)
	require.Equal(t, badgeBody["id"], auditRows[0].BadgeID)
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

	var count int64
	require.NoError(t, db.Table("players").
		Where("project_id = ?", secondProjectID).
		Count(&count).Error)
	require.Zero(t, count)
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

func TestRESTRejectsOversizedPublicInputBeforePersistence(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	resp, body := requestJSON(t, app, http.MethodPost, "/v1/projects", "test-admin-key", map[string]any{
		"name": strings.Repeat("界", 256),
	})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorBody := body["error"].(map[string]any)
	require.Equal(t, "invalid_project_name", errorBody["code"])

	var count int64
	require.NoError(t, db.Table("projects").Count(&count).Error)
	require.Zero(t, count)
}

func TestRESTConditionalRuleRewardsOnlyMatchingProperties(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	projectID, apiKey := createProjectViaAPI(t, app, "Conditional Learning")

	resp, playerBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey,
		map[string]any{"external_id": "student-conditional"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)

	resp, ruleBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{
			"event_type": "lesson_completed",
			"xp":         100,
			"conditions": map[string]any{
				"course_id":  "course_7",
				"difficulty": 2,
			},
		},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, map[string]any{
		"course_id":  "course_7",
		"difficulty": float64(2),
	}, ruleBody["conditions"])
	require.Equal(t, float64(1), ruleBody["match_every"])

	baseEvent := map[string]any{
		"player_id":   playerID,
		"type":        "lesson_completed",
		"occurred_at": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
	}

	nonMatching := make(map[string]any, len(baseEvent)+2)
	for key, value := range baseEvent {
		nonMatching[key] = value
	}
	nonMatching["event_id"] = "evt_conditional_miss"
	nonMatching["properties"] = map[string]any{
		"course_id":  "course_7",
		"difficulty": 1,
	}
	resp, missBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, nonMatching,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Empty(t, missBody["grants"].([]any))
	require.Equal(t, float64(0), missBody["state"].(map[string]any)["xp"])

	matching := make(map[string]any, len(baseEvent)+2)
	for key, value := range baseEvent {
		matching[key] = value
	}
	matching["event_id"] = "evt_conditional_match"
	matching["properties"] = map[string]any{
		"course_id":  "course_7",
		"difficulty": 2.0,
		"extra":      "ignored",
	}
	resp, matchBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, matching,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Len(t, matchBody["grants"].([]any), 1)
	require.Equal(t, float64(100), matchBody["state"].(map[string]any)["xp"])
}

func TestRESTAggregateRuleRewardsEveryNthMatchingEvent(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	projectID, apiKey := createProjectViaAPI(t, app, "Aggregate Learning")
	resp, playerBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey,
		map[string]any{"external_id": "student-aggregate"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)

	resp, invalidRule := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{
			"event_type":  "lesson_completed",
			"xp":          250,
			"match_every": 0,
		},
	)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "invalid_rule", invalidRule["error"].(map[string]any)["code"])

	resp, ruleBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{
			"event_type":  "lesson_completed",
			"xp":          250,
			"match_every": 2,
			"conditions":  map[string]any{"course_id": "course_7"},
		},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, float64(2), ruleBody["match_every"])
	ruleID := ruleBody["id"].(string)

	occurredAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	send := func(eventID, courseID string) (*http.Response, map[string]any) {
		return requestJSON(t, app, http.MethodPost,
			fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey,
			map[string]any{
				"event_id":    eventID,
				"player_id":   playerID,
				"type":        "lesson_completed",
				"occurred_at": occurredAt,
				"properties":  map[string]any{"course_id": courseID},
			},
		)
	}

	resp, miss := send("evt_aggregate_miss", "course_8")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Empty(t, miss["grants"].([]any))

	resp, first := send("evt_aggregate_1", "course_7")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Empty(t, first["grants"].([]any))
	require.Equal(t, float64(0), first["state"].(map[string]any)["xp"])

	resp, second := send("evt_aggregate_2", "course_7")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Len(t, second["grants"].([]any), 1)
	require.Equal(t, float64(250), second["state"].(map[string]any)["xp"])

	resp, retry := send("evt_aggregate_2", "course_7")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, retry["duplicate"])
	require.Len(t, retry["grants"].([]any), 1)
	require.Equal(t, float64(250), retry["state"].(map[string]any)["xp"])

	var count uint64
	require.NoError(t, db.Table("rule_match_counts").
		Select("match_count").
		Where("project_id = ? AND player_id = ? AND rule_id = ? AND rule_version = ?", projectID, playerID, ruleID, 1).
		Scan(&count).Error)
	require.Equal(t, uint64(2), count)
}

func TestRESTDailyRuleRewardsAtMostOncePerUTCDay(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	projectID, apiKey := createProjectViaAPI(t, app, "Daily Learning")
	resp, playerBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey,
		map[string]any{"external_id": "student-daily"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)

	resp, invalid := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{
			"event_type":       "daily_login",
			"xp":               25,
			"match_every":      2,
			"once_per_utc_day": true,
		},
	)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "invalid_rule", invalid["error"].(map[string]any)["code"])

	resp, ruleBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{
			"event_type":       "daily_login",
			"xp":               25,
			"once_per_utc_day": true,
		},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, true, ruleBody["once_per_utc_day"])
	require.Equal(t, float64(1), ruleBody["match_every"])

	send := func(eventID string, occurredAt time.Time) (*http.Response, map[string]any) {
		return requestJSON(t, app, http.MethodPost,
			fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey,
			map[string]any{
				"event_id":    eventID,
				"player_id":   playerID,
				"type":        "daily_login",
				"occurred_at": occurredAt.Format(time.RFC3339Nano),
			},
		)
	}

	resp, first := send("evt_daily_api_first", time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Len(t, first["grants"].([]any), 1)
	require.Equal(t, float64(25), first["state"].(map[string]any)["xp"])

	resp, sameDay := send("evt_daily_api_second", time.Date(2026, 9, 27, 23, 59, 0, 0, time.UTC))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Empty(t, sameDay["grants"].([]any))
	require.Equal(t, float64(25), sameDay["state"].(map[string]any)["xp"])

	resp, nextDay := send("evt_daily_api_next", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Len(t, nextDay["grants"].([]any), 1)
	require.Equal(t, float64(50), nextDay["state"].(map[string]any)["xp"])
}

func TestRESTXPLevelsAreProjectScopedAndDerivedFromXP(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	app := newHTTPIntegrationApp(db)

	projectID, apiKey := createProjectViaAPI(t, app, "Leveled Learning")
	otherProjectID, otherKey := createProjectViaAPI(t, app, "Other Learning")

	resp, created := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/levels", projectID), apiKey,
		map[string]any{"min_xp": 100},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, float64(2), created["number"])
	require.Equal(t, float64(100), created["min_xp"])

	resp, created = requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/levels", projectID), apiKey,
		map[string]any{"min_xp": 250},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, float64(3), created["number"])

	resp, invalid := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/levels", projectID), apiKey,
		map[string]any{"min_xp": 200},
	)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "invalid_level", invalid["error"].(map[string]any)["code"])

	resp, listed := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/levels", projectID), apiKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, []any{
		map[string]any{"number": float64(1), "min_xp": float64(0)},
		map[string]any{"number": float64(2), "min_xp": float64(100)},
		map[string]any{"number": float64(3), "min_xp": float64(250)},
	}, listed["levels"])

	resp, otherLevels := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/levels", otherProjectID), otherKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, []any{
		map[string]any{"number": float64(1), "min_xp": float64(0)},
	}, otherLevels["levels"])

	resp, _ = requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/levels", projectID), otherKey, nil,
	)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, playerBody := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/players", projectID), apiKey,
		map[string]any{"external_id": "level-player"},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	playerID := playerBody["id"].(string)

	resp, _ = requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/rules", projectID), apiKey,
		map[string]any{"event_type": "lesson_completed", "xp": 100},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	event := map[string]any{
		"event_id":    "evt_level_1",
		"player_id":   playerID,
		"type":        "lesson_completed",
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	resp, first := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, event,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, float64(100), first["state"].(map[string]any)["xp"])
	require.Equal(t, float64(2), first["state"].(map[string]any)["level"])

	resp, retry := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, event,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, retry["duplicate"])
	require.Equal(t, float64(100), retry["state"].(map[string]any)["xp"])
	require.Equal(t, float64(2), retry["state"].(map[string]any)["level"])

	event["event_id"] = "evt_level_2"
	resp, second := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, event,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, float64(200), second["state"].(map[string]any)["xp"])
	require.Equal(t, float64(2), second["state"].(map[string]any)["level"])

	event["event_id"] = "evt_level_3"
	resp, third := requestJSON(t, app, http.MethodPost,
		fmt.Sprintf("/v1/projects/%s/events", projectID), apiKey, event,
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, float64(300), third["state"].(map[string]any)["xp"])
	require.Equal(t, float64(3), third["state"].(map[string]any)["level"])

	resp, state := requestJSON(t, app, http.MethodGet,
		fmt.Sprintf("/v1/projects/%s/players/%s/state", projectID, playerID), apiKey, nil,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, float64(3), state["level"])
}
