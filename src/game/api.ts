import { fallbackBootstrap, fallbackLeaderboard } from "./fallbackContent";
import type {
  ApiErrorResponse,
  GameBootstrap,
  LeaderboardEntry,
  RunResult,
  RunSubmission,
  UserReservation
} from "./types";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";
const API_RECOVERY_PROBE_COOLDOWN_MS = 3_000;
const API_HEALTH_TIMEOUT_MS = 2_500;
const API_REQUEST_TIMEOUT_MS = 8_000;
let apiEnabled = true;
let apiUsesServerFallback = false;
let lastRecoveryProbeAt = 0;

export type ApiConnectionState = {
  kind: "checking" | "online" | "fallback" | "error";
  label: string;
  detail: string;
};

type HealthResponse = {
  status?: string;
  database?: "online" | "fallback" | string;
};

let apiConnectionState: ApiConnectionState = {
  kind: "checking",
  label: "API 检测中",
  detail: "正在检测 Go 后端是否可用。"
};

class ApiRequestError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string
  ) {
    super(message);
    this.name = "ApiRequestError";
  }
}

function setApiConnectionState(nextState: ApiConnectionState): void {
  apiConnectionState = nextState;
}

export function getApiConnectionState(): ApiConnectionState {
  return { ...apiConnectionState };
}

function setOnlineState(): void {
  apiEnabled = true;
  apiUsesServerFallback = false;
  setApiConnectionState({
    kind: "online",
    label: "Go API 已接通",
    detail: "内容、用户名、成绩和排行榜正在通过 Go 后端和 PostgreSQL 处理。"
  });
}

function setFallbackState(detail: string): void {
  apiEnabled = false;
  apiUsesServerFallback = false;
  setApiConnectionState({
    kind: "fallback",
    label: "本地兜底数据",
    detail
  });
}

function setServerFallbackState(): void {
  apiEnabled = true;
  apiUsesServerFallback = true;
  setApiConnectionState({
    kind: "fallback",
    label: "Go API 内存兜底",
    detail: "Go API 已启动，但没有配置 DATABASE_URL；内容、用户名、成绩和排行榜正在使用后端开发兜底，不是 PostgreSQL。"
  });
}

function setErrorState(detail: string): void {
  setApiConnectionState({
    kind: "error",
    label: "API 请求异常",
    detail
  });
}

async function handleSuccessfulHealthResponse(response: Response): Promise<void> {
  const payload = (await response.json().catch(() => null)) as Partial<HealthResponse> | null;

  /*
   * /healthz 的 HTTP 200 只说明浏览器已经连到 Go 进程，不一定说明真实数据库链路也可用。
   * Go 后端在没有 DATABASE_URL 时会返回 database:"fallback"，这条路径仍然可以服务本地
   * 开发，但内容、用户名、成绩和排行榜都不是 PostgreSQL。前端必须把这两种 200 分开显示，
   * 否则数据库联调时会被“Go API 已接通”的文案误导，以为真实持久化已经生效。
   */
  if (payload?.status === "ok" && payload.database === "fallback") {
    setServerFallbackState();
    return;
  }

  setOnlineState();
}

async function refreshServerFallbackHealthIfDue(): Promise<void> {
  if (!apiUsesServerFallback) {
    return;
  }

  const now = Date.now();
  if (now - lastRecoveryProbeAt < API_RECOVERY_PROBE_COOLDOWN_MS) {
    return;
  }
  lastRecoveryProbeAt = now;

  /*
   * “Go API 内存兜底”和“Go API + PostgreSQL”都是 HTTP 200 的后端可达状态。不同之处在于
   * 前者只适合本地开发，后者才满足当前目标里的真实数据库联动。用户可能先打开页面时没有
   * DATABASE_URL，随后重启 Go 后端接上 PostgreSQL；如果这里不重新探健康检查，业务请求会
   * 已经写到数据库，界面却仍显示“内存兜底”。所以只在服务器兜底状态下按冷却时间复查一次
   * /healthz，避免每个请求都额外打探活，也避免状态长期停在旧后端形态。
   */
  try {
    const response = await fetchWithTimeout("/healthz", undefined, API_HEALTH_TIMEOUT_MS);
    if (response.ok) {
      await handleSuccessfulHealthResponse(response);
      return;
    }

    await handleStructuredHealthError(response);
  } catch {
    setFallbackState("Go 后端请求失败，前端已经切换到本地兜底数据。");
  }
}

async function fetchWithTimeout(path: string, init: RequestInit | undefined, timeoutMs: number): Promise<Response> {
  /*
   * 浏览器原生 fetch 默认没有“业务超时”。如果网络连接被代理、数据库或后端进程拖住，
   * Promise 可能长时间不结束，调用方写的 await 就会一直等下去。这里用 AbortController
   * 包一层很薄的取消能力：AbortController 是浏览器提供的请求取消器，setTimeout 到点后
   * 调用 abort，fetch 会抛出异常，然后继续走下面已有的兜底逻辑。这样不会改变 API 协议，
   * 只是保证前端界面不会因为一个悬挂请求卡住开局、结算或排行榜刷新。
   */
  const controller = new AbortController();
  const timeoutId = window.setTimeout(() => controller.abort(), timeoutMs);

  try {
    return await fetch(`${API_BASE_URL}${path}`, {
      ...init,
      signal: controller.signal
    });
  } finally {
    window.clearTimeout(timeoutId);
  }
}

async function restoreApiIfPossible(): Promise<boolean> {
  if (apiEnabled) {
    return true;
  }

  const now = Date.now();
  if (now - lastRecoveryProbeAt < API_RECOVERY_PROBE_COOLDOWN_MS) {
    return false;
  }
  lastRecoveryProbeAt = now;

  /*
   * apiEnabled 是前端为了减少离线时重复 502 的本地开关，不代表后端永远不可用。
   * 如果 Go API 或 PostgreSQL 短暂故障后恢复，后续用户名占用、成绩提交和排行榜刷新
   * 应该能重新接回真实后端，而不是一直停留在本地兜底。这里用很轻的 /healthz 探活
   * 做恢复检查，并加一个短冷却，避免后端没启动时每次渲染都打到 Vite 代理。
  */
  try {
    const response = await fetchWithTimeout("/healthz", undefined, API_HEALTH_TIMEOUT_MS);
    if (response.ok) {
      await handleSuccessfulHealthResponse(response);
      return true;
    }

    if (await handleStructuredHealthError(response)) {
      return true;
    }
  } catch {
    // 保持当前兜底状态。具体业务函数会继续走本地兜底分支。
  }

  return false;
}

async function readStructuredApiError(response: Response): Promise<ApiErrorResponse | null> {
  const contentType = response.headers.get("Content-Type") ?? "";

  if (contentType.includes("application/json")) {
    const body = (await response.json().catch(() => null)) as Partial<ApiErrorResponse> | null;

    if (typeof body?.code === "string" && typeof body.message === "string") {
      return { code: body.code, message: body.message };
    }
  }

  return null;
}

async function readApiError(response: Response): Promise<ApiErrorResponse> {
  const structuredError = await readStructuredApiError(response);
  if (structuredError) {
    return structuredError;
  }

  return {
    code: `http_${response.status}`,
    message: `API 请求失败，HTTP 状态码 ${response.status}`
  };
}

async function handleStructuredHealthError(response: Response): Promise<boolean> {
  const error = await readStructuredApiError(response);
  if (!error) {
    return false;
  }

  /*
   * /healthz 返回 Go API 的结构化 JSON 错误时，说明浏览器已经打到了后端，只是后端
   * 认为数据库或自身状态不可用。这种情况不能当成“后端没启动”去加载本地兜底内容；
   * 否则正式联调会被误导成可玩。这里保持 apiEnabled 为 true，让后续内容包、用户名、
   * 排行榜请求继续打到 Go API，并把更具体的数据库错误显示出来。
  */
  apiEnabled = true;
  apiUsesServerFallback = false;
  setErrorState(`Go API 探活失败：${error.message}`);
  return true;
}

/**
 * 这里的泛型 `T` 可以理解成“调用者期望拿回来的 JSON 形状”。浏览器的
 * `fetch` 只知道自己拿到了一段 JSON 文本，并不知道这段文本应该是内容包、
 * 排行榜，还是用户名占用结果。调用者在 `requestJson<GameBootstrap>` 里写
 * 出 `GameBootstrap`，就是告诉 TypeScript：如果请求成功，后面的代码会按
 * 游戏内容包来使用这个返回值。
 *
 * 这个函数还会先检查 `apiEnabled`。`apiEnabled` 是健康检查后的本地开关，
 * 它不是安全机制，只是避免 Go API 没启动时每个业务请求都打到 Vite 代理，
 * 导致控制台反复出现 502。入口文件会先调用 `checkApiHealth`，如果后端不在，
 * 后续加载内容、排行榜、用户名和成绩提交会先走兜底；但业务请求仍会按冷却间隔探活，
 * 一旦 Go API 恢复，就重新回到真实后端。
 *
 * 后端明确拒绝请求时会返回 JSON 错误，例如 `{ code, message }`。这里会把
 * 这个错误转换成 `ApiRequestError`，让调用者能区分“服务端说这个请求不合法”
 * 和“服务端根本没连上”。这个区分很重要：用户名非法不能走本地兜底，否则等于绕过
 * 后端校验；后端没启动时才允许兜底，让前端开发还能继续。
 */
async function requestJson<T>(path: string, init?: RequestInit): Promise<T> {
  if (!apiEnabled && !(await restoreApiIfPossible())) {
    throw new Error("API disabled");
  }
  await refreshServerFallbackHealthIfDue();
  if (!apiEnabled) {
    throw new Error("API disabled");
  }

  let response: Response;
  try {
    response = await fetchWithTimeout(path, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...init?.headers
      }
    }, API_REQUEST_TIMEOUT_MS);
  } catch {
    lastRecoveryProbeAt = Date.now();
    setFallbackState("Go 后端请求失败，前端已经切换到本地兜底数据。");
    throw new Error("API unavailable");
  }

  if (!response.ok) {
    const error = await readApiError(response);
    setErrorState(error.message);
    throw new ApiRequestError(response.status, error.code, error.message);
  }

  if (apiUsesServerFallback) {
    setServerFallbackState();
  } else {
    setOnlineState();
  }
  return response.json() as Promise<T>;
}

export async function checkApiHealth(): Promise<boolean> {
  lastRecoveryProbeAt = Date.now();

  try {
    const response = await fetchWithTimeout("/healthz", undefined, API_HEALTH_TIMEOUT_MS);

    if (response.ok) {
      await handleSuccessfulHealthResponse(response);
      return true;
    }

    if (await handleStructuredHealthError(response)) {
      return true;
    }

    setFallbackState(`Go API 探活失败，HTTP 状态码 ${response.status}。`);
    return false;
  } catch {
    setFallbackState("Go API 未启动或网络不可达，当前使用本地兜底数据。");
    return false;
  }
}

/**
 * 内容包优先来自 Go 后端，因为商品、事件、状态和音乐入口后续都需要运营调整。
 * 这里保留 `fallbackBootstrap`，只是为了本地没有启动后端时仍然能打开页面、
 * 试玩核心交互。也就是说，兜底数据是开发体验，不是长期内容源。
 *
 * 如果后端已经明确返回 JSON 错误，说明请求到达了 Go API，但数据库读取、内容契约或
 * 服务端逻辑失败了。这个时候不能悄悄切到本地内容，否则玩家看到的商品和真实数据库
 * 不一致，后续算法排查也会被误导。因此明确的后端错误会继续抛出，让页面显示启动失败。
 */
export async function loadBootstrap(): Promise<GameBootstrap> {
  try {
    return await requestJson<GameBootstrap>("/api/content/bootstrap");
  } catch (error) {
    if (error instanceof ApiRequestError) {
      setErrorState(`内容包请求被后端拒绝：${error.message}`);
      throw error;
    } else {
      setFallbackState("内容包无法从 Go 后端加载，当前使用本地兜底内容。");
    }

    return fallbackBootstrap;
  }
}

/**
 * 排行榜也按同样策略处理：Go API 完全不可达时，才使用少量本地样例数据填充界面，
 * 让前端开发不用每次都先启动数据库。这里要特别区分“后端不可达”和“后端明确返回
 * 错误”。如果 Go API 已经返回了 JSON 错误，说明请求到达了后端，只是数据库读取、
 * 字段校验或服务端逻辑失败；这种情况下继续展示本地假榜，会让玩家误以为真实排行榜
 * 仍然可用。所以明确的后端错误只返回空列表，并通过 API 状态标签展示原因。
 */
export async function loadLeaderboard(contentVersion?: string): Promise<LeaderboardEntry[]> {
  const normalizedContentVersion = contentVersion?.trim();
  const leaderboardPath = normalizedContentVersion
    ? `/api/leaderboard?${new URLSearchParams({ contentVersion: normalizedContentVersion }).toString()}`
    : "/api/leaderboard";

  try {
    return await requestJson<LeaderboardEntry[]>(leaderboardPath);
  } catch (error) {
    if (error instanceof ApiRequestError) {
      setErrorState(`排行榜请求被后端拒绝：${error.message}`);
      return [];
    } else {
      setFallbackState("排行榜无法从 Go 后端加载，当前显示本地兜底榜单。");
    }

    return fallbackLeaderboard;
  }
}

export async function reserveUsername(username: string, reservationToken?: string): Promise<UserReservation> {
  try {
    return await requestJson<UserReservation>("/api/users/reserve", {
      method: "POST",
      body: JSON.stringify({
        username,
        ...(reservationToken ? { reservationToken } : {})
      })
    });
  } catch (error) {
    if (error instanceof ApiRequestError) {
      return { username, reserved: false, message: error.message };
    }

    setFallbackState("用户名占用请求没有连上 Go 后端，本局按本地兜底继续。");
    return { username, reserved: true, message: "后端未连接，已使用本地兜底用户名" };
  }
}

export async function submitRun(run: RunSubmission, reservationToken?: string): Promise<RunResult> {
  const serverRun = {
    username: run.username,
    durationMs: run.durationMs,
    maxSingleSpend: run.maxSingleSpend,
    finalBalance: run.finalBalance,
    totalSpent: run.totalSpent,
    totalIncome: run.totalIncome,
    endedBy: run.endedBy,
    chaosSeed: run.chaosSeed,
    ...(run.contentVersion ? { contentVersion: run.contentVersion } : {}),
    ...(reservationToken ? { reservationToken } : {}),
    endingId: run.endingId,
    endingTitle: run.endingTitle,
    endingDetail: run.endingDetail
  };

  try {
    return await requestJson<RunResult>("/api/runs", {
      method: "POST",
      body: JSON.stringify(serverRun)
    });
  } catch (error) {
    if (error instanceof ApiRequestError) {
      return {
        accepted: false,
        message: error.message,
        entry: {
          rank: 0,
          username: run.username,
          durationMs: run.durationMs,
          maxSingleSpend: run.maxSingleSpend
        }
      };
    }

    setFallbackState("成绩提交没有连上 Go 后端，本局成绩只在前端本地展示。");
    return {
      accepted: false,
      message: "后端未连接，成绩只在本地兜底流程里展示",
      entry: {
        rank: 0,
        username: run.username,
        durationMs: run.durationMs,
        maxSingleSpend: run.maxSingleSpend
      }
    };
  }
}
