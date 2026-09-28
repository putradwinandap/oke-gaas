import assert from "node:assert/strict";
import http from "node:http";
import test from "node:test";
import { createGaas, GaasError } from "../dist/index.js";

async function withServer(handler, run) {
  const server = http.createServer(handler);
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.equal(typeof address, "object");
  assert.ok(address);
  try {
    await run(`http://127.0.0.1:${address.port}`);
  } finally {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

async function readJson(request) {
  const chunks = [];
  for await (const chunk of request) {
    chunks.push(chunk);
  }
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

test("track authenticates, generates event identity, and maps the result", async () => {
  await withServer(async (request, response) => {
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/v1/projects/proj_test/events");
    assert.equal(request.headers.authorization, "Bearer secret-key");
    assert.equal(request.headers["content-type"], "application/json");

    const body = await readJson(request);
    assert.match(body.event_id, /^evt_[0-9a-f-]{36}$/i);
    assert.equal(body.player_id, "player_123");
    assert.equal(body.type, "lesson_completed");
    assert.equal(typeof body.occurred_at, "string");
    assert.deepEqual(body.properties, { lessonId: "lesson_5" });

    response.writeHead(201, { "content-type": "application/json" });
    response.end(JSON.stringify({
      event_id: body.event_id,
      duplicate: false,
      grants: [{
        id: "grant_1",
        rule_id: "rule_1",
        rule_version: 1,
        reward_type: "xp",
        amount: 100,
      }],
      state: {
        player_id: "player_123",
        xp: 100,
        level: 2,
        updated_at: "2026-09-26T10:00:00Z",
      },
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    const result = await gaas.track("lesson_completed", {
      playerId: "player_123",
      properties: { lessonId: "lesson_5" },
    });

    assert.match(result.eventId, /^evt_/);
    assert.equal(result.duplicate, false);
    assert.deepEqual(result.grants, [{
      id: "grant_1",
      ruleId: "rule_1",
      ruleVersion: 1,
      rewardType: "xp",
      amount: 100,
    }]);
    assert.deepEqual(result.state, {
      playerId: "player_123",
      xp: 100,
      level: 2,
      updatedAt: "2026-09-26T10:00:00Z",
    });
  });
});

test("track preserves caller-supplied event identity and timestamp", async () => {
  await withServer(async (request, response) => {
    const body = await readJson(request);
    assert.equal(body.event_id, "checkout-42");
    assert.equal(body.occurred_at, "2026-09-26T12:34:56.000Z");
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({
      event_id: body.event_id,
      duplicate: true,
      grants: [],
      state: { player_id: "player_123", xp: 100, level: 2 },
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    const result = await gaas.track("purchase_completed", {
      playerId: "player_123",
      eventId: "checkout-42",
      occurredAt: new Date("2026-09-26T12:34:56Z"),
    });
    assert.equal(result.eventId, "checkout-42");
    assert.equal(result.duplicate, true);
  });
});

test("players and rules wrap the project-scoped REST endpoints", async () => {
  const requests = [];
  await withServer(async (request, response) => {
    requests.push({ method: request.method, url: request.url, body: request.method === "POST" ? await readJson(request) : undefined });
    response.setHeader("content-type", "application/json");

    if (request.url === "/v1/projects/proj_test/players" && request.method === "POST") {
      response.writeHead(201);
      response.end(JSON.stringify({
        id: "player_123",
        project_id: "proj_test",
        external_id: "customer-9",
        created_at: "2026-09-26T10:00:00Z",
      }));
      return;
    }
    if (request.url === "/v1/projects/proj_test/rules" && request.method === "POST") {
      response.writeHead(201);
      response.end(JSON.stringify({
        id: "rule_1",
        project_id: "proj_test",
        version: 1,
        event_type: "lesson_completed",
        xp: 100,
        conditions: { difficulty: "hard" },
        match_every: 3,
        once_per_utc_day: false,
      }));
      return;
    }
    if (request.url === "/v1/projects/proj_test/players/player_123/state" && request.method === "GET") {
      response.writeHead(200);
      response.end(JSON.stringify({
        project_id: "proj_test",
        player_id: "player_123",
        xp: 100,
        level: 2,
        updated_at: "2026-09-26T10:00:00Z",
      }));
      return;
    }
    response.writeHead(404);
    response.end(JSON.stringify({ error: { code: "not_found", message: "not found" } }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });

    assert.deepEqual(await gaas.players.create({ externalId: "customer-9" }), {
      id: "player_123",
      projectId: "proj_test",
      externalId: "customer-9",
      createdAt: "2026-09-26T10:00:00Z",
    });
    assert.deepEqual(await gaas.rules.create({
      eventType: "lesson_completed",
      xp: 100,
      conditions: { difficulty: "hard" },
      matchEvery: 3,
    }), {
      id: "rule_1",
      projectId: "proj_test",
      version: 1,
      eventType: "lesson_completed",
      xp: 100,
      conditions: { difficulty: "hard" },
      matchEvery: 3,
      oncePerUtcDay: false,
    });
    assert.deepEqual(await gaas.players.get("player_123"), {
      projectId: "proj_test",
      playerId: "player_123",
      xp: 100,
      level: 2,
      updatedAt: "2026-09-26T10:00:00Z",
    });
  });

  assert.deepEqual(requests[1].body, {
    event_type: "lesson_completed",
    xp: 100,
    conditions: { difficulty: "hard" },
    match_every: 3,
    once_per_utc_day: false,
  });

  assert.deepEqual(requests.map(({ method, url }) => ({ method, url })), [
    { method: "POST", url: "/v1/projects/proj_test/players" },
    { method: "POST", url: "/v1/projects/proj_test/rules" },
    { method: "GET", url: "/v1/projects/proj_test/players/player_123/state" },
  ]);
});

test("rules default matchEvery to one and reject invalid thresholds", async () => {
  await withServer(async (request, response) => {
    const body = await readJson(request);
    assert.equal(body.match_every, 1);
    response.writeHead(201, { "content-type": "application/json" });
    response.end(JSON.stringify({
      id: "rule_default",
      project_id: "proj_test",
      version: 1,
      event_type: "daily_login",
      xp: 10,
      conditions: {},
      match_every: 1,
      once_per_utc_day: false,
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    const result = await gaas.rules.create({ eventType: "daily_login", xp: 10 });
    assert.equal(result.matchEvery, 1);
    assert.equal(result.oncePerUtcDay, false);
    await assert.rejects(
      () => gaas.rules.create({ eventType: "daily_login", xp: 10, matchEvery: 0 }),
      /matchEvery/,
    );
  });
});

test("rules support once-per-UTC-day gating and leave composition validation to the server", async () => {
  let calls = 0;
  await withServer(async (request, response) => {
    calls += 1;
    const body = await readJson(request);
    assert.equal(body.once_per_utc_day, true);

    if (calls === 1) {
      assert.equal(body.match_every, 1);
      response.writeHead(201, { "content-type": "application/json" });
      response.end(JSON.stringify({
        id: "rule_daily",
        project_id: "proj_test",
        version: 1,
        event_type: "daily_login",
        xp: 25,
        conditions: {},
        match_every: 1,
        once_per_utc_day: true,
      }));
      return;
    }

    assert.equal(body.match_every, 2);
    response.writeHead(400, { "content-type": "application/json" });
    response.end(JSON.stringify({
      error: {
        code: "invalid_rule",
        message: "once_per_utc_day requires match_every=1",
      },
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    const result = await gaas.rules.create({
      eventType: "daily_login",
      xp: 25,
      oncePerUtcDay: true,
    });
    assert.equal(result.oncePerUtcDay, true);

    await assert.rejects(
      () => gaas.rules.create({
        eventType: "daily_login",
        xp: 25,
        matchEvery: 2,
        oncePerUtcDay: true,
      }),
      (error) => {
        assert.ok(error instanceof GaasError);
        assert.equal(error.code, "invalid_rule");
        assert.equal(error.status, 400);
        return true;
      },
    );
  });
});

test("API error envelopes become GaasError without exposing unrelated response data", async () => {
  await withServer((_request, response) => {
    response.writeHead(401, { "content-type": "application/json" });
    response.end(JSON.stringify({
      error: { code: "unauthorized", message: "invalid project api key" },
      internal: "must-not-leak-through-error-message",
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "wrong", baseUrl });
    await assert.rejects(
      () => gaas.players.get("player_123"),
      (error) => {
        assert.ok(error instanceof GaasError);
        assert.equal(error.code, "unauthorized");
        assert.equal(error.status, 401);
        assert.equal(error.message, "invalid project api key");
        return true;
      },
    );
  });
});

test("client configuration rejects empty credentials and unsafe URL schemes", () => {
  assert.throws(() => createGaas({ projectId: "", apiKey: "secret" }), /projectId/);
  assert.throws(() => createGaas({ projectId: "proj_test", apiKey: "" }), /apiKey/);
  assert.throws(
    () => createGaas({ projectId: "proj_test", apiKey: "secret", baseUrl: "file:///tmp/gaas" }),
    /http or https/,
  );
});


test("levels append immutable thresholds and list the implicit level one", async () => {
  let call = 0;
  await withServer(async (request, response) => {
    call += 1;
    assert.equal(request.headers.authorization, "Bearer secret-key");
    response.setHeader("content-type", "application/json");

    if (call === 1) {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/v1/projects/proj_test/levels");
      assert.deepEqual(await readJson(request), { min_xp: 100 });
      response.writeHead(201);
      response.end(JSON.stringify({ project_id: "proj_test", number: 2, min_xp: 100 }));
      return;
    }

    assert.equal(request.method, "GET");
    assert.equal(request.url, "/v1/projects/proj_test/levels");
    response.writeHead(200);
    response.end(JSON.stringify({
      project_id: "proj_test",
      levels: [
        { number: 1, min_xp: 0 },
        { number: 2, min_xp: 100 },
      ],
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    assert.deepEqual(await gaas.levels.append(100), { number: 2, minXp: 100 });
    assert.deepEqual(await gaas.levels.list(), [
      { number: 1, minXp: 0 },
      { number: 2, minXp: 100 },
    ]);
  });
});

test("counters create, list, and retrieve Player progress", async () => {
  let call = 0;
  await withServer(async (request, response) => {
    call += 1;
    response.setHeader("content-type", "application/json");
    if (call === 1) {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/v1/projects/proj_test/counters");
      assert.deepEqual(await readJson(request), {
        name: "lessons_completed",
        event_type: "lesson_completed",
        conditions: { course_id: "course_7" },
      });
      response.writeHead(201);
      response.end(JSON.stringify({
        id: "counter_123",
        project_id: "proj_test",
        name: "lessons_completed",
        event_type: "lesson_completed",
        conditions: { course_id: "course_7" },
        created_at: "2026-09-28T10:00:00Z",
      }));
      return;
    }
    if (call === 2) {
      assert.equal(request.method, "GET");
      assert.equal(request.url, "/v1/projects/proj_test/counters");
      response.writeHead(200);
      response.end(JSON.stringify({
        project_id: "proj_test",
        counters: [{
          id: "counter_123",
          project_id: "proj_test",
          name: "lessons_completed",
          event_type: "lesson_completed",
          conditions: { course_id: "course_7" },
          created_at: "2026-09-28T10:00:00Z",
        }],
      }));
      return;
    }

    assert.equal(request.method, "GET");
    assert.equal(request.url, "/v1/projects/proj_test/players/player_123/counters");
    response.writeHead(200);
    response.end(JSON.stringify({
      project_id: "proj_test",
      player_id: "player_123",
      counters: [{
        counter_id: "counter_123",
        name: "lessons_completed",
        event_type: "lesson_completed",
        conditions: { course_id: "course_7" },
        value: 3,
        updated_at: "2026-09-28T10:00:00Z",
      }],
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    const created = await gaas.counters.create({
      name: "lessons_completed",
      eventType: "lesson_completed",
      conditions: { course_id: "course_7" },
    });
    assert.deepEqual(created, {
      id: "counter_123",
      projectId: "proj_test",
      name: "lessons_completed",
      eventType: "lesson_completed",
      conditions: { course_id: "course_7" },
      createdAt: "2026-09-28T10:00:00Z",
    });
    assert.deepEqual(await gaas.counters.list(), [created]);
    assert.deepEqual(await gaas.players.getCounters("player_123"), [{
      counterId: "counter_123",
      name: "lessons_completed",
      eventType: "lesson_completed",
      conditions: { course_id: "course_7" },
      value: 3,
      updatedAt: "2026-09-28T10:00:00Z",
    }]);
  });
});

test("Player Counter progress rejects unsafe integer responses", async () => {
  await withServer((_request, response) => {
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({
      project_id: "proj_test",
      player_id: "player_123",
      counters: [{
        counter_id: "counter_123",
        name: "lessons_completed",
        event_type: "lesson_completed",
        conditions: {},
        value: Number.MAX_SAFE_INTEGER + 1,
      }],
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret-key", baseUrl });
    await assert.rejects(
      () => gaas.players.getCounters("player_123"),
      (error) => error instanceof GaasError && error.code === "invalid_response",
    );
  });
});
