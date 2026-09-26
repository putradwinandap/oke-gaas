const DEFAULT_BASE_URL = "http://localhost:8080";
const DEFAULT_TIMEOUT_MS = 15_000;
const MAX_TIMEOUT_MS = 2_147_483_647;

export interface GaasConfig {
  projectId: string;
  apiKey: string;
  baseUrl?: string;
  timeoutMs?: number;
}

export interface RequestOptions {
  signal?: AbortSignal;
}

export interface CreatePlayerInput {
  externalId: string;
}

export interface Player {
  id: string;
  projectId: string;
  externalId: string;
  createdAt: string;
}

export interface CreateRuleInput {
  eventType: string;
  xp: number;
}

export interface Rule {
  id: string;
  projectId: string;
  version: number;
  eventType: string;
  xp: number;
}

export interface TrackInput {
  playerId: string;
  properties?: Record<string, unknown>;
  eventId?: string;
  occurredAt?: Date | string;
}

export interface RewardGrant {
  id: string;
  ruleId: string;
  ruleVersion: number;
  rewardType: string;
  amount: number;
}

export interface TrackPlayerState {
  playerId: string;
  xp: number;
  updatedAt?: string;
}

export interface TrackResult {
  eventId: string;
  duplicate: boolean;
  grants: RewardGrant[];
  state: TrackPlayerState;
}

export interface PlayerState {
  projectId: string;
  playerId: string;
  xp: number;
  updatedAt?: string;
}

interface ApiErrorEnvelope {
  error: {
    code: string;
    message: string;
  };
}

interface ApiPlayer {
  id: string;
  project_id: string;
  external_id: string;
  created_at: string;
}

interface ApiRule {
  id: string;
  project_id: string;
  version: number;
  event_type: string;
  xp: number;
}

interface ApiRewardGrant {
  id: string;
  rule_id: string;
  rule_version: number;
  reward_type: string;
  amount: number;
}

interface ApiTrackPlayerState {
  player_id: string;
  xp: number;
  updated_at?: string;
}

interface ApiTrackResult {
  event_id: string;
  duplicate: boolean;
  grants: ApiRewardGrant[];
  state: ApiTrackPlayerState;
}

interface ApiPlayerState {
  project_id: string;
  player_id: string;
  xp: number;
  updated_at?: string;
}

interface RequestResult {
  payload: unknown;
  status: number;
}

/** Error returned by the Oke Gaas SDK for HTTP, transport, or response failures. */
export class GaasError extends Error {
  readonly code: string;
  readonly status: number | undefined;

  constructor(code: string, message: string, status?: number, options?: ErrorOptions) {
    super(message, options);
    this.name = "GaasError";
    this.code = code;
    this.status = status;
  }
}

export interface GaasClient {
  /**
   * Send one external event. When eventId is omitted the SDK generates one once
   * for this call. Supply eventId explicitly when idempotency must survive a new
   * process or a caller-managed retry.
   */
  track(eventType: string, input: TrackInput, options?: RequestOptions): Promise<TrackResult>;
  players: {
    create(input: CreatePlayerInput, options?: RequestOptions): Promise<Player>;
    /** Retrieve the current materialized Player State. */
    get(playerId: string, options?: RequestOptions): Promise<PlayerState>;
  };
  rules: {
    /** Create a version-1 exact-event XP rule. */
    create(input: CreateRuleInput, options?: RequestOptions): Promise<Rule>;
  };
}

/**
 * Create a project-scoped Oke Gaas client.
 *
 * @remarks
 * This API is intentionally unstable during MVP validation. The project API key
 * is a secret and must not be embedded in an untrusted browser bundle.
 */
export function createGaas(config: GaasConfig): GaasClient {
  const projectId = requireNonEmpty(config.projectId, "projectId");
  const apiKey = requireNonEmpty(config.apiKey, "apiKey");
  const baseUrl = normalizeBaseUrl(config.baseUrl ?? DEFAULT_BASE_URL);
  const timeoutMs = normalizeTimeoutMs(config.timeoutMs ?? DEFAULT_TIMEOUT_MS);
  const projectPath = `/v1/projects/${encodeURIComponent(projectId)}`;

  const request = async (
    path: string,
    init: RequestInit = {},
    options: RequestOptions = {},
  ): Promise<RequestResult> => {
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");
    headers.set("Authorization", `Bearer ${apiKey}`);
    if (init.body !== undefined && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    const requestSignal = createRequestSignal(options.signal, timeoutMs);
    let response: Response;
    let text: string;

    try {
      response = await fetch(`${baseUrl}${path}`, {
        ...init,
        headers,
        signal: requestSignal.signal,
      });
      text = await response.text();
    } catch (cause) {
      if (requestSignal.didTimeout()) {
        throw new GaasError(
          "timeout_error",
          `request to Oke Gaas exceeded ${timeoutMs}ms`,
          undefined,
          { cause },
        );
      }
      if (requestSignal.wasCallerAborted()) {
        throw new GaasError("request_aborted", "request to Oke Gaas was aborted", undefined, { cause });
      }
      throw new GaasError("network_error", "request to Oke Gaas failed", undefined, { cause });
    } finally {
      requestSignal.cleanup();
    }

    const payload = parseJson(text);

    if (!response.ok) {
      if (isApiErrorEnvelope(payload)) {
        throw new GaasError(payload.error.code, payload.error.message, response.status);
      }
      throw new GaasError("http_error", `Oke Gaas returned HTTP ${response.status}`, response.status);
    }

    if (payload === undefined) {
      throw invalidResponse(response.status);
    }

    return { payload, status: response.status };
  };

  return {
    async track(eventType, input, options) {
      const normalizedEventType = requireNonEmpty(eventType, "eventType");
      const playerId = requireNonEmpty(input.playerId, "playerId");
      const eventId = input.eventId === undefined
        ? generateEventId()
        : requireNonEmpty(input.eventId, "eventId");
      const occurredAt = normalizeOccurredAt(input.occurredAt);
      const result = await request(
        `${projectPath}/events`,
        {
          method: "POST",
          body: stringifyJson({
            event_id: eventId,
            player_id: playerId,
            type: normalizedEventType,
            occurred_at: occurredAt,
            properties: input.properties ?? {},
          }),
        },
        options,
      );
      const value = requireApiTrackResult(result.payload, result.status);
      requireResponseInvariant(value.event_id === eventId, result.status);
      requireResponseInvariant(value.state.player_id === playerId, result.status);

      return {
        eventId: value.event_id,
        duplicate: value.duplicate,
        grants: value.grants.map((grant) => ({
          id: grant.id,
          ruleId: grant.rule_id,
          ruleVersion: grant.rule_version,
          rewardType: grant.reward_type,
          amount: grant.amount,
        })),
        state: {
          playerId: value.state.player_id,
          xp: value.state.xp,
          ...(value.state.updated_at === undefined ? {} : { updatedAt: value.state.updated_at }),
        },
      };
    },

    players: {
      async create(input, options) {
        const externalId = requireNonEmpty(input.externalId, "externalId");
        const result = await request(
          `${projectPath}/players`,
          {
            method: "POST",
            body: stringifyJson({ external_id: externalId }),
          },
          options,
        );
        const value = requireApiPlayer(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        requireResponseInvariant(value.external_id === externalId, result.status);
        return {
          id: value.id,
          projectId: value.project_id,
          externalId: value.external_id,
          createdAt: value.created_at,
        };
      },

      async get(playerId, options) {
        const normalizedPlayerId = requireNonEmpty(playerId, "playerId");
        const result = await request(
          `${projectPath}/players/${encodeURIComponent(normalizedPlayerId)}/state`,
          {},
          options,
        );
        const value = requireApiPlayerState(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        requireResponseInvariant(value.player_id === normalizedPlayerId, result.status);
        return {
          projectId: value.project_id,
          playerId: value.player_id,
          xp: value.xp,
          ...(value.updated_at === undefined ? {} : { updatedAt: value.updated_at }),
        };
      },
    },

    rules: {
      async create(input, options) {
        const eventType = requireNonEmpty(input.eventType, "eventType");
        const xp = requirePositiveSafeInteger(input.xp, "xp");
        const result = await request(
          `${projectPath}/rules`,
          {
            method: "POST",
            body: stringifyJson({ event_type: eventType, xp }),
          },
          options,
        );
        const value = requireApiRule(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        requireResponseInvariant(value.event_type === eventType, result.status);
        requireResponseInvariant(value.xp === xp, result.status);
        return {
          id: value.id,
          projectId: value.project_id,
          version: value.version,
          eventType: value.event_type,
          xp: value.xp,
        };
      },
    },
  };
}

function requireNonEmpty(value: string, name: string): string {
  if (typeof value !== "string" || value.trim() === "") {
    throw new TypeError(`${name} must not be empty`);
  }
  return value;
}

function requirePositiveSafeInteger(value: number, name: string): number {
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new TypeError(`${name} must be a positive safe integer`);
  }
  return value;
}

function normalizeTimeoutMs(value: number): number {
  if (!Number.isSafeInteger(value) || value <= 0 || value > MAX_TIMEOUT_MS) {
    throw new TypeError(`timeoutMs must be an integer between 1 and ${MAX_TIMEOUT_MS}`);
  }
  return value;
}

function normalizeBaseUrl(value: string): string {
  if (typeof value !== "string") {
    throw new TypeError("baseUrl must be a valid absolute URL");
  }
  if (value.includes("?")) {
    throw new TypeError("baseUrl must not contain a query string");
  }
  if (value.includes("#")) {
    throw new TypeError("baseUrl must not contain a fragment");
  }

  let url: URL;
  try {
    url = new URL(value);
  } catch (cause) {
    throw new TypeError("baseUrl must be a valid absolute URL", { cause });
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new TypeError("baseUrl must use http or https");
  }
  if (url.username !== "" || url.password !== "") {
    throw new TypeError("baseUrl must not contain credentials");
  }
  url.pathname = url.pathname.replace(/\/+$/, "");
  return url.toString().replace(/\/+$/, "");
}

function normalizeOccurredAt(value: Date | string | undefined): string {
  if (value === undefined) {
    return new Date().toISOString();
  }
  if (value instanceof Date) {
    if (Number.isNaN(value.getTime())) {
      throw new TypeError("occurredAt must be a valid Date");
    }
    return value.toISOString();
  }
  return requireNonEmpty(value, "occurredAt");
}

function generateEventId(): string {
  if (typeof globalThis.crypto?.randomUUID !== "function") {
    throw new GaasError(
      "crypto_unavailable",
      "crypto.randomUUID is required when track() is called without eventId",
    );
  }
  return `evt_${globalThis.crypto.randomUUID()}`;
}

function stringifyJson(value: unknown): string {
  try {
    const encoded = JSON.stringify(value);
    if (encoded === undefined) {
      throw new TypeError("request body is not JSON-serializable");
    }
    return encoded;
  } catch (cause) {
    if (cause instanceof TypeError && cause.message === "request body is not JSON-serializable") {
      throw cause;
    }
    throw new TypeError("request body must be JSON-serializable", { cause });
  }
}

function parseJson(value: string): unknown {
  if (value === "") {
    return undefined;
  }
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return undefined;
  }
}

function isApiErrorEnvelope(value: unknown): value is ApiErrorEnvelope {
  if (!isRecord(value) || !isRecord(value.error)) {
    return false;
  }
  return typeof value.error.code === "string" && typeof value.error.message === "string";
}

function requireApiPlayer(value: unknown, status: number): ApiPlayer {
  if (!isApiPlayer(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiRule(value: unknown, status: number): ApiRule {
  if (!isApiRule(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiTrackResult(value: unknown, status: number): ApiTrackResult {
  if (!isApiTrackResult(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiPlayerState(value: unknown, status: number): ApiPlayerState {
  if (!isApiPlayerState(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireResponseInvariant(condition: boolean, status: number): void {
  if (!condition) {
    throw invalidResponse(status);
  }
}

function invalidResponse(status: number): GaasError {
  return new GaasError(
    "invalid_response",
    "Oke Gaas returned an invalid or empty JSON response",
    status,
  );
}

function isApiPlayer(value: unknown): value is ApiPlayer {
  return isRecord(value)
    && typeof value.id === "string"
    && typeof value.project_id === "string"
    && typeof value.external_id === "string"
    && typeof value.created_at === "string";
}

function isApiRule(value: unknown): value is ApiRule {
  return isRecord(value)
    && typeof value.id === "string"
    && typeof value.project_id === "string"
    && isPositiveSafeInteger(value.version)
    && typeof value.event_type === "string"
    && isPositiveSafeInteger(value.xp);
}

function isApiRewardGrant(value: unknown): value is ApiRewardGrant {
  return isRecord(value)
    && typeof value.id === "string"
    && typeof value.rule_id === "string"
    && isPositiveSafeInteger(value.rule_version)
    && typeof value.reward_type === "string"
    && Number.isSafeInteger(value.amount);
}

function isApiTrackPlayerState(value: unknown): value is ApiTrackPlayerState {
  return isRecord(value)
    && typeof value.player_id === "string"
    && isNonNegativeSafeInteger(value.xp)
    && isOptionalString(value.updated_at);
}

function isApiTrackResult(value: unknown): value is ApiTrackResult {
  return isRecord(value)
    && typeof value.event_id === "string"
    && typeof value.duplicate === "boolean"
    && Array.isArray(value.grants)
    && value.grants.every(isApiRewardGrant)
    && isApiTrackPlayerState(value.state);
}

function isApiPlayerState(value: unknown): value is ApiPlayerState {
  return isRecord(value)
    && typeof value.project_id === "string"
    && typeof value.player_id === "string"
    && isNonNegativeSafeInteger(value.xp)
    && isOptionalString(value.updated_at);
}

function isPositiveSafeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value > 0;
}

function isNonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}

function isOptionalString(value: unknown): boolean {
  return value === undefined || typeof value === "string";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function createRequestSignal(callerSignal: AbortSignal | undefined, timeoutMs: number): {
  signal: AbortSignal;
  cleanup: () => void;
  didTimeout: () => boolean;
  wasCallerAborted: () => boolean;
} {
  const controller = new AbortController();
  let timedOut = false;

  const timeoutID = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);

  const onCallerAbort = () => {
    controller.abort(callerSignal?.reason);
  };

  if (callerSignal?.aborted) {
    onCallerAbort();
  } else {
    callerSignal?.addEventListener("abort", onCallerAbort, { once: true });
  }

  return {
    signal: controller.signal,
    cleanup: () => {
      clearTimeout(timeoutID);
      callerSignal?.removeEventListener("abort", onCallerAbort);
    },
    didTimeout: () => timedOut,
    wasCallerAborted: () => callerSignal?.aborted === true,
  };
}
