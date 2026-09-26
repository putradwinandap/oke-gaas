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
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

function assertGaasError(code, status) {
  return (error) => {
    assert.ok(error instanceof GaasError);
    assert.equal(error.code, code);
    assert.equal(error.status, status);
    return true;
  };
}

test("malformed successful responses become invalid_response", async () => {
  await withServer((_request, response) => {
    response.writeHead(200, { "content-type": "application/json" });
    response.end("{}");
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret", baseUrl });
    await assert.rejects(
      () => gaas.players.get("player_123"),
      assertGaasError("invalid_response", 200),
    );
  });
});

test("unsafe integer responses are rejected instead of silently losing precision", async () => {
  await withServer((_request, response) => {
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({
      project_id: "proj_test",
      player_id: "player_123",
      xp: Number.MAX_SAFE_INTEGER + 1,
    }));
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret", baseUrl });
    await assert.rejects(
      () => gaas.players.get("player_123"),
      assertGaasError("invalid_response", 200),
    );
  });
});

test("body read failures become network_error instead of leaking a raw transport error", async () => {
  await withServer((_request, response) => {
    response.writeHead(200, {
      "content-type": "application/json",
      "content-length": "1024",
    });
    response.write('{"project_id":"proj_test"');
    setImmediate(() => response.destroy());
  }, async (baseUrl) => {
    const gaas = createGaas({ projectId: "proj_test", apiKey: "secret", baseUrl });
    await assert.rejects(
      () => gaas.players.get("player_123"),
      assertGaasError("network_error", undefined),
    );
  });
});

test("requests have a finite timeout", async () => {
  await withServer(() => {
    // Intentionally never write a response. The client timeout must abort the request.
  }, async (baseUrl) => {
    const gaas = createGaas({
      projectId: "proj_test",
      apiKey: "secret",
      baseUrl,
      timeoutMs: 25,
    });
    await assert.rejects(
      () => gaas.players.get("player_123"),
      assertGaasError("timeout_error", undefined),
    );
  });
});

test("caller cancellation is distinct from timeout", async () => {
  await withServer(() => {
    // Intentionally never write a response. The caller abort must win first.
  }, async (baseUrl) => {
    const gaas = createGaas({
      projectId: "proj_test",
      apiKey: "secret",
      baseUrl,
      timeoutMs: 5_000,
    });
    const controller = new AbortController();
    const pending = gaas.players.get("player_123", { signal: controller.signal });
    setTimeout(() => controller.abort(), 20);
    await assert.rejects(pending, assertGaasError("request_aborted", undefined));
  });
});

test("baseUrl rejects credentials, query strings, and fragments", () => {
  const base = { projectId: "proj_test", apiKey: "secret" };
  assert.throws(
    () => createGaas({ ...base, baseUrl: "https://user:pass@example.com" }),
    /credentials/,
  );
  assert.throws(
    () => createGaas({ ...base, baseUrl: "https://example.com?tenant=x" }),
    /query string/,
  );
  assert.throws(
    () => createGaas({ ...base, baseUrl: "https://example.com#fragment" }),
    /fragment/,
  );
});

test("timeout configuration rejects values that setTimeout cannot safely represent", () => {
  const base = { projectId: "proj_test", apiKey: "secret" };
  assert.throws(() => createGaas({ ...base, timeoutMs: 0 }), /timeoutMs/);
  assert.throws(() => createGaas({ ...base, timeoutMs: 2_147_483_648 }), /timeoutMs/);
  assert.throws(() => createGaas({ ...base, timeoutMs: 1.5 }), /timeoutMs/);
});

test("invalid Date input fails predictably before a request is sent", async () => {
  const gaas = createGaas({ projectId: "proj_test", apiKey: "secret" });
  await assert.rejects(
    () => gaas.track("lesson_completed", {
      playerId: "player_123",
      occurredAt: new Date(Number.NaN),
    }),
    /occurredAt must be a valid Date/,
  );
});

test("rule XP must stay within JavaScript safe-integer precision", async () => {
  const gaas = createGaas({ projectId: "proj_test", apiKey: "secret" });
  await assert.rejects(
    () => gaas.rules.create({ eventType: "lesson_completed", xp: Number.MAX_SAFE_INTEGER + 1 }),
    /positive safe integer/,
  );
});
