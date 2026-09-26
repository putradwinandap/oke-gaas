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
      state: { player_id: "player_123", xp: 100 },
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
      }));
      return;
    }
    if (request.url === "/v1/projects/proj_test/players/player_123/state" && request.method === "GET") {
      response.writeHead(200);
      response.end(JSON.stringify({
        project_id: "proj_test",
        player_id: "player_123",
        xp: 100,
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
    assert.deepEqual(await gaas.rules.create({ eventType: "lesson_completed", xp: 100 }), {
      id: "rule_1",
      projectId: "proj_test",
      version: 1,
      eventType: "lesson_completed",
      xp: 100,
    });
    assert.deepEqual(await gaas.players.get("player_123"), {
      projectId: "proj_test",
      playerId: "player_123",
      xp: 100,
      updatedAt: "2026-09-26T10:00:00Z",
    });
  });

  assert.deepEqual(requests.map(({ method, url }) => ({ method, url })), [
    { method: "POST", url: "/v1/projects/proj_test/players" },
    { method: "POST", url: "/v1/projects/proj_test/rules" },
    { method: "GET", url: "/v1/projects/proj_test/players/player_123/state" },
  ]);
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
