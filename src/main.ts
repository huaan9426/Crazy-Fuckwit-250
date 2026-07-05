import Phaser from "phaser";
import "./styles.css";
import {
  checkApiHealth,
  getApiConnectionState,
  loadBootstrap,
  loadLeaderboard,
  reserveUsername,
  submitRun,
  type ApiConnectionState
} from "./game/api";
import { AudioDirector } from "./game/audio";
import { GAME_CANVAS_HEIGHT, GAME_CANVAS_WIDTH } from "./game/constants";
import { CheckoutRushScene, type GameFeedEvent, type GameRuntimeState } from "./game/CheckoutRushScene";
import { formatDuration, formatMoney } from "./game/format";
import type { GameBootstrap, LeaderboardEntry, RunResult, RunSubmission, UserReservation } from "./game/types";

const app = document.querySelector<HTMLDivElement>("#app");

if (!app) {
  throw new Error("Missing #app root");
}

app.innerHTML = `
  <main class="arcade-shell">
    <section class="hud-bar" aria-label="游戏状态">
      <div class="brand-block">
        <span>混沌人生</span>
        <h1>极速刷爆 250 万</h1>
      </div>
      <div class="hud-metrics">
        <article>
          <span>余额</span>
          <strong data-balance>¥2,500,000</strong>
        </article>
        <article>
          <span>用时</span>
          <strong data-timer>00:00.00</strong>
        </article>
        <article>
          <span>阶段</span>
          <strong data-stage>01/12</strong>
        </article>
      </div>
      <div class="sound-controls">
        <button class="icon-button" type="button" data-sound-toggle aria-label="开启声音">♪</button>
      </div>
    </section>

    <section class="play-layout">
      <div class="arena-frame">
        <div class="pressure-rail" aria-hidden="true">
          <span data-pressure></span>
        </div>
        <div id="game-canvas" class="game-canvas" aria-label="高压购物支付游戏画布"></div>
        <div class="payment-dock" aria-label="支付操作">
          <button class="pay-button" type="button" data-pay-button>刷 VISA</button>
          <button class="clear-button" type="button" data-clear-cart-button>清空购物车</button>
          <span data-api-status aria-live="polite">API 检测中</span>
        </div>
        <div class="start-layer" data-start-layer>
          <form class="start-panel" data-start-form>
            <span class="start-kicker">READY</span>
            <div class="settlement-report hidden" data-settlement-report aria-live="polite"></div>
            <label for="username">用户名</label>
            <div class="start-row">
              <input id="username" name="username" maxlength="16" autocomplete="off" placeholder="今晚就花完" />
              <button type="submit">开局</button>
            </div>
            <p data-start-error role="status"></p>
          </form>
        </div>
      </div>

      <aside class="side-panel" aria-label="排行榜和实时事件">
        <section class="panel-section balance-flow">
          <div>
            <span>已花</span>
            <strong data-spent>¥0</strong>
          </div>
          <div>
            <span>返钱</span>
            <strong data-income>¥0</strong>
          </div>
        </section>

        <section class="panel-section feed-section">
          <header>
            <h2>事件流水</h2>
            <span data-status>待开局</span>
          </header>
          <div class="event-feed" data-feed></div>
        </section>

        <section class="panel-section leaderboard-section">
          <header>
            <h2>排行榜</h2>
            <span>清零优先</span>
          </header>
          <ol class="leaderboard" data-leaderboard></ol>
        </section>
      </aside>
    </section>
  </main>
`;

const balanceEl = app.querySelector<HTMLElement>("[data-balance]")!;
const timerEl = app.querySelector<HTMLElement>("[data-timer]")!;
const stageEl = app.querySelector<HTMLElement>("[data-stage]")!;
const pressureEl = app.querySelector<HTMLElement>("[data-pressure]")!;
const spentEl = app.querySelector<HTMLElement>("[data-spent]")!;
const incomeEl = app.querySelector<HTMLElement>("[data-income]")!;
const statusEl = app.querySelector<HTMLElement>("[data-status]")!;
const feedEl = app.querySelector<HTMLElement>("[data-feed]")!;
const leaderboardEl = app.querySelector<HTMLElement>("[data-leaderboard]")!;
const startLayerEl = app.querySelector<HTMLElement>("[data-start-layer]")!;
const startForm = app.querySelector<HTMLFormElement>("[data-start-form]")!;
const startButton = startForm.querySelector<HTMLButtonElement>("button[type='submit']")!;
const startErrorEl = app.querySelector<HTMLElement>("[data-start-error]")!;
const settlementReportEl = app.querySelector<HTMLElement>("[data-settlement-report]")!;
const soundToggle = app.querySelector<HTMLButtonElement>("[data-sound-toggle]")!;
const payButton = app.querySelector<HTMLButtonElement>("[data-pay-button]")!;
const clearCartButton = app.querySelector<HTMLButtonElement>("[data-clear-cart-button]")!;
const apiStatusEl = app.querySelector<HTMLElement>("[data-api-status]")!;

const feedEvents: GameFeedEvent[] = [];
let bootstrap: GameBootstrap;
let scene: CheckoutRushScene;
let game: Phaser.Game | null = null;
let audio: AudioDirector;
let soundEnabled = false;
let gameReady = false;
let roundStartInFlight = false;
let currentRoundCanSubmitLeaderboard = false;
let currentRoundSubmissionBlockMessage = "本局不是用 PostgreSQL 内容包开局，战报只保存在本地。";
/*
 * 每次真正开始新一局时都会递增这个数字。它不是游戏内的阶段，也不会提交给后端，只用于
 * 前端界面判断“一个异步回调回来时，它是否还属于用户正在看的这一局”。结算提交会先显示
 * 本地战报，再等待 Go API 返回；如果玩家趁这个等待时间已经点了“再来”，上一局的网络结果
 * 就不能再把旧结算层打开，也不能把旧的提交失败消息塞进新一局的事件流水。
 */
let roundViewRevision = 0;

const RECENT_REPORT_STORAGE_KEY = "crazy-fuckwit-250:recent-report";
const USERNAME_RESERVATION_STORAGE_KEY = "crazy-fuckwit-250:username-reservation";
const MIN_USERNAME_LENGTH = 2;
const MAX_USERNAME_LENGTH = 16;
const FORBIDDEN_USERNAME_PATTERN = /[<>{}\[\]/\\|]/;

type StoredUsernameReservation = {
  username: string;
  reservationToken: string;
};

function renderApiStatus(state: ApiConnectionState = getApiConnectionState()): void {
  apiStatusEl.textContent = state.label;
  apiStatusEl.title = state.detail;
  apiStatusEl.classList.remove("checking", "online", "fallback", "error");
  apiStatusEl.classList.add(state.kind);
}

function renderState(state: GameRuntimeState): void {
  balanceEl.textContent = formatMoney(state.balance);
  timerEl.textContent = formatDuration(state.elapsedMs);
  stageEl.textContent = `${String(state.currentStage).padStart(2, "0")}/${state.totalStages} 段`;
  pressureEl.style.width = `${Math.round(state.pressure * 100)}%`;
  spentEl.textContent = formatMoney(state.totalSpent);
  incomeEl.textContent = `+${formatMoney(state.totalIncome)}`;
  const statusBadge = state.activeStatusName ? `状态 ${state.activeStatusName} ${formatWholeSeconds(state.activeStatusMs)}s · ` : "";
  statusEl.textContent =
    state.status === "running"
      ? `${state.currentSceneName} · ${statusBadge}${state.handFrozen ? "购物车锁定" : state.checkoutLockMs > 0 ? "收银" : "货架"} ${formatWholeSeconds(state.nextHandRefreshMs)}s · 利息 ${formatWholeSeconds(state.nextInterestMs)}s · 硬结算 ${formatShortClock(state.remainingMs)}`
      : state.status === "ended"
        ? formatEndedStatus(state)
        : "待开局";
  renderActionButtons(state);
  syncRunningMusic(state);
}

function formatEndedStatus(state: GameRuntimeState): string {
  switch (state.endedBy) {
    case "balance_zero":
      return "已结算 · 余额归零";
    case "timeout":
      return "已结算 · 硬结算到时";
    case "terminal_event":
      return "已结算 · 特殊终局";
    case "manual":
      return "已结算 · 手动结束";
    default:
      return "已结算";
  }
}

function formatWholeSeconds(ms: number): number {
  return Math.max(0, Math.ceil(ms / 1000));
}

function formatShortClock(ms: number): string {
  const totalSeconds = Math.max(0, Math.ceil(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

function renderActionButtons(state: GameRuntimeState): void {
  const running = state.status === "running";
  const visaPending = state.visaPendingMs > 0;
  const visaCooling = state.visaCooldownMs > 0;
  const clearPending = state.clearPendingMs > 0;
  const clearCooling = state.clearCooldownMs > 0;
  const checkoutBusy = state.handFrozen || state.checkoutLockMs > 0;

  payButton.disabled = !running || checkoutBusy || visaCooling || visaPending || !state.canUseVisa;
  clearCartButton.disabled = !running || checkoutBusy || clearCooling || clearPending || !state.canUseClearCart;

  if (!running) {
    payButton.textContent = "刷 VISA";
    clearCartButton.textContent = "清空购物车";
    return;
  }

  if (visaPending) {
    payButton.textContent = `VISA 扣款 ${formatWholeSeconds(state.visaPendingMs)}s`;
  } else if (state.handFrozen) {
    payButton.textContent = "购物车锁定";
  } else if (state.checkoutLockMs > 0) {
    payButton.textContent = "收银中";
  } else if (visaCooling) {
    payButton.textContent = `VISA 冷却 ${formatWholeSeconds(state.visaCooldownMs)}s`;
  } else {
    payButton.textContent = state.canUseVisa ? "刷 VISA" : "VISA 买不起";
  }

  if (clearPending) {
    clearCartButton.textContent = `购物车结算 ${formatWholeSeconds(state.clearPendingMs)}s`;
  } else if (state.handFrozen) {
    clearCartButton.textContent = "购物车锁定";
  } else if (state.checkoutLockMs > 0) {
    clearCartButton.textContent = "收银中";
  } else if (clearCooling) {
    clearCartButton.textContent = `购物车冷却 ${formatWholeSeconds(state.clearCooldownMs)}s`;
  } else {
    clearCartButton.textContent = state.canUseClearCart ? "清空购物车" : "购物车买不起";
  }
}

function syncRunningMusic(state: GameRuntimeState): void {
  if (!soundEnabled || state.status !== "running") {
    return;
  }

  const shouldUseDangerMusic = state.pressure >= 0.82 || state.activeStatusName !== null;
  audio.playMusic(shouldUseDangerMusic ? "danger" : "rush");
}

function renderFeed(): void {
  feedEl.innerHTML = feedEvents
    .slice(0, 8)
    .map(
      (event) => `
        <article class="feed-item ${event.kind}">
          <span>${escapeHtml(event.title)}</span>
          <p>${escapeHtml(event.detail)}</p>
        </article>
      `
    )
    .join("");
}

function pushFeed(event: GameFeedEvent): void {
  feedEvents.unshift(event);
  renderFeed();
}

function resetRoundFeed(): void {
  /*
   * 事件流水是“本局战报”的一部分，不是全站通知中心。玩家点击“再来”开始新一局时，
   * 旧流水如果继续留在数组里，侧边栏会混着显示上一局事件，结算页和 localStorage 最近
   * 战报也会把上一局的扣款、返钱或提交失败原因保存进去。这里只在新局已经通过用户名
   * 预约、即将真正调用 scene.startRound 时清空，避免预约失败时把上一局战报提前抹掉。
   */
  feedEvents.length = 0;
  renderFeed();
}

function renderLeaderboard(entries: LeaderboardEntry[]): void {
  if (entries.length === 0) {
    leaderboardEl.innerHTML = `
      <li class="leaderboard-empty">
        <span>暂无真实成绩</span>
        <strong>等待数据库排行榜返回。</strong>
      </li>
    `;
    return;
  }

  leaderboardEl.innerHTML = entries
    .slice(0, 8)
    .map(
      (entry) => `
        <li>
          <b>${String(entry.rank).padStart(2, "0")}</b>
          <span>${escapeHtml(entry.username)}</span>
          <em>${formatDuration(entry.durationMs)}</em>
          <strong>${formatMoney(entry.maxSingleSpend)}</strong>
        </li>
      `
    )
    .join("");
}

function renderBootstrapFailure(): void {
  const state = getApiConnectionState();
  startLayerEl.classList.remove("hidden");
  startButton.disabled = true;
  payButton.disabled = true;
  clearCartButton.disabled = true;
  soundToggle.disabled = true;
  startErrorEl.textContent = state.detail || "内容包加载失败，当前不能开局。";
  statusEl.textContent = "内容包加载失败";
  renderLeaderboard([]);
  renderFeed();
}

function escapeHtml(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function resolveSettlementTitle(run: RunSubmission): { title: string; badge: string; detail: string } {
  const savedRatio = run.finalBalance / Math.max(1, bootstrap.config.initialBalance);
  const targetClearMs = bootstrap.config.balanceTuning?.targetClearMs ?? 420_000;
  const hardLimitLabel = formatShortClock(bootstrap.config.roundLimitMs);

  if (run.endedBy === "terminal_event") {
    return {
      title: run.endingTitle ?? "特殊终局",
      badge: "特殊终局",
      detail: run.endingDetail ?? "本局被终局事件提前停表，系统直接生成特殊战报。"
    };
  }

  if (run.endedBy === "balance_zero") {
    return run.durationMs <= targetClearMs
      ? {
          title: "极速清空",
          badge: "主线通关",
          detail: "你在目标节奏内把额度清到了 0，属于真正的刷卡爆发局。"
        }
      : {
          title: "压线清空",
          badge: "主线通关",
          detail: "你最后还是把额度清掉了，但过程已经被利息和返钱拖慢。"
        };
  }

  if (savedRatio >= 1) {
    return {
      title: "越花越有钱",
      badge: "隐藏结局",
      detail: "你没有清空额度，反而靠返钱和利息把余额守住了。"
    };
  }

  if (savedRatio >= 0.72) {
    return {
      title: "省钱专家",
      badge: "隐藏结局",
      detail: `${hardLimitLabel} 硬结算时还攒着大量余额，这不是失败，是另一条少花多挣路线。`
    };
  }

  if (savedRatio >= 0.35) {
    return {
      title: "余额守门员",
      badge: "隐藏结局",
      detail: "你花掉了一部分钱，但没有被消费系统彻底带跑。"
    };
  }

  return {
    title: "差点清空",
    badge: "时间到",
    detail: `你已经接近清空，但 ${hardLimitLabel} 硬时限先到了。`
  };
}

function resolveEndReason(run: RunSubmission): { label: string; detail: string } {
  if (run.endedBy === "balance_zero") {
    return {
      label: "余额归零",
      detail: "本局是因为余额扣到 0 立即结束；12 个阶段只负责切换消费压力，不是结束条件。"
    };
  }

  if (run.endedBy === "timeout") {
    const savedRatio = run.finalBalance / Math.max(1, bootstrap.config.initialBalance);

    return {
      label: `${formatShortClock(bootstrap.config.roundLimitMs)} 硬结算`,
      detail:
        savedRatio >= 0.35
          ? "本局不是 12 阶段结束，而是硬结算时间到。剩余余额较多，所以进入“少花钱、多挣钱”的隐藏反馈。"
          : "本局不是 12 阶段结束，而是硬结算时间到。剩余余额已经不多，所以按时间到或差点清空结算。"
    };
  }

  if (run.endedBy === "terminal_event") {
    return {
      label: run.endingTitle ?? "特殊终局",
      detail: run.endingDetail ?? "本局由终局事件提前停表，余额和流水会按事件发生后的状态提交。"
    };
  }

  return {
    label: "手动结束",
    detail: "本局由手动或外部流程结束，排行榜只保存后端接受的成绩摘要。"
  };
}

function renderSettlementLog(): string {
  const logItems = feedEvents.slice(0, 8);

  if (logItems.length === 0) {
    return `<p class="settlement-empty-log">本局没有记录到流水。</p>`;
  }

  return `
    <ol class="settlement-log">
      ${logItems
        .map(
          (event) => `
            <li class="${event.kind}">
              <span>${escapeHtml(event.title)}</span>
              <p>${escapeHtml(event.detail)}</p>
            </li>
          `
        )
        .join("")}
    </ol>
  `;
}

function renderSettlementHighlights(run: RunSubmission): string {
  const stats = run.settlementStats;

  if (!stats) {
    return "";
  }

  const annoyingIncome = stats.mostAnnoyingIncome;
  const absurdSpend = stats.mostAbsurdSpend;

  /*
   * 这里显示的是 Phaser 场景在本局运行时记录的“战报高光”，不是后端排行榜字段。
   * 排行榜仍只展示用户名、用时、最大单笔和名次；高光用于解释这局为什么烦、为什么荒诞，
   * 让结算页不只是数字表格。
   */
  return `
    <div class="settlement-highlights">
      <article class="income">
        <span>最烦人返钱</span>
        <strong>${annoyingIncome ? `+${formatMoney(annoyingIncome.amount)}` : "无"}</strong>
        <p>${annoyingIncome ? `${escapeHtml(annoyingIncome.title)} · ${escapeHtml(annoyingIncome.detail)}` : "本局没有明显返钱阻碍。"}</p>
      </article>
      <article class="spend">
        <span>最荒诞扣款</span>
        <strong>${absurdSpend ? `-${formatMoney(absurdSpend.amount)}` : "无"}</strong>
        <p>${absurdSpend ? `${escapeHtml(absurdSpend.title)} · ${escapeHtml(absurdSpend.detail)}` : "本局没有记录到消费扣款。"}</p>
      </article>
      <article class="tempo">
        <span>节奏统计</span>
        <strong>${stats.paymentCount} 次结算</strong>
        <p>${stats.interestCount} 次利息，${stats.eventCount} 次金额事件，${stats.spendCount} 笔扣款，${stats.incomeCount} 笔入账。</p>
      </article>
    </div>
  `;
}

function renderSettlementTimeline(run: RunSubmission): string {
  const tuning = bootstrap.config.balanceTuning;
  const stageCount = Math.max(1, tuning?.stageCount ?? 12);
  const stageDurationMs = Math.max(1, tuning?.stageDurationMs ?? 35_000);
  const targetClearMs = Math.max(1, tuning?.targetClearMs ?? 420_000);
  const completedStage = Math.min(stageCount, Math.max(1, Math.ceil(run.durationMs / stageDurationMs)));
  const savedRatio = run.finalBalance / Math.max(1, bootstrap.config.initialBalance);
  const endNote =
    run.endedBy === "balance_zero"
      ? "余额归零后立即停表，阶段不会继续推进。"
      : run.endedBy === "timeout"
        ? savedRatio >= 0.35
          ? "硬结算到时仍有大量余额，所以进入省钱路线反馈。"
          : "硬结算时间先到，按时间到战报提交。"
        : run.endedBy === "terminal_event"
          ? "终局事件会提前停表，即使余额还没清空也会结算。"
          : "本局由外部流程结束，按当前摘要提交。";

  /*
   * 这段时间线专门解释“阶段”和“结束条件”的区别。12 个阶段只负责改变货架压力和场景，
   * 不是第 12 段结束就自动判负；真正自动停表的是 roundLimitMs 硬结算，特殊终局则是
   * 事件提前停表。把这几条放到结算页，能避免用户看到 12/12 段后误以为游戏突然结束。
   */
  return `
    <div class="settlement-timeline">
      <article>
        <span>阶段进度</span>
        <strong>${completedStage}/${stageCount} 段</strong>
        <p>阶段只切换消费压力，不是结束条件。</p>
      </article>
      <article>
        <span>主线目标</span>
        <strong>${formatShortClock(targetClearMs)}</strong>
        <p>目标是约 7 分钟清空；超过目标仍可继续。</p>
      </article>
      <article>
        <span>本局停表</span>
        <strong>${formatDuration(run.durationMs)}</strong>
        <p>${escapeHtml(endNote)}</p>
      </article>
    </div>
  `;
}

function renderSettlementReport(run: RunSubmission, result: RunResult): void {
  const outcome = resolveSettlementTitle(run);
  const endReason = resolveEndReason(run);
  const netSpent = Math.max(0, run.totalSpent - run.totalIncome);
  const savedRatio = Math.round((run.finalBalance / Math.max(1, bootstrap.config.initialBalance)) * 100);
  const rankText = result.accepted && result.entry.rank > 0 ? `#${result.entry.rank}` : "未上榜";
  const targetClearMs = bootstrap.config.balanceTuning?.targetClearMs ?? 420_000;

  /*
   * 结算战报只使用本局已经提交给后端的摘要字段，不额外读取 Phaser 内部状态。这样用户看到
   * 的结算、排行榜提交和后端保存的数据来自同一份 run payload，避免“画面显示一套、提交
   * 又是另一套”的不一致。
   */
  settlementReportEl.classList.remove("hidden");
  settlementReportEl.innerHTML = `
    <div class="settlement-heading">
      <span>${escapeHtml(outcome.badge)}</span>
      <h2>${escapeHtml(outcome.title)}</h2>
      <p>${escapeHtml(outcome.detail)}</p>
    </div>
    <p class="settlement-reason">${escapeHtml(endReason.detail)}</p>
    ${renderSettlementTimeline(run)}
    <div class="settlement-grid">
      <article><span>结束原因</span><strong>${escapeHtml(endReason.label)}</strong></article>
      <article><span>用时</span><strong>${formatDuration(run.durationMs)}</strong></article>
      <article><span>主线目标</span><strong>${formatShortClock(targetClearMs)}</strong></article>
      <article><span>硬结算线</span><strong>${formatShortClock(bootstrap.config.roundLimitMs)}</strong></article>
      <article><span>剩余</span><strong>${formatMoney(run.finalBalance)}</strong></article>
      <article><span>保住比例</span><strong>${savedRatio}%</strong></article>
      <article><span>已花</span><strong>${formatMoney(run.totalSpent)}</strong></article>
      <article><span>返钱/利息</span><strong>+${formatMoney(run.totalIncome)}</strong></article>
      <article><span>净消费</span><strong>${formatMoney(netSpent)}</strong></article>
      <article><span>最大单笔</span><strong>${formatMoney(run.maxSingleSpend)}</strong></article>
    </div>
    ${renderSettlementHighlights(run)}
    <div class="settlement-log-block">
      <h3>最终流水</h3>
      ${renderSettlementLog()}
    </div>
    <p class="settlement-footnote">排行榜：${escapeHtml(rankText)}。${result.accepted ? "成绩已提交到 Go API。" : escapeHtml(result.message ?? "成绩提交失败。")}</p>
  `;
}

function saveRecentReport(run: RunSubmission, result: RunResult): void {
  try {
    /*
     * 本地战报只保存最近一局的摘要和最终流水，方便后续做分享弹层或“最近战报”入口。
     * localStorage 是浏览器自己的小型键值存储，不参与后端排行榜排序，也不能当作可信成绩。
     * 真实排行榜仍然只相信 Go API 写入数据库后的结果。
     */
    localStorage.setItem(
      RECENT_REPORT_STORAGE_KEY,
      JSON.stringify({
        savedAt: new Date().toISOString(),
        run,
        result,
        feed: feedEvents.slice(0, 12)
      })
    );
  } catch {
    pushFeed({
      id: crypto.randomUUID(),
      title: "本地战报未保存",
      detail: "浏览器拒绝写入 localStorage，不影响排行榜提交",
      kind: "system"
    });
  }
}

function readUsernameReservationToken(username: string): string | undefined {
  try {
    /*
     * 用户名预约 token 只解决“刷新页面后还能续用自己刚预约的名字”这个本地体验问题。
     * 它不是登录凭证，也不参与排行榜可信排序；真正能让用户名永久占用的仍然是 Go API
     * 成功写入 runs 表后的成绩记录。
     */
    const raw = sessionStorage.getItem(USERNAME_RESERVATION_STORAGE_KEY);
    if (!raw) {
      return undefined;
    }

    const stored = JSON.parse(raw) as Partial<StoredUsernameReservation>;
    if (stored.username === username && typeof stored.reservationToken === "string" && stored.reservationToken.length > 0) {
      return stored.reservationToken;
    }
  } catch {
    sessionStorage.removeItem(USERNAME_RESERVATION_STORAGE_KEY);
  }

  return undefined;
}

function rememberUsernameReservation(reservation: UserReservation): void {
  if (!reservation.reserved || !reservation.reservationToken) {
    return;
  }

  try {
    sessionStorage.setItem(
      USERNAME_RESERVATION_STORAGE_KEY,
      JSON.stringify({
        username: reservation.username,
        reservationToken: reservation.reservationToken
      } satisfies StoredUsernameReservation)
    );
  } catch {
    // 浏览器拒绝 sessionStorage 时，下一次刷新只是不能续租预约，不影响本局开局。
  }
}

function clearUsernameReservation(username: string): void {
  try {
    const raw = sessionStorage.getItem(USERNAME_RESERVATION_STORAGE_KEY);
    if (!raw) {
      return;
    }

    const stored = JSON.parse(raw) as Partial<StoredUsernameReservation>;
    if (stored.username === username) {
      sessionStorage.removeItem(USERNAME_RESERVATION_STORAGE_KEY);
    }
  } catch {
    sessionStorage.removeItem(USERNAME_RESERVATION_STORAGE_KEY);
  }
}

async function refreshLeaderboard(): Promise<boolean> {
  const entries = await loadLeaderboard(bootstrap?.config.contentVersion);
  const state = getApiConnectionState();

  renderLeaderboard(entries);
  renderApiStatus(state);

  /*
   * loadLeaderboard 遇到 Go API 返回的结构化 JSON 错误时，会返回空榜并把 API 状态改成 error，
   * 而不是抛异常。调用方如果只写 catch，就分不清“真实榜单刷新成功但没有成绩”和“榜单读取
   * 失败所以只能显示空列表”。这里把状态转换成布尔值，结算尾段可以把失败写进最终流水；开局
   * 流程也能把排行榜当作旁路展示，不再因为榜单读不到就阻止玩家进入本局。
   */
  return state.kind === "online";
}

function remountGameWithBootstrap(nextBootstrap: GameBootstrap): void {
  /*
   * CheckoutRushScene 在构造函数里接收 GameBootstrap，后续抽卡、事件、状态、音轨入口和
   * balanceTuning 都围绕这份内容包运行。页面如果先用本地/Go 内存兜底内容启动，后来又在
   * 开局前恢复到 PostgreSQL，就不能只把 API 状态改成“已接通”，还继续让旧 Scene 拿着小
   * 样例卡池开局。这里销毁旧 Phaser 实例和旧音频控制器，再用新的数据库内容包重建场景，
   * 保证玩家看到的状态、排行榜和真正进入游戏的卡池来自同一条后端链路。
   */
  game?.destroy(true);
  game = null;
  audio.dispose();
  bootstrap = nextBootstrap;
  audio = new AudioDirector(bootstrap.audioTracks);
  mountGame();
}

async function refreshBootstrapAfterFallbackRecovery(previousApiState: ApiConnectionState): Promise<boolean> {
  if (previousApiState.kind !== "fallback" || getApiConnectionState().kind !== "online") {
    return true;
  }

  startButton.textContent = "同步内容";

  try {
    const nextBootstrap = await loadBootstrap();
    renderApiStatus();
    remountGameWithBootstrap(nextBootstrap);
    return true;
  } catch {
    renderApiStatus();
    startErrorEl.textContent = getApiConnectionState().detail || "Go API 已恢复，但数据库内容包重新加载失败。";
    return false;
  }
}

function createLocalRunResult(run: RunSubmission, message: string): RunResult {
  /*
   * 这个结果不是后端成绩，只是前端在提交未完成或提交失败时给结算页使用的本地占位。
   * 它使用同一份 run payload 里的用户名、用时和最大单笔消费，是为了让玩家立刻看到本局
   * 战报，同时仍然把“是否上榜”和“真实名次”交给 Go API 的最终返回值决定。
   */
  return {
    accepted: false,
    message,
    entry: {
      rank: 0,
      username: run.username,
      durationMs: run.durationMs,
      maxSingleSpend: run.maxSingleSpend
    }
  };
}

type RoundSubmissionPolicyContext = {
  apiStateBeforeReservation: ApiConnectionState;
};

function rememberRoundSubmissionPolicy(state: ApiConnectionState, context: RoundSubmissionPolicyContext): void {
  /*
   * 成绩是否能进入真实排行榜，取决于“这一局开局时”使用的内容包，而不是结算那一刻
   * API 是否刚好恢复。玩家可能先在本地兜底或 Go 内存兜底内容里开局，途中 PostgreSQL
   * 恢复可用；如果结算时再直接提交，就会把非数据库内容包跑出来的成绩写进真实榜单。
   * 所以这里把开局时的 API 状态拍成快照：只有 database:"online" 对应的 online 状态
   * 才允许提交排行榜，其它兜底局都只保留本地战报。
   *
   * 还有一种更细的边界：页面已经用 PostgreSQL 内容包完成初始化，但玩家点击开局时，
   * 用户名预约请求刚好断线。这个时候卡池不是本地兜底内容，仍然不能提交真实排行榜，因为
   * 后端没有确认这个用户名属于当前浏览器会话。这里用预约前后的状态差异把原因说清楚，
   * 避免结算页把“预约链路断开”误说成“用了本地兜底数据”。
   */
  currentRoundCanSubmitLeaderboard = state.kind === "online";
  if (currentRoundCanSubmitLeaderboard) {
    currentRoundSubmissionBlockMessage = "";
    return;
  }

  if (context.apiStateBeforeReservation.kind === "online") {
    currentRoundSubmissionBlockMessage =
      "本局开局前已经加载 PostgreSQL 内容包，但用户名预约没有连上 Go API；为了避免未预约成绩写入真实排行榜，本局战报只保存在本地。请在 API 恢复后重新开局冲榜。";
    return;
  }

  currentRoundSubmissionBlockMessage = `本局开局时使用${state.label}，未提交真实排行榜；请用 PostgreSQL 内容重新开局后再冲榜。`;
}

function revealRoundEnd(run: RunSubmission, result: RunResult): void {
  saveRecentReport(run, result);
  renderSettlementReport(run, result);

  /*
   * 结算报告在 DOM 里属于 startLayer。这里显示 startLayer 的职责很单纯：让玩家看到战报，
   * 并且能马上点击“再来”。排行榜刷新和成绩提交都是网络动作，不能把这个基础交互卡住。
   */
  startLayerEl.classList.remove("hidden");
  startButton.disabled = false;
  startButton.textContent = "再来";
}

async function handleRoundEnd(run: RunSubmission): Promise<void> {
  const endedRoundRevision = roundViewRevision;
  const canSubmitLeaderboard = currentRoundCanSubmitLeaderboard;
  const submissionBlockMessage = currentRoundSubmissionBlockMessage;
  revealRoundEnd(
    run,
    createLocalRunResult(run, canSubmitLeaderboard ? "成绩提交中，战报已先保存在本地。" : submissionBlockMessage)
  );
  audio.playMusic("settlement");

  let result: RunResult;
  if (canSubmitLeaderboard) {
    try {
      result = await submitRun(run, readUsernameReservationToken(run.username));
    } catch {
      result = createLocalRunResult(run, "成绩提交时出现未知异常，本局战报仍保存在本地。");
    }
  } else {
    result = createLocalRunResult(run, submissionBlockMessage);
  }
  renderApiStatus();
  if (result.accepted) {
    clearUsernameReservation(run.username);
  }

  if (endedRoundRevision !== roundViewRevision) {
    /*
     * 这里说明玩家已经在上一局成绩提交完成前开始了新一局。旧提交如果成功，排行榜可以静默刷新；
     * 但结算面板、最近战报和事件流水都属于玩家当前操作的界面，继续渲染旧结果会造成“玩着玩着
     * 被上一局结算弹回来”的错觉。失败消息也不写入流水，因为那会混进新一局的事件列表。
     */
    if (result.accepted) {
      try {
        await refreshLeaderboard();
      } catch {
        renderApiStatus();
      }
    }
    return;
  }

  if (!result.accepted) {
    pushFeed({
      id: crypto.randomUUID(),
      title: "成绩未提交",
      detail: result.message ?? "后端拒绝了这局成绩",
      kind: "system"
    });
  }
  revealRoundEnd(run, result);

  let leaderboardRefreshed = false;
  try {
    leaderboardRefreshed = await refreshLeaderboard();
  } catch {
    renderApiStatus();
  }
  if (leaderboardRefreshed) {
    return;
  }
  if (endedRoundRevision !== roundViewRevision) {
    renderApiStatus();
    return;
  }

  pushFeed({
    id: crypto.randomUUID(),
    title: "排行榜刷新失败",
    detail: "战报已经生成，排行榜稍后刷新不影响再开一局",
    kind: "system"
  });
  renderApiStatus();
  /*
   * 排行榜刷新失败发生在成绩提交之后，但它仍然是本局结算流程的最后一个系统状态。
   * 如果只写侧边栏流水，不重新保存和渲染结算报告，玩家看到的“最终流水”和 localStorage
   * 最近战报会漏掉这条信息；如果玩家已经开了新局，上面的 revision 检查会先返回，避免旧局
   * 的网络结果污染新局流水。
   */
  revealRoundEnd(run, result);
}

function mountGame(): void {
  scene = new CheckoutRushScene(bootstrap, {
    onStateChange: renderState,
    onFeedEvent: pushFeed,
    onTone: (kind) => audio.playPaymentTone(kind),
    onRoundEnd: (run) => {
      void handleRoundEnd(run);
    }
  });

  game = new Phaser.Game({
    type: Phaser.AUTO,
    parent: "game-canvas",
    width: GAME_CANVAS_WIDTH,
    height: GAME_CANVAS_HEIGHT,
    backgroundColor: "#050505",
    scale: {
      mode: Phaser.Scale.RESIZE,
      autoCenter: Phaser.Scale.CENTER_BOTH
    },
    scene
  });
}

function normalizeUsername(value: string): string {
  return value.trim().replace(/\s+/g, " ");
}

function validateUsername(username: string): string | null {
  const usernameLength = Array.from(username).length;
  if (usernameLength < MIN_USERNAME_LENGTH || usernameLength > MAX_USERNAME_LENGTH) {
    return "用户名需要 2 到 16 个字";
  }

  /*
   * 前端校验不是安全边界，真正的边界仍然在 Go API 和数据库约束里。这里提前拦下这些
   * 字符，是为了让在线和离线兜底路径看到同一套用户名规则：尖括号、斜杠和管道这类符号
   * 很容易在排行榜、日志、URL 或后续分享文案里造成歧义，所以首版直接不允许作为用户名。
   */
  if (FORBIDDEN_USERNAME_PATTERN.test(username)) {
    return "用户名不能包含尖括号、斜杠、方括号或管道符";
  }

  return null;
}

async function prepareRoundAudio(): Promise<void> {
  if (!soundEnabled) {
    audio.setEnabled(false);
    audio.playMusic("rush");
    return;
  }

  try {
    await audio.unlock();
  } catch {
    /*
     * Web Audio 可能因为浏览器策略、设备限制或异常环境拒绝创建/恢复 AudioContext。
     * 声音失败不能挡住开局，否则玩家点“开局”后会卡在开始层，看起来像用户名预约或
     * 后端接口出了问题。这里把声音降级为关闭状态，游戏主流程继续走。
     */
    soundEnabled = false;
    soundToggle.textContent = "♪";
    soundToggle.setAttribute("aria-label", "开启声音");
  }
  audio.setEnabled(soundEnabled);
  audio.playMusic("rush");
}

soundToggle.addEventListener("click", async () => {
  if (!gameReady) {
    return;
  }

  const nextEnabled = !soundEnabled;
  if (nextEnabled) {
    try {
      await audio.unlock();
    } catch {
      soundEnabled = false;
      audio.setEnabled(false);
      soundToggle.textContent = "♪";
      soundToggle.setAttribute("aria-label", "开启声音");
      return;
    }
  }

  soundEnabled = nextEnabled;
  audio.setEnabled(soundEnabled);
  audio.playMusic("rush");
  soundToggle.textContent = soundEnabled ? "♫" : "♪";
  soundToggle.setAttribute("aria-label", soundEnabled ? "关闭声音" : "开启声音");
});

startForm.addEventListener("submit", async (event) => {
  event.preventDefault();

  if (roundStartInFlight) {
    return;
  }

  if (!gameReady) {
    startErrorEl.textContent = getApiConnectionState().detail || "内容包尚未加载完成，暂时不能开局。";
    return;
  }

  const formData = new FormData(startForm);
  const username = normalizeUsername(String(formData.get("username") ?? ""));
  const usernameError = validateUsername(username);

  if (usernameError) {
    startErrorEl.textContent = usernameError;
    return;
  }

  startErrorEl.textContent = "";
  roundStartInFlight = true;
  const previousStartText = startButton.textContent;
  startButton.disabled = true;
  startButton.textContent = "准备中";

  try {
    const apiStateBeforeReservation = getApiConnectionState();
    const reservation = await reserveUsername(username, readUsernameReservationToken(username));
    renderApiStatus();

    if (!reservation.reserved) {
      startErrorEl.textContent = reservation.message ?? "用户名已被占用";
      return;
    }
    rememberUsernameReservation(reservation);
    let roundSubmissionState = getApiConnectionState();

    if (getApiConnectionState().kind === "online") {
      /*
       * 如果页面最初是本地或 Go 内存兜底，后来在用户名占用这一步重新连上 PostgreSQL，
       * 当前用户名已经被后端接受，所以先把预约 token 存好，再刷新内容包和重建 Phaser
       * 场景。这样即使数据库内容包在这一步失败，玩家刷新页面或重试时仍能带着自己的 token
       * 续租同一个名字，不会把刚预约到的用户名变成自己也无法继续使用的短期占用。
       */
      if (!(await refreshBootstrapAfterFallbackRecovery(apiStateBeforeReservation))) {
        return;
      }
      /*
       * 本局是否可以提交真实排行榜，要按“开局内容包来自哪里”判断，而不能按后续排行榜
       * 刷新是否成功判断。排行榜刷新只是右侧展示动作，可能因为读取榜单超时或后端返回
       * 临时错误把 API 状态改成 error；但只要用户名预约和内容包都已经走 PostgreSQL，
       * 这一局仍然应该尝试提交真实成绩，真正的提交成功与否交给 /api/runs 再决定。
       */
      roundSubmissionState = getApiConnectionState();
      try {
        await refreshLeaderboard();
      } catch {
        /*
         * 排行榜是开局前的旁路展示，不是本局玩法的前置条件。用户名预约和数据库内容包已经
         * 成功时，就应该允许玩家进入游戏；失败状态会留在 API 徽标里，结算时是否能提交成绩
         * 仍由 /api/runs 单独决定。
         */
        renderApiStatus();
      }
    }

    await prepareRoundAudio();
    settlementReportEl.classList.add("hidden");
    settlementReportEl.innerHTML = "";
    startLayerEl.classList.add("hidden");
    blurActiveElement();
    resetRoundFeed();
    roundViewRevision += 1;
    rememberRoundSubmissionPolicy(roundSubmissionState, { apiStateBeforeReservation });
    scene.startRound(username);
  } finally {
    roundStartInFlight = false;
    startButton.disabled = false;
    startButton.textContent = previousStartText;
  }
});

payButton.addEventListener("click", () => {
  if (!gameReady) {
    return;
  }

  scene.authorizeVisa();
});

clearCartButton.addEventListener("click", () => {
  if (!gameReady) {
    return;
  }

  scene.clearCart();
});

function eventTargetIsTextEntry(target: EventTarget | null): boolean {
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    (target instanceof HTMLElement && target.isContentEditable)
  );
}

function blurActiveElement(): void {
  const activeElement = document.activeElement;
  if (activeElement instanceof HTMLElement) {
    activeElement.blur();
  }
}

window.addEventListener("keydown", (event) => {
  if (event.code === "Space") {
    if (eventTargetIsTextEntry(event.target)) {
      return;
    }

    event.preventDefault();
    /*
     * 空格键是 VISA 的键盘快捷入口，但键盘事件不会像真实按钮点击那样自动尊重 disabled
     * 状态。这里直接复用按钮当前状态：当页面已经显示“收银中”“冷却中”“买不起”或游戏未运行
     * 时，空格也不应该继续调用 Scene 方法、写入重复的“收银忙碌”流水。真正的业务边界仍在
     * CheckoutRushScene.authorizeVisa 里，这里只是让键盘入口和可见按钮保持同一套交互语义。
     */
    if (!gameReady || payButton.disabled) {
      return;
    }

    scene.authorizeVisa();
  }
});

async function initializeApp(): Promise<void> {
  await checkApiHealth();
  renderApiStatus();

  try {
    bootstrap = await loadBootstrap();
  } catch {
    renderApiStatus();
    renderBootstrapFailure();
    return;
  }

  renderApiStatus();
  audio = new AudioDirector(bootstrap.audioTracks);
  mountGame();
  gameReady = true;
  renderFeed();
  await refreshLeaderboard();
}

await initializeApp();

window.addEventListener("beforeunload", () => {
  game?.destroy(true);
});
