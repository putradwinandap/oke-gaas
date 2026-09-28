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
  conditions?: Record<string, unknown>;
  /** Grant on every Nth matching Event for each Player. Defaults to 1. */
  matchEvery?: number;
  /** Grant at most once per UTC calendar day, based on occurredAt. Defaults to false. */
  oncePerUtcDay?: boolean;
}

export interface CreateCounterInput {
  name: string;
  eventType: string;
  conditions?: Record<string, unknown>;
}

export interface CreateBadgeInput { name: string; description?: string; }
export interface CreateBadgeRuleInput { eventType: string; badgeId: string; conditions?: Record<string, unknown>; }
export interface BadgeDefinition { id: string; projectId: string; name: string; description: string; createdAt: string; }
export interface PlayerBadge { badgeId: string; name: string; description: string; eventId: string; ruleId: string; ruleVersion: number; grantedAt: string; }
export interface CreateAchievementInput { name: string; counterId: string; target: number; }
export interface AchievementDefinition { id: string; projectId: string; name: string; counterId: string; target: number; createdAt: string; }
export interface PlayerAchievementProgress extends AchievementDefinition { unlocked: boolean; eventId?: string; counterValue?: number; unlockedAt?: string; }

export interface Rule {
  id: string;
  projectId: string;
  version: number;
  eventType: string;
  xp: number;
  rewardType?: "xp" | "badge";
  badgeId?: string;
  conditions: Record<string, unknown>;
  matchEvery: number;
  oncePerUtcDay: boolean;
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
  badgeId?: string;
}

export interface TrackPlayerState {
  playerId: string;
  xp: number;
  level: number;
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
  level: number;
  updatedAt?: string;
}

export interface LevelThreshold {
  number: number;
  minXp: number;
}

export interface CounterDefinition {
  id: string;
  projectId: string;
  name: string;
  eventType: string;
  conditions: Record<string, unknown>;
  createdAt: string;
}

export interface PlayerCounterProgress {
  counterId: string;
  name: string;
  eventType: string;
  conditions: Record<string, unknown>;
  value: number;
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
  reward_type?: "xp" | "badge";
  badge_id?: string;
  conditions: Record<string, unknown>;
  match_every: number;
  once_per_utc_day: boolean;
}

interface ApiRewardGrant {
  id: string;
  rule_id: string;
  rule_version: number;
  reward_type: string;
  amount: number;
  badge_id?: string;
}

interface ApiBadge { id:string; project_id:string; name:string; description:string; created_at:string; }
interface ApiBadgeList { project_id:string; badges:ApiBadge[]; }
interface ApiPlayerBadge { badge_id:string; name:string; description:string; event_id:string; rule_id:string; rule_version:number; granted_at:string; }
interface ApiPlayerBadgeList { project_id:string; player_id:string; badges:ApiPlayerBadge[]; }
interface ApiAchievement { id:string; project_id:string; name:string; counter_id:string; target:number; created_at:string; }
interface ApiAchievementList { project_id:string; achievements:ApiAchievement[]; }
interface ApiPlayerAchievement extends ApiAchievement { unlocked:boolean; event_id?:string; counter_value?:number; unlocked_at?:string; }
interface ApiPlayerAchievementList { project_id:string; player_id:string; achievements:ApiPlayerAchievement[]; }

interface ApiTrackPlayerState {
  player_id: string;
  xp: number;
  level: number;
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
  level: number;
  updated_at?: string;
}

interface ApiLevelThreshold {
  number: number;
  min_xp: number;
}

interface ApiLevelList {
  project_id: string;
  levels: ApiLevelThreshold[];
}

interface ApiCounter {
  id: string;
  project_id: string;
  name: string;
  event_type: string;
  conditions: Record<string, unknown>;
  created_at: string;
}

interface ApiCounterList {
  project_id: string;
  counters: ApiCounter[];
}

interface ApiPlayerCounterProgress {
  counter_id: string;
  name: string;
  event_type: string;
  conditions: Record<string, unknown>;
  value: number;
  updated_at?: string;
}

interface ApiPlayerCounterList {
  project_id: string;
  player_id: string;
  counters: ApiPlayerCounterProgress[];
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
    /** Retrieve all Project Counter progress for the Player, including zero values. */
    getCounters(playerId: string, options?: RequestOptions): Promise<PlayerCounterProgress[]>;
    getBadges(playerId: string, options?: RequestOptions): Promise<PlayerBadge[]>;
    getAchievements(playerId: string, options?: RequestOptions): Promise<PlayerAchievementProgress[]>;
  };
  rules: {
    /** Create a version-1 exact-event XP rule. */
    create(input: CreateRuleInput, options?: RequestOptions): Promise<Rule>;
    createBadge(input: CreateBadgeRuleInput, options?: RequestOptions): Promise<Rule>;
  };
  levels: {
    append(minXp: number, options?: RequestOptions): Promise<LevelThreshold>;
    list(options?: RequestOptions): Promise<LevelThreshold[]>;
  };
  counters: {
    create(input: CreateCounterInput, options?: RequestOptions): Promise<CounterDefinition>;
    list(options?: RequestOptions): Promise<CounterDefinition[]>;
  };
  badges: { create(input: CreateBadgeInput, options?: RequestOptions): Promise<BadgeDefinition>; list(options?: RequestOptions): Promise<BadgeDefinition[]>; };
  achievements: { create(input: CreateAchievementInput, options?: RequestOptions): Promise<AchievementDefinition>; list(options?: RequestOptions): Promise<AchievementDefinition[]>; };
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
          ...(grant.badge_id === undefined ? {} : { badgeId: grant.badge_id }),
        })),
        state: {
          playerId: value.state.player_id,
          xp: value.state.xp,
          level: value.state.level,
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
          level: value.level,
          ...(value.updated_at === undefined ? {} : { updatedAt: value.updated_at }),
        };
      },

      async getCounters(playerId, options) {
        const normalizedPlayerId = requireNonEmpty(playerId, "playerId");
        const result = await request(
          `${projectPath}/players/${encodeURIComponent(normalizedPlayerId)}/counters`,
          {},
          options,
        );
        const value = requireApiPlayerCounterList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        requireResponseInvariant(value.player_id === normalizedPlayerId, result.status);
        return value.counters.map((entry) => ({
          counterId: entry.counter_id,
          name: entry.name,
          eventType: entry.event_type,
          conditions: entry.conditions,
          value: entry.value,
          ...(entry.updated_at === undefined ? {} : { updatedAt: entry.updated_at }),
        }));
      },

      async getBadges(playerId, options) {
        const normalizedPlayerId = requireNonEmpty(playerId, "playerId");
        const result = await request(`${projectPath}/players/${encodeURIComponent(normalizedPlayerId)}/badges`, {}, options);
        const value = requireApiPlayerBadgeList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId && value.player_id === normalizedPlayerId, result.status);
        return value.badges.map((item) => ({ badgeId:item.badge_id, name:item.name, description:item.description, eventId:item.event_id, ruleId:item.rule_id, ruleVersion:item.rule_version, grantedAt:item.granted_at }));
      },
      async getAchievements(playerId, options) {
        const normalizedPlayerId = requireNonEmpty(playerId, "playerId");
        const result = await request(`${projectPath}/players/${encodeURIComponent(normalizedPlayerId)}/achievements`, {}, options);
        const value = requireApiPlayerAchievementList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId && value.player_id === normalizedPlayerId, result.status);
        return value.achievements.map((entry) => ({ id:entry.id, projectId:entry.project_id, name:entry.name, counterId:entry.counter_id, target:entry.target, createdAt:entry.created_at, unlocked:entry.unlocked, ...(entry.event_id === undefined ? {} : {eventId:entry.event_id}), ...(entry.counter_value === undefined ? {} : {counterValue:entry.counter_value}), ...(entry.unlocked_at === undefined ? {} : {unlockedAt:entry.unlocked_at}) }));
      },
    },

    levels: {
      async append(minXp, options) {
        const normalizedMinXp = requirePositiveSafeInteger(minXp, "minXp");
        const result = await request(
          `${projectPath}/levels`,
          {
            method: "POST",
            body: stringifyJson({ min_xp: normalizedMinXp }),
          },
          options,
        );
        const value = requireApiLevelThreshold(result.payload, result.status);
        return { number: value.number, minXp: value.min_xp };
      },

      async list(options) {
        const result = await request(`${projectPath}/levels`, {}, options);
        const value = requireApiLevelList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        return value.levels.map((entry) => ({ number: entry.number, minXp: entry.min_xp }));
      },
    },

    rules: {
      async create(input, options) {
        const eventType = requireNonEmpty(input.eventType, "eventType");
        const xp = requirePositiveSafeInteger(input.xp, "xp");
        const matchEvery = input.matchEvery === undefined
          ? 1
          : requirePositiveSafeInteger(input.matchEvery, "matchEvery");
        const oncePerUtcDay = input.oncePerUtcDay === undefined
          ? false
          : requireBoolean(input.oncePerUtcDay, "oncePerUtcDay");
        const result = await request(
          `${projectPath}/rules`,
          {
            method: "POST",
            body: stringifyJson({
              event_type: eventType,
              xp,
              conditions: input.conditions ?? {},
              match_every: matchEvery,
              once_per_utc_day: oncePerUtcDay,
            }),
          },
          options,
        );
        const value = requireApiRule(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        requireResponseInvariant(value.event_type === eventType, result.status);
        requireResponseInvariant(value.xp === xp, result.status);
        requireResponseInvariant(value.reward_type === undefined || value.reward_type === "xp", result.status);
        requireResponseInvariant(value.match_every === matchEvery, result.status);
        requireResponseInvariant(value.once_per_utc_day === oncePerUtcDay, result.status);
        return {
          id: value.id,
          projectId: value.project_id,
          version: value.version,
          eventType: value.event_type,
          xp: value.xp,
          conditions: value.conditions,
          matchEvery: value.match_every,
          oncePerUtcDay: value.once_per_utc_day,
        };
      },

      async createBadge(input, options) {
        const eventType = requireNonEmpty(input.eventType, "eventType");
        const badgeId = requireNonEmpty(input.badgeId, "badgeId");
        const result = await request(`${projectPath}/rules`, { method:"POST", body:stringifyJson({ event_type:eventType, reward_type:"badge", badge_id:badgeId, conditions:input.conditions ?? {} }) }, options);
        const value = requireApiRule(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId && value.event_type === eventType && value.reward_type === "badge" && value.badge_id === badgeId, result.status);
        return { id:value.id, projectId:value.project_id, version:value.version, eventType:value.event_type, xp:0, rewardType:"badge", badgeId, conditions:value.conditions, matchEvery:1, oncePerUtcDay:false };
      },
    },

    counters: {
      async create(input, options) {
        const name = requireNonEmpty(input.name, "name");
        const eventType = requireNonEmpty(input.eventType, "eventType");
        const result = await request(
          `${projectPath}/counters`,
          {
            method: "POST",
            body: stringifyJson({
              name,
              event_type: eventType,
              conditions: input.conditions ?? {},
            }),
          },
          options,
        );
        const value = requireApiCounter(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        requireResponseInvariant(value.name === name, result.status);
        requireResponseInvariant(value.event_type === eventType, result.status);
        return {
          id: value.id,
          projectId: value.project_id,
          name: value.name,
          eventType: value.event_type,
          conditions: value.conditions,
          createdAt: value.created_at,
        };
      },

      async list(options) {
        const result = await request(`${projectPath}/counters`, {}, options);
        const value = requireApiCounterList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        return value.counters.map((entry) => ({
          id: entry.id,
          projectId: entry.project_id,
          name: entry.name,
          eventType: entry.event_type,
          conditions: entry.conditions,
          createdAt: entry.created_at,
        }));
      },
    },

    badges: {
      async create(input, options) {
        const name = requireNonEmpty(input.name, "name");
        const description = input.description === undefined ? "" : input.description;
        if (typeof description !== "string") throw new TypeError("description must be a string");
        const result = await request(`${projectPath}/badges`, { method:"POST", body:stringifyJson({ name, description }) }, options);
        const value = requireApiBadge(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId && value.name === name, result.status);
        return { id:value.id, projectId:value.project_id, name:value.name, description:value.description, createdAt:value.created_at };
      },
      async list(options) {
        const result = await request(`${projectPath}/badges`, {}, options);
        const value = requireApiBadgeList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        return value.badges.map((item) => ({ id:item.id, projectId:item.project_id, name:item.name, description:item.description, createdAt:item.created_at }));
      },
    },
    achievements: {
      async create(input, options) {
        const name = requireNonEmpty(input.name, "name");
        const counterId = requireNonEmpty(input.counterId, "counterId");
        if (counterId.length > 64) throw new TypeError("counterId must not exceed 64 characters");
        const target = requirePositiveSafeInteger(input.target, "target");
        const result = await request(`${projectPath}/achievements`, { method:"POST", body:stringifyJson({name, counter_id:counterId, target}) }, options);
        const value = requireApiAchievement(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId && value.name === name && value.counter_id === counterId && value.target === target, result.status);
        return {id:value.id, projectId:value.project_id, name:value.name, counterId:value.counter_id, target:value.target, createdAt:value.created_at};
      },
      async list(options) {
        const result = await request(`${projectPath}/achievements`, {}, options);
        const value = requireApiAchievementList(result.payload, result.status);
        requireResponseInvariant(value.project_id === projectId, result.status);
        return value.achievements.map((entry) => ({id:entry.id, projectId:entry.project_id, name:entry.name, counterId:entry.counter_id, target:entry.target, createdAt:entry.created_at}));
      },
    },
  };
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

function requireBoolean(value: boolean, name: string): boolean {
  if (typeof value !== "boolean") {
    throw new TypeError(`${name} must be a boolean`);
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

function requireApiAchievement(value: unknown, status: number): ApiAchievement { if (!isApiAchievement(value)) throw invalidResponse(status); return value; }
function requireApiAchievementList(value: unknown, status: number): ApiAchievementList { if (!isApiAchievementList(value)) throw invalidResponse(status); return value; }
function requireApiPlayerAchievementList(value: unknown, status: number): ApiPlayerAchievementList { if (!isApiPlayerAchievementList(value)) throw invalidResponse(status); return value; }

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

function requireApiLevelThreshold(value: unknown, status: number): ApiLevelThreshold {
  if (!isApiLevelThreshold(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiLevelList(value: unknown, status: number): ApiLevelList {
  if (!isApiLevelList(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiCounter(value: unknown, status: number): ApiCounter {
  if (!isApiCounter(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiCounterList(value: unknown, status: number): ApiCounterList {
  if (!isApiCounterList(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiPlayerCounterList(value: unknown, status: number): ApiPlayerCounterList {
  if (!isApiPlayerCounterList(value)) {
    throw invalidResponse(status);
  }
  return value;
}

function requireApiBadge(value: unknown, status:number): ApiBadge { if (!isApiBadge(value)) throw invalidResponse(status); return value; }
function requireApiBadgeList(value: unknown, status:number): ApiBadgeList { if (!isApiBadgeList(value)) throw invalidResponse(status); return value; }
function requireApiPlayerBadgeList(value: unknown, status:number): ApiPlayerBadgeList { if (!isApiPlayerBadgeList(value)) throw invalidResponse(status); return value; }

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
    && isNonNegativeSafeInteger(value.xp)
    && (value.reward_type === undefined || value.reward_type === "xp" || value.reward_type === "badge")
    && isOptionalString(value.badge_id)
    && isRecord(value.conditions)
    && isPositiveSafeInteger(value.match_every)
    && typeof value.once_per_utc_day === "boolean";
}

function isApiBadge(value:unknown): value is ApiBadge { return isRecord(value) && typeof value.id === "string" && typeof value.project_id === "string" && typeof value.name === "string" && typeof value.description === "string" && typeof value.created_at === "string"; }
function isApiBadgeList(value:unknown): value is ApiBadgeList { return isRecord(value) && typeof value.project_id === "string" && Array.isArray(value.badges) && value.badges.every(isApiBadge); }
function isApiAchievement(value:unknown): value is ApiAchievement { return isRecord(value) && typeof value.id === "string" && typeof value.project_id === "string" && typeof value.name === "string" && typeof value.counter_id === "string" && isPositiveSafeInteger(value.target) && typeof value.created_at === "string"; }
function isApiAchievementList(value:unknown): value is ApiAchievementList { return isRecord(value) && typeof value.project_id === "string" && Array.isArray(value.achievements) && value.achievements.every(isApiAchievement); }
function isApiPlayerAchievement(value:unknown): value is ApiPlayerAchievement { if (!isApiAchievement(value) || !isRecord(value) || typeof value.unlocked !== "boolean") return false; if (!value.unlocked) return value.event_id === undefined && value.counter_value === undefined && value.unlocked_at === undefined; return typeof value.event_id === "string" && isPositiveSafeInteger(value.counter_value) && typeof value.unlocked_at === "string"; }
function isApiPlayerAchievementList(value:unknown): value is ApiPlayerAchievementList { return isRecord(value) && typeof value.project_id === "string" && typeof value.player_id === "string" && Array.isArray(value.achievements) && value.achievements.every(isApiPlayerAchievement); }
function isApiPlayerBadge(value:unknown): value is ApiPlayerBadge { return isRecord(value) && typeof value.badge_id === "string" && typeof value.name === "string" && typeof value.description === "string" && typeof value.event_id === "string" && typeof value.rule_id === "string" && isPositiveSafeInteger(value.rule_version) && typeof value.granted_at === "string"; }
function isApiPlayerBadgeList(value:unknown): value is ApiPlayerBadgeList { return isRecord(value) && typeof value.project_id === "string" && typeof value.player_id === "string" && Array.isArray(value.badges) && value.badges.every(isApiPlayerBadge); }

function isApiRewardGrant(value: unknown): value is ApiRewardGrant {
  return isRecord(value)
    && typeof value.id === "string"
    && typeof value.rule_id === "string"
    && isPositiveSafeInteger(value.rule_version)
    && typeof value.reward_type === "string"
    && isOptionalString(value.badge_id)
    && Number.isSafeInteger(value.amount);
}

function isApiTrackPlayerState(value: unknown): value is ApiTrackPlayerState {
  return isRecord(value)
    && typeof value.player_id === "string"
    && isNonNegativeSafeInteger(value.xp)
    && isPositiveSafeInteger(value.level)
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
    && isPositiveSafeInteger(value.level)
    && isOptionalString(value.updated_at);
}

function isApiLevelThreshold(value: unknown): value is ApiLevelThreshold {
  return isRecord(value)
    && isPositiveSafeInteger(value.number)
    && isNonNegativeSafeInteger(value.min_xp);
}

function isApiLevelList(value: unknown): value is ApiLevelList {
  return isRecord(value)
    && typeof value.project_id === "string"
    && Array.isArray(value.levels)
    && value.levels.every(isApiLevelThreshold);
}

function isApiCounter(value: unknown): value is ApiCounter {
  return isRecord(value)
    && typeof value.id === "string"
    && typeof value.project_id === "string"
    && typeof value.name === "string"
    && typeof value.event_type === "string"
    && isRecord(value.conditions)
    && typeof value.created_at === "string";
}

function isApiCounterList(value: unknown): value is ApiCounterList {
  return isRecord(value)
    && typeof value.project_id === "string"
    && Array.isArray(value.counters)
    && value.counters.every(isApiCounter);
}

function isApiPlayerCounterProgress(value: unknown): value is ApiPlayerCounterProgress {
  return isRecord(value)
    && typeof value.counter_id === "string"
    && typeof value.name === "string"
    && typeof value.event_type === "string"
    && isRecord(value.conditions)
    && isNonNegativeSafeInteger(value.value)
    && isOptionalString(value.updated_at);
}

function isApiPlayerCounterList(value: unknown): value is ApiPlayerCounterList {
  return isRecord(value)
    && typeof value.project_id === "string"
    && typeof value.player_id === "string"
    && Array.isArray(value.counters)
    && value.counters.every(isApiPlayerCounterProgress);
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
  let abortSource: "timeout" | "caller" | undefined;

  const timeoutID = setTimeout(() => {
    if (!controller.signal.aborted) {
      abortSource = "timeout";
      controller.abort();
    }
  }, timeoutMs);

  const onCallerAbort = () => {
    if (!controller.signal.aborted) {
      abortSource = "caller";
      controller.abort(callerSignal?.reason);
    }
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
    didTimeout: () => abortSource === "timeout",
    wasCallerAborted: () => abortSource === "caller",
  };
}
