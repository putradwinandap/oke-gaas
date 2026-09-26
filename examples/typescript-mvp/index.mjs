import { randomUUID } from "node:crypto";
import { createGaas, GaasError } from "../../sdk/typescript/dist/index.js";

const baseUrl = process.env.OKE_GAAS_BASE_URL ?? "http://localhost:8080";
const projectId = requiredEnv("OKE_GAAS_PROJECT_ID");
const apiKey = requiredEnv("OKE_GAAS_API_KEY");

const gaas = createGaas({ baseUrl, projectId, apiKey });
const runId = randomUUID();
const eventType = `example.lesson_completed.${runId}`;

try {
  const player = await gaas.players.create({
    externalId: `example-player-${runId}`,
  });

  await gaas.rules.create({
    eventType,
    xp: 100,
  });

  const processed = await gaas.track(eventType, {
    playerId: player.id,
    eventId: `example-event-${runId}`,
    properties: {
      lessonId: "lesson_5",
    },
  });

  const state = await gaas.players.get(player.id);

  console.log(JSON.stringify({ player, processed, state }, null, 2));
} catch (error) {
  if (error instanceof GaasError) {
    console.error(`Oke Gaas error [${error.code}]${error.status ? ` HTTP ${error.status}` : ""}: ${error.message}`);
    process.exitCode = 1;
  } else {
    throw error;
  }
}

function requiredEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}
