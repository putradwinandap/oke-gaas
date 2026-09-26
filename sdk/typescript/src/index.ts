const DEFAULT_BASE_URL = "http://localhost:8080";

export interface GaasConfig {
  projectId: string;
  apiKey: string;
  baseUrl?: string;
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

interface ApiTrackResult {
  event_id: string;
  duplicate: boolean;
  grants: Array<{
    id: string;
    rule_id: string;
    rule_version: number;
    reward_type: string;
    amount: number;
  }>;
  state: {
    player_id: string;
    xp: number;
    updated_at?: string;
  };
}

interface ApiPlayerState {
  project_id: string;
  player_id: string;
  xp: number;
  updated_at?: string;
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
  track(eventType: string, input: TrackInput): Promise<TrackResult>;
  players: {
    create(input: CreatePlayerInput): Promise<Player>;
    /** Retrieve the current materialized Player State. */
    get(playerId: string): Promise<PlayerState>;
  };
  rules: {
    /** Create a version-1 exact-event XP rule. */
    create(input: CreateRuleInput): Promise<Rule>;
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
  const projectPath = `/v1/projects/${encodeURIComponent(projectId)}`;

  const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");
    headers.set("Authorization", `Bearer ${apiKey}`);
    if (init.body !== undefined && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    let response: Response;
    try {
      response = await fetch(`${baseUrl}${path}`, {
        ...init,
        headers,
      });
    } catch (cause) {
      throw new GaasError("network_error", "request to Oke Gaas failed", undefined, { cause });
    }

    const text = await response.text();
    const payload = parseJson(text);

    if (!response.ok) {
      if (isApiErrorEnvelope(payload)) {
        throw new GaasError(payload.error.code, payload.error.message, response.status);
      }
      throw new GaasError("http_error", `Oke Gaas returned HTTP ${response.status}`, response.status);
    }

    if (payload === undefined) {
      throw new GaasError(
        "invalid_response",
        "Oke Gaas returned an invalid or empty JSON response",
        response.status,
      );
    }

    return payload as T;
  };

  return {
    async track(eventType, input) {
      const occurredAt = input.occurredAt instanceof Date
        ? input.occurredAt.toISOString()
        : input.occurredAt ?? new Date().toISOString();
      const eventId = input.eventId ?? generateEventId();
      const value = await request<ApiTrackResult>(`${projectPath}/events`, {
        method: "POST",
        body: JSON.stringify({
          event_id: eventId,
          player_id: input.playerId,
          type: eventType,
          occurred_at: occurredAt,
          properties: input.properties ?? {},
        }),
      });

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
      async create(input) {
        const value = await request<ApiPlayer>(`${projectPath}/players`, {
          method: "POST",
          body: JSON.stringify({ external_id: input.externalId }),
        });
        return {
          id: value.id,
          projectId: value.project_id,
          externalId: value.external_id,
          createdAt: value.created_at,
        };
      },

      async get(playerId) {
        const value = await request<ApiPlayerState>(
          `${projectPath}/players/${encodeURIComponent(playerId)}/state`,
        );
        return {
          projectId: value.project_id,
          playerId: value.player_id,
          xp: value.xp,
          ...(value.updated_at === undefined ? {} : { updatedAt: value.updated_at }),
        };
      },
    },

    rules: {
      async create(input) {
        const value = await request<ApiRule>(`${projectPath}/rules`, {
          method: "POST",
          body: JSON.stringify({ event_type: input.eventType, xp: input.xp }),
        });
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
  if (value.trim() === "") {
    throw new TypeError(`${name} must not be empty`);
  }
  return value;
}

function normalizeBaseUrl(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch (cause) {
    throw new TypeError("baseUrl must be a valid absolute URL", { cause });
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new TypeError("baseUrl must use http or https");
  }
  return value.replace(/\/+$/, "");
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
  if (typeof value !== "object" || value === null || !("error" in value)) {
    return false;
  }
  const error = (value as { error?: unknown }).error;
  return typeof error === "object"
    && error !== null
    && typeof (error as { code?: unknown }).code === "string"
    && typeof (error as { message?: unknown }).message === "string";
}
