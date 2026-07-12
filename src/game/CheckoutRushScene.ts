import Phaser from "phaser";
import { DEFAULT_BALANCE_TUNING } from "./constants";
import { formatDuration, formatMoney } from "./format";
import { findItemArtwork, listItemArtworkAssets } from "./itemArtwork";
import type {
  BalanceTuning,
  GameBootstrap,
  GameEvent,
  Item,
  MultiplierRule,
  RoundSettlementStats,
  RunSubmission,
  Scene,
  SettlementHighlight,
  StatusEffect,
  TerminalEvent
} from "./types";

type RuntimeStatus = "ready" | "running" | "ended";

export type GameRuntimeState = {
  status: RuntimeStatus;
  balance: number;
  totalSpent: number;
  totalIncome: number;
  maxSingleSpend: number;
  elapsedMs: number;
  remainingMs: number;
  pressure: number;
  currentStage: number;
  totalStages: number;
  currentSceneName: string;
  nextInterestMs: number;
  nextHandRefreshMs: number;
  handFrozen: boolean;
  checkoutLockMs: number;
  visaCooldownMs: number;
  visaPendingMs: number;
  clearCooldownMs: number;
  clearPendingMs: number;
  canUseVisa: boolean;
  canUseClearCart: boolean;
  pendingPaymentCount: number;
  activeStatusName: string | null;
  activeStatusMs: number;
  username: string;
  endedBy: RunSubmission["endedBy"] | null;
};

export type GameFeedEvent = {
  id: string;
  title: string;
  detail: string;
  kind: "spend" | "income" | "system";
};

type SceneCallbacks = {
  onStateChange: (state: GameRuntimeState) => void;
  onFeedEvent: (event: GameFeedEvent) => void;
  onTone: (kind: "spend" | "income" | "danger") => void;
  onRoundEnd: (run: RunSubmission) => void;
};

type CardSlot = {
  id: string;
  item: Item;
  multiplier: MultiplierRule;
  container: Phaser.GameObjects.Container | null;
  selected: boolean;
  hitBounds: Phaser.Geom.Rectangle;
  x: number;
  y: number;
};

type PaymentLine = {
  item: Item;
  multiplier: MultiplierRule;
};

type PendingPayment = {
  id: string;
  executeAt: number;
  actionLabel: string;
  lines: PaymentLine[];
  scene: Scene;
  refreshHandAfterPayment: boolean;
};

type ActiveStatus = {
  effect: StatusEffect;
  expiresAt: number;
};

type DebugCardSnapshot = {
  id: string;
  itemId: string;
  name: string;
  total: number;
  price: number;
  tier: Item["tier"];
  tags: string[];
  multiplier: string;
  x: number;
  y: number;
  selected: boolean;
  affordable: boolean;
  artwork: "loaded" | "fallback" | "unmapped";
};

type DebugSnapshot = {
  state: GameRuntimeState;
  targetSpend: number;
  cards: DebugCardSnapshot[];
};

declare global {
  interface Window {
    __checkoutRushDebug?: () => DebugSnapshot;
  }
}

const TIER_WEIGHT: Record<Item["tier"], number> = {
  coin: 0.92,
  small: 1,
  daily: 1.12,
  premium: 0.9,
  large: 0.54,
  heavy: 0.18,
  shock: 0.12,
  income: 0.68
};

const TIER_COLORS: Record<
  Item["tier"],
  { fill: number; stroke: number; highlight: number; shadow: number; glow: number; text: string }
> = {
  coin: { fill: 0x172028, stroke: 0x9fb9c7, highlight: 0xf1fbff, shadow: 0x24323b, glow: 0x74d9ff, text: "#edf6fb" },
  small: { fill: 0x14251f, stroke: 0x8fd5bd, highlight: 0xedfff8, shadow: 0x17372f, glow: 0x6fffd5, text: "#effbf7" },
  daily: { fill: 0x2a2112, stroke: 0xe4b94e, highlight: 0xfff1b8, shadow: 0x5b3a0c, glow: 0xffcc55, text: "#fff6de" },
  premium: { fill: 0x2b1d17, stroke: 0xe09b62, highlight: 0xffe0c2, shadow: 0x5b2b18, glow: 0xff9c61, text: "#fff0df" },
  large: { fill: 0x241927, stroke: 0xc79ae8, highlight: 0xf4dcff, shadow: 0x472653, glow: 0xd57cff, text: "#f7ecff" },
  heavy: { fill: 0x2b171b, stroke: 0xdb7f88, highlight: 0xffd8d5, shadow: 0x5f2028, glow: 0xff6676, text: "#fff0ec" },
  shock: { fill: 0x2c160f, stroke: 0xf0b23f, highlight: 0xffefb4, shadow: 0x761d14, glow: 0xff4d32, text: "#fff7d7" },
  income: { fill: 0x0d2b1b, stroke: 0x55f39a, highlight: 0xdffff0, shadow: 0x0c4c2a, glow: 0x41ff91, text: "#d8ffe7" }
};

/*
 * 这些时间和概率常量直接对应当前玩法规则。它们放在文件顶部，是为了避免把
 * “9 张卡、阶段长度、利息间隔、VISA 延迟、技能冷却”这些关键规则现在优先来自
 * Go 后端返回的 balanceTuning。初学者读这里时，可以先把它理解成一张规则表：
 * 下面的状态更新、按钮禁用、事件流水和结算逻辑，都会围绕这张表工作。
 */
const CARD_COUNT = 9;
const CARD_COLUMNS = 3;
const CARD_ROWS = 3;
const STATE_EMIT_INTERVAL_MS = 80;
const CARD_REFRESH_FLASH_MS = 180;
const CARD_DETAIL_FLIP_MS = 220;
const CARD_DETAIL_EXIT_MS = 260;
const CARD_DETAIL_MIN_EXIT_DELAY_MS = 900;
const EXPECTED_DECISION_MS = 3_200;
const MAX_NORMAL_HIGH_CARD_HAND_CHANCE = 0.1;
const MIN_BEHIND_RATIO_FOR_HIGH_PACING_ASSIST = 0.28;
const MIN_ELAPSED_RATIO_FOR_HIGH_PACING_ASSIST = 0.7;
const PAYMENT_LANE_X_RATIO = 0.5;
const PAYMENT_LANE_Y_RATIO = 0.83;
const VISA_PAYMENT_LABEL = "VISA 延迟扣款";
const CLEAR_CART_PAYMENT_LABEL = "清空购物车";
const ARTWORK_INSET_PX = 6;
const ARTWORK_LABEL_BACKGROUND = "rgba(5, 5, 5, 0.7)";
const ARTWORK_PRICE_BACKGROUND = "rgba(5, 5, 5, 0.76)";
const CARD_BACKPLATE_OVERHANG_PX = 5;
const CARD_BACKPLATE_DROP_PX = 5;

function resolveBalanceTuning(bootstrap: GameBootstrap): BalanceTuning {
  const remote = bootstrap.config.balanceTuning;
  if (!remote) {
    return DEFAULT_BALANCE_TUNING;
  }

  return {
    ...DEFAULT_BALANCE_TUNING,
    ...remote,
    interestBands: remote.interestBands?.length ? remote.interestBands : DEFAULT_BALANCE_TUNING.interestBands,
    multiplierRules: remote.multiplierRules?.length ? remote.multiplierRules : DEFAULT_BALANCE_TUNING.multiplierRules
  };
}

function createInitialState(bootstrap: GameBootstrap, tuning: BalanceTuning): GameRuntimeState {
  const totalStages = Math.max(1, tuning.stageCount);

  return {
    status: "ready",
    balance: bootstrap.config.initialBalance,
    totalSpent: 0,
    totalIncome: 0,
    maxSingleSpend: 0,
    elapsedMs: 0,
    remainingMs: bootstrap.config.roundLimitMs,
    pressure: 0,
    currentStage: 1,
    totalStages,
    currentSceneName: "待开局",
    nextInterestMs: tuning.interestStartDelayMs + tuning.interestIntervalMs,
    nextHandRefreshMs: tuning.handRefreshMs,
    handFrozen: false,
    checkoutLockMs: 0,
    visaCooldownMs: 0,
    visaPendingMs: 0,
    clearCooldownMs: 0,
    clearPendingMs: 0,
    canUseVisa: false,
    canUseClearCart: false,
    pendingPaymentCount: 0,
    activeStatusName: null,
    activeStatusMs: 0,
    username: "",
    endedBy: null
  };
}

function createEmptySettlementStats(): RoundSettlementStats {
  return {
    mostAnnoyingIncome: null,
    mostAbsurdSpend: null,
    paymentCount: 0,
    spendCount: 0,
    incomeCount: 0,
    eventCount: 0,
    interestCount: 0
  };
}

function cloneSettlementStats(stats: RoundSettlementStats): RoundSettlementStats {
  return {
    mostAnnoyingIncome: stats.mostAnnoyingIncome ? { ...stats.mostAnnoyingIncome } : null,
    mostAbsurdSpend: stats.mostAbsurdSpend ? { ...stats.mostAbsurdSpend } : null,
    paymentCount: stats.paymentCount,
    spendCount: stats.spendCount,
    incomeCount: stats.incomeCount,
    eventCount: stats.eventCount,
    interestCount: stats.interestCount
  };
}

function isHighSpend(item: Item, highPriceThreshold = DEFAULT_BALANCE_TUNING.highPriceThreshold): boolean {
  return item.tier !== "income" && (item.tier === "heavy" || item.tier === "shock" || item.price >= highPriceThreshold);
}

function isSpend(item: Item): boolean {
  return item.tier !== "income";
}

function cardTitleFontSize(itemName: string, cardWidth: number): string {
  /*
   * 中文商品名通常没有空格，窄屏上的一张卡只有约 100 像素宽。只设置 wordWrapWidth 时，
   * 字号仍可能大到两行都放不下，标题就会越过相邻卡片。这里先按字符数估算“两行能容纳的
   * 最大字号”，再由 Phaser 的 advanced word wrap 做真实断行；桌面端仍保留 18px 上限，
   * 移动端遇到长模板名时才逐步缩小，最小 9px 避免文字小到完全无法辨认。
   */
  const maximumSize = cardWidth < 150 ? 14 : 18;
  const glyphCount = Math.max(1, Array.from(itemName).length);
  const availableWidth = Math.max(40, cardWidth - 22);
  const sizeThatFitsTwoLines = Math.floor((availableWidth * 2) / glyphCount);

  return `${Phaser.Math.Clamp(sizeThatFitsTwoLines, 9, maximumSize)}px`;
}

/**
 * 这个 Scene 是当前游戏的一局。它现在不再使用“商品持续移动然后连点”的玩法，
 * 而是按开发计划里的货架思路：一次展示 9 张商品卡，玩家从中选一张，
 * 结算后马上刷新下一组。这样节奏来自“在一组消费里做选择”，而不是靠
 * 连续点击同一个按钮快速清空余额。
 */
export class CheckoutRushScene extends Phaser.Scene {
  private state: GameRuntimeState;
  private cards: CardSlot[] = [];
  private pendingPayments: PendingPayment[] = [];
  private eventCooldownUntil = new Map<string, number>();
  private cardDetailOverlay: Phaser.GameObjects.Container | null = null;
  private paymentGraphics!: Phaser.GameObjects.Graphics;
  private arenaGraphics!: Phaser.GameObjects.Graphics;
  private handTimerGraphics!: Phaser.GameObjects.Graphics;
  private handTimerText!: Phaser.GameObjects.Text;
  private stateEmitAccumulator = 0;
  private readonly tuning: BalanceTuning;
  private startedAt = 0;
  private nextInterestAt = 0;
  private nextHandRefreshAt = 0;
  private handFrozenUntil = 0;
  private checkoutLockedUntil = 0;
  private waitingForCheckoutRefresh = false;
  private visaAvailableAt = 0;
  private clearCartAvailableAt = 0;
  private activeStatus: ActiveStatus | null = null;
  private activeStage = 1;
  private activeScene: Scene | null = null;
  private chargedSceneEntryStages = new Set<number>();
  private chaosSeed = "";
  private sceneRotationOffset = 0;
  private settlementStats = createEmptySettlementStats();
  private purchasedItemCounts = new Map<string, number>();
  private pendingItemCounts = new Map<string, number>();
  private callbacks: SceneCallbacks;
  private readonly handleNativeCanvasPointerDown = (event: PointerEvent): void => {
    if (event.pointerType === "mouse" && event.button !== 0) {
      return;
    }

    const canvas = this.game.canvas;
    const bounds = canvas.getBoundingClientRect();
    const scaleX = this.scale.width / Math.max(1, bounds.width);
    const scaleY = this.scale.height / Math.max(1, bounds.height);
    const x = (event.clientX - bounds.left) * scaleX;
    const y = (event.clientY - bounds.top) * scaleY;

    /*
     * Phaser 4 的场景级 pointerdown 在当前页面里存在漏触发风险，但浏览器原生 canvas
     * 事件稳定可达。这里把原生点击坐标换算成 Phaser 画布坐标后，仍然交给同一个
     * handleCanvasPointerDown 处理，所以卡牌命中、余额校验、结算锁和事件触发不会分叉。
     */
    this.handleCanvasPointerDown(x, y);
  };
  private readonly handleScaleResize = (): void => {
    this.redrawArena();
    this.renderHand();
    this.renderHandTimer(performance.now());
  };

  constructor(
    private readonly bootstrap: GameBootstrap,
    callbacks: SceneCallbacks
  ) {
    super("CheckoutRushScene");
    this.callbacks = callbacks;
    this.tuning = resolveBalanceTuning(bootstrap);
    this.state = createInitialState(bootstrap, this.tuning);
  }

  preload(): void {
    /*
     * Phaser 的 preload 会在 create 之前完成资源请求。资源索引里只包含已经制作并提交的图片，
     * 所以这里不会按内容包里的每个数据库 id 猜 URL。图片请求成功后会进入 Phaser 的纹理管理器；
     * 请求失败时纹理不存在，后面的 createArtworkImage 会返回 null，卡牌自动退回原来的程序
     * 绘制样式。加载失败只影响这一张商品图，不会阻止 Scene 创建，也不会改变抽卡和结算。
     */
    const contentItemIds = new Set(this.bootstrap.items.map((item) => item.id));
    for (const asset of listItemArtworkAssets()) {
      if (contentItemIds.has(asset.itemId) && !this.textures.exists(asset.textureKey)) {
        this.load.image(asset.textureKey, asset.url);
      }
    }
  }

  create(): void {
    this.cameras.main.setBackgroundColor("#060606");
    this.arenaGraphics = this.add.graphics();
    this.paymentGraphics = this.add.graphics().setDepth(160);
    this.handTimerGraphics = this.add.graphics().setDepth(155);
    this.handTimerText = this.add
      .text(26, 22, "", {
        color: "#ffd15e",
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
        fontSize: "14px",
        fontStyle: "900"
      })
      .setDepth(156);
    this.attachCanvasPointerListener();
    this.installDebugSnapshot();
    this.redrawArena();
    this.attachScaleResizeListener();

    this.emitState();
  }

  private attachScaleResizeListener(): void {
    /*
     * Scale Manager 属于整个 Phaser Game，而不是只属于当前 Scene。匿名 resize 回调如果
     * 不解绑，旧 Scene 关闭后仍会收到下一次尺寸变化，并尝试调用已经销毁的 Graphics.clear，
     * 浏览器就会报告一次看似偶发的启动错误。这里先移除同一个固定函数，避免 Scene 被重新
     * create 时重复注册，再在 SHUTDOWN 和 DESTROY 两个生命周期出口清理。两个出口都调用
     * off 是安全的：Phaser 找不到监听器时只会保持原状，不会影响新 Scene 的回调。
     */
    this.scale.off("resize", this.handleScaleResize);
    this.scale.on("resize", this.handleScaleResize);
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => this.scale.off("resize", this.handleScaleResize));
    this.events.once(Phaser.Scenes.Events.DESTROY, () => this.scale.off("resize", this.handleScaleResize));
  }

  private attachCanvasPointerListener(): void {
    this.detachCanvasPointerListener();
    this.game.canvas.addEventListener("pointerdown", this.handleNativeCanvasPointerDown);
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => this.detachCanvasPointerListener());
    this.events.once(Phaser.Scenes.Events.DESTROY, () => this.detachCanvasPointerListener());
  }

  private detachCanvasPointerListener(): void {
    this.game.canvas.removeEventListener("pointerdown", this.handleNativeCanvasPointerDown);
  }

  private installDebugSnapshot(): void {
    if (!import.meta.env.DEV) {
      return;
    }

    /*
     * 这个调试入口只在 Vite 开发环境存在。游戏画面是 Phaser canvas，浏览器测试不能像
     * 普通 DOM 页面那样直接读取 9 张卡的文字和金额。这里暴露一个只读快照，方便我们用
     * 真实页面、真实计时和真实后端内容跑“读卡选择”的长局审计；它不提供付款方法，也不
     * 改写余额，所以不会形成第二套游戏逻辑。生产构建里 import.meta.env.DEV 为 false，
     * 这个入口不会作为用户功能启用。
     */
    window.__checkoutRushDebug = () => this.createDebugSnapshot();
    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => this.clearDebugSnapshot());
    this.events.once(Phaser.Scenes.Events.DESTROY, () => this.clearDebugSnapshot());
  }

  private clearDebugSnapshot(): void {
    if (import.meta.env.DEV) {
      delete window.__checkoutRushDebug;
    }
  }

  private createDebugSnapshot(): DebugSnapshot {
    return {
      state: { ...this.state },
      targetSpend: this.targetSpendPerSelection(),
      cards: this.cards.map((card) => ({
        id: card.id,
        itemId: card.item.id,
        name: card.item.name,
        total: this.cardTotal(card),
        price: card.item.price,
        tier: card.item.tier,
        tags: [...card.item.tags],
        multiplier: card.multiplier.label,
        x: card.x,
        y: card.y,
        selected: card.selected,
        affordable: this.cardIsAffordable(card),
        artwork: this.cardArtworkState(card.item)
      }))
    };
  }

  /**
   * `update` 每帧推进时间。这里不再生成持续移动的商品物件，而是维护四种时间压力：
   * 11 分钟硬倒计时、约 35 秒切换一次压力阶段、每 10 秒增加一次利息、
   * 以及 VISA/清空购物车的延迟扣款和冷却。玩家可以快速选择卡片，
   * 但两个技能不会立刻爆发式清空余额。
   */
  update(_time: number, delta: number): void {
    if (this.state.status !== "running") {
      return;
    }

    const now = performance.now();
    const roundLimitAt = this.roundLimitAt();
    const reachedRoundLimit = now >= roundLimitAt;
    const timelineNow = reachedRoundLimit ? roundLimitAt : now;
    this.syncClock(timelineNow);

    if (reachedRoundLimit) {
      this.settleRoundLimit();
      return;
    }

    this.updateStage(now);
    this.updateActiveStatus(now);
    this.applyInterest(now);
    this.applyPendingPayments(now);
    this.applyCheckoutRefresh(now);
    this.applyHandRefresh(now);
    this.updateActionTimers(now);
    this.renderHandTimer(now);

    this.stateEmitAccumulator += delta;
    if (this.stateEmitAccumulator >= STATE_EMIT_INTERVAL_MS) {
      this.stateEmitAccumulator = 0;
      this.emitState();
    }
  }

  private roundLimitAt(): number {
    return this.startedAt + this.bootstrap.config.roundLimitMs;
  }

  private syncClock(now: number): void {
    this.state.elapsedMs = Math.max(0, Math.min(this.bootstrap.config.roundLimitMs, now - this.startedAt));
    this.state.remainingMs = Math.max(0, this.bootstrap.config.roundLimitMs - this.state.elapsedMs);
    this.state.pressure = Phaser.Math.Clamp(this.state.elapsedMs / this.targetClearMs(), 0, 1);
  }

  private settleRoundLimit(): void {
    if (this.state.status !== "running") {
      return;
    }

    const roundLimitAt = this.roundLimitAt();

    /*
     * 浏览器游戏循环不是精确定时器，某一帧可能直接从 10:59.98 跳到 11:00.03。
     * 硬结算线代表“到这一刻就停表”，所以不能让 11 分钟之后才到期的 VISA、购物车或
     * 利息先落账，再显示“硬结算到时”。这里仍然处理严格发生在硬结算线之前的待结算项，
     * 避免因为掉帧漏掉本来应该在 10:59 已经发生的扣款；但不会处理 11:00 及之后的变化。
     */
    const lastPlayableNow = Math.max(this.startedAt, roundLimitAt - 0.001);
    this.syncClock(roundLimitAt);
    this.updateStage(lastPlayableNow);
    this.updateActiveStatus(lastPlayableNow);
    this.applyInterest(lastPlayableNow);
    this.applyPendingPayments(lastPlayableNow);
    this.updateActionTimers(roundLimitAt);
    this.renderHandTimer(roundLimitAt);
    if (this.state.status === "running") {
      this.endRound("timeout", undefined, roundLimitAt);
    }
  }

  private stopIfPastRoundLimit(now = performance.now()): boolean {
    if (this.state.status !== "running" || now < this.roundLimitAt()) {
      return false;
    }

    this.settleRoundLimit();
    return true;
  }

  startRound(username: string): void {
    this.cardDetailOverlay?.destroy();
    this.cardDetailOverlay = null;
    this.clearCards();
    this.pendingPayments = [];
    this.eventCooldownUntil.clear();
    this.purchasedItemCounts.clear();
    this.pendingItemCounts.clear();
    this.chargedSceneEntryStages.clear();
    this.state = createInitialState(this.bootstrap, this.tuning);
    this.settlementStats = createEmptySettlementStats();
    this.state.status = "running";
    this.state.username = username;
    this.startedAt = performance.now();
    this.nextInterestAt = this.startedAt + this.tuning.interestStartDelayMs + this.tuning.interestIntervalMs;
    this.nextHandRefreshAt = this.startedAt + this.tuning.handRefreshMs;
    this.handFrozenUntil = 0;
    this.checkoutLockedUntil = 0;
    this.waitingForCheckoutRefresh = false;
    this.visaAvailableAt = this.startedAt;
    this.clearCartAvailableAt = this.startedAt;
    this.activeStatus = null;
    this.activeStage = 1;
    this.chaosSeed = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
    this.sceneRotationOffset = this.hashSceneSeed(this.chaosSeed);
    this.activeScene = this.selectSceneForStage(this.activeStage);
    this.state.currentSceneName = this.activeScene.name;
    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title: "开局",
      detail: `${username} 拿到 ${formatMoney(this.state.balance)} 的虚拟额度，${Math.round((this.tuning.interestStartDelayMs + this.tuning.interestIntervalMs) / 1000)} 秒后收到第一笔阶梯利息`,
      kind: "system"
    });
    this.applySceneEntryCost(this.activeScene, this.activeStage, this.startedAt);
    if (this.state.status === "running") {
      this.drawNewHand({ now: this.startedAt, force: true });
    }
    this.emitState();
  }

  authorizeVisa(): void {
    if (this.state.status !== "running") {
      return;
    }

    const now = performance.now();
    if (this.stopIfPastRoundLimit(now)) {
      return;
    }
    if (now < this.visaAvailableAt) {
      this.pushSystemFeed("VISA 冷却中", `${Math.ceil((this.visaAvailableAt - now) / 1000)} 秒后可再次使用`);
      return;
    }
    if (this.hasPendingPayment(VISA_PAYMENT_LABEL)) {
      this.pushSystemFeed("VISA 扣款等待中", `上一笔 VISA 还没有落账，${Math.ceil(this.nextPendingMs(VISA_PAYMENT_LABEL, now) / 1000)} 秒后再试`);
      this.updateActionTimers(now);
      this.emitState();
      return;
    }

    if (this.waitingForCheckoutRefresh || now < this.checkoutLockedUntil || now < this.handFrozenUntil) {
      this.pushSystemFeed("收银忙碌", "当前货架正在结算，稍后再刷 VISA");
      return;
    }

    const highest = this.affordableSpendCards()
      .sort((first, second) => this.cardTotal(second) - this.cardTotal(first))[0];

    if (!highest) {
      this.pushSystemFeed("VISA 买不起", "当前余额买不起货架里的消费卡，先等刷新或处理入账卡");
      return;
    }

    const lines = [this.paymentLineFromCard(highest)];
    if (!this.reservePendingLines(lines)) {
      this.pushSystemFeed("VISA 已买过", "这张一次性消费已经被锁定或买过，等下一手货架");
      this.updateActionTimers(now);
      this.emitState();
      return;
    }

    highest.selected = true;
    this.markCardPending(highest, `VISA ${Math.round(this.tuning.visaDelayMs / 1000)} 秒后扣款`);
    this.pendingPayments.push({
      id: crypto.randomUUID(),
      executeAt: now + this.tuning.visaDelayMs,
      actionLabel: VISA_PAYMENT_LABEL,
      lines,
      scene: this.currentScene(),
      refreshHandAfterPayment: false
    });
    this.visaAvailableAt = now + this.tuning.visaCooldownMs;
    this.pushSystemFeed(
      "VISA 已授权",
      `${this.describeLine(this.paymentLineFromCard(highest))} 将在 ${Math.round(this.tuning.visaDelayMs / 1000)} 秒后扣款 ${formatMoney(this.cardTotal(highest))}`
    );
    this.drawNewHand({ now });
    this.updateActionTimers(now);
    this.emitState();
  }

  clearCart(): void {
    if (this.state.status !== "running") {
      return;
    }

    const now = performance.now();
    if (this.stopIfPastRoundLimit(now)) {
      return;
    }
    if (now < this.clearCartAvailableAt) {
      this.pushSystemFeed("购物车冷却中", `${Math.ceil((this.clearCartAvailableAt - now) / 1000)} 秒后可再次使用`);
      return;
    }
    if (this.hasPendingPayment(CLEAR_CART_PAYMENT_LABEL)) {
      this.pushSystemFeed("购物车结算中", `上一笔购物车还没有落账，${Math.ceil(this.nextPendingMs(CLEAR_CART_PAYMENT_LABEL, now) / 1000)} 秒后再试`);
      this.updateActionTimers(now);
      this.emitState();
      return;
    }

    if (this.waitingForCheckoutRefresh || now < this.checkoutLockedUntil || now < this.handFrozenUntil) {
      this.pushSystemFeed("收银忙碌", "当前货架正在结算，稍后再清空购物车");
      return;
    }

    const spendCards = this.pickClearCartCards();

    if (spendCards.length < Math.max(1, this.tuning.clearCartPickCount)) {
      this.pushSystemFeed("购物车买不起", "当前余额不足以同时支付 3 张消费卡，等刷新或先处理更小的消费");
      return;
    }

    const lines = spendCards.map((card) => this.paymentLineFromCard(card));
    if (!this.reservePendingLines(lines)) {
      this.pushSystemFeed("购物车有重复限购", "部分一次性消费已经被锁定或买过，等下一手货架");
      this.updateActionTimers(now);
      this.emitState();
      return;
    }

    for (const card of spendCards) {
      card.selected = true;
      this.markCardPending(card, `购物车 ${Math.round(this.tuning.clearCartDelayMs / 1000)} 秒结算`);
    }

    this.pendingPayments.push({
      id: crypto.randomUUID(),
      executeAt: now + this.tuning.clearCartDelayMs,
      actionLabel: CLEAR_CART_PAYMENT_LABEL,
      lines,
      scene: this.currentScene(),
      refreshHandAfterPayment: true
    });
    this.clearCartAvailableAt = now + this.tuning.clearCartCooldownMs;
    this.handFrozenUntil = now + this.tuning.clearCartDelayMs;
    this.pushSystemFeed(
      "购物车结算中",
      `${spendCards.map((card) => this.describeLine(this.paymentLineFromCard(card))).join("、")} 将在 ${Math.round(this.tuning.clearCartDelayMs / 1000)} 秒后扣款`
    );
    this.renderHandTimer(now);
    this.updateActionTimers(now);
    this.emitState();
  }

  private updateStage(now: number): void {
    const nextStage = Math.min(this.state.totalStages, Math.floor(this.state.elapsedMs / this.tuning.stageDurationMs) + 1);
    const nextScene = this.selectSceneForStage(nextStage);
    if (nextStage === this.activeStage && this.activeScene?.id === nextScene.id) {
      this.state.currentStage = nextStage;
      this.state.currentSceneName = nextScene.name;
      return;
    }

    const stageChanged = nextStage !== this.activeStage;
    this.activeStage = nextStage;
    this.activeScene = nextScene;
    this.state.currentStage = nextStage;
    this.state.currentSceneName = this.activeScene.name;
    this.pushSceneChangeFeed(stageChanged, nextStage, this.activeScene, now);
    this.applySceneEntryCost(this.activeScene, nextStage, now);
    if (this.state.status !== "running") {
      return;
    }

    if (this.handRefreshIsLocked(now)) {
      /*
       * 阶段切换是时间线事件，购物车等待和普通刷卡锁也是时间线事件。两者撞在同一小段时间
       * 时，不能让阶段切换强行刷新货架，否则“购物车结算期间不刷新新卡”和“收银结算锁”
       * 就会被绕开。这里仍然更新当前阶段、场景名和入场成本，因为时间确实走到了下一段；
       * 但真正的 9 张卡会等锁结束后由 applyPendingPayments 或 applyCheckoutRefresh 统一刷新。
       */
      this.updateActionTimers(now);
      this.renderHandTimer(now);
      this.emitState();
      return;
    }

    this.drawNewHand({ now });
    this.updateActionTimers(now);
  }

  private handRefreshIsLocked(now: number): boolean {
    return this.waitingForCheckoutRefresh || now < this.checkoutLockedUntil || now < this.handFrozenUntil;
  }

  private applySceneEntryCost(scene: Scene, stage: number, now: number): void {
    if (scene.entryCost <= 0 || this.chargedSceneEntryStages.has(stage) || this.state.status !== "running") {
      return;
    }

    this.chargedSceneEntryStages.add(stage);
    const spent = Math.min(scene.entryCost, this.state.balance);
    if (spent <= 0) {
      return;
    }

    /*
     * 场景入场费是数据库内容的一部分，代表进入高压消费场景时立刻产生的服务费、押金或
     * 门槛成本。它不是玩家点选的商品，所以不增加 paymentCount；但它确实减少余额，
     * 也应该进入总消费、最大单笔和战报高光，否则结算页会和真实余额变化对不上。
     */
    this.state.balance -= spent;
    this.state.totalSpent += spent;
    this.state.maxSingleSpend = Math.max(this.state.maxSingleSpend, spent);
    this.settlementStats.eventCount += 1;
    this.recordSettlementHighlight({
      title: `${scene.name} 入场成本`,
      detail: `第 ${stage} 段进入高压消费场景，先结算服务费或押金`,
      amount: spent,
      kind: "spend",
      source: "event"
    });
    this.callbacks.onTone(spent >= this.tuning.highPriceThreshold ? "danger" : "spend");
    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title: "场景入场成本",
      detail: `进入「${scene.name}」先扣 ${formatMoney(spent)}`,
      kind: "spend"
    });
    this.showCheckoutPulse(spent);
    this.emitState();

    if (this.state.balance <= 0) {
      this.endRound("balance_zero", undefined, now);
    }
  }

  private applyInterest(now: number): void {
    while (now >= this.nextInterestAt && this.state.status === "running") {
      const rate = this.interestRateForBalance();
      const amount = Math.max(1, Math.round(this.state.balance * rate));
      this.state.balance += amount;
      this.state.totalIncome += amount;
      this.settlementStats.interestCount += 1;
      this.recordSettlementHighlight({
        title: "利息入账",
        detail: `当前余额按 ${this.formatPercent(rate)} 增加，拖慢清空节奏`,
        amount,
        kind: "income",
        source: "interest"
      });
      this.nextInterestAt += this.tuning.interestIntervalMs;
      this.callbacks.onTone("income");
      this.callbacks.onFeedEvent({
        id: crypto.randomUUID(),
        title: "利息入账",
        detail: `当前余额增加 ${this.formatPercent(rate)}：+${formatMoney(amount)}`,
        kind: "income"
      });
    }
  }

  private interestRateForBalance(): number {
    const bands = [...this.tuning.interestBands].sort((first, second) => second.minBalance - first.minBalance);
    return bands.find((band) => this.state.balance >= band.minBalance)?.rate ?? this.tuning.interestRate;
  }

  private formatPercent(rate: number): string {
    const percent = rate * 100;
    return Number.isInteger(percent) ? `${percent}%` : `${percent.toFixed(1)}%`;
  }

  private recordSettlementHighlight(highlight: SettlementHighlight): void {
    if (highlight.amount <= 0) {
      return;
    }

    /*
     * 这里记录的是一笔真实发生的金额变动，所以它既会增加“扣款/入账笔数”，也会参与
     * “最烦人返钱/最荒诞扣款”的高光比较。排行榜仍然只看后端接受的成绩摘要；这些统计
     * 只服务结算页和本地最近战报，帮助玩家理解这一局的过程。
     */
    this.countSettlementMovement(highlight);
    this.rememberSettlementHighlight(highlight);
  }

  private countSettlementMovement(highlight: SettlementHighlight): void {
    if (highlight.kind === "income") {
      this.settlementStats.incomeCount += 1;
      return;
    }

    this.settlementStats.spendCount += 1;
  }

  private rememberSettlementHighlight(highlight: SettlementHighlight): void {
    if (highlight.kind === "income") {
      if (!this.settlementStats.mostAnnoyingIncome || highlight.amount > this.settlementStats.mostAnnoyingIncome.amount) {
        this.settlementStats.mostAnnoyingIncome = highlight;
      }
      return;
    }

    if (!this.settlementStats.mostAbsurdSpend || highlight.amount > this.settlementStats.mostAbsurdSpend.amount) {
      this.settlementStats.mostAbsurdSpend = highlight;
    }
  }

  private recordPaymentLine(line: PaymentLine, actionLabel: string): void {
    const amount = this.lineTotal(line);
    const item = line.item;

    this.recordSettlementHighlight({
      title: this.describeLine(line),
      detail: `${actionLabel} · ${item.category} · ${item.flavor}`,
      amount,
      kind: item.tier === "income" ? "income" : "spend",
      source: "payment"
    });
  }

  private rememberPaymentActionHighlight(lines: PaymentLine[], actionLabel: string, spent: number, income: number): void {
    if (lines.length <= 1) {
      return;
    }

    /*
     * 清空购物车这类动作会把多张卡合并成一次账单。上面的 recordPaymentLine 仍然逐张记录
     * “扣款/入账笔数”，这样节奏统计不会被合计账单重复增加；这里单独把合计金额拿来参与
     * 高光比较，保证结算页的“最荒诞扣款”能和玩家看到的整次购物车账单对得上。
     */
    const detail = lines.map((line) => this.describeLine(line)).join("、");
    if (spent > 0) {
      this.rememberSettlementHighlight({
        title: actionLabel,
        detail: `${detail} 合并扣款`,
        amount: spent,
        kind: "spend",
        source: "payment"
      });
    }
    if (income > 0) {
      this.rememberSettlementHighlight({
        title: actionLabel,
        detail: `${detail} 合并入账`,
        amount: income,
        kind: "income",
        source: "payment"
      });
    }
  }

  private currentStatusEffect(now = performance.now()): StatusEffect | null {
    if (!this.activeStatus || now >= this.activeStatus.expiresAt) {
      return null;
    }

    return this.activeStatus.effect;
  }

  private currentHandRefreshMs(now = performance.now()): number {
    const refreshSpeedMultiplier = this.currentStatusEffect(now)?.itemRefreshMultiplier ?? 1;

    /*
     * 数据库里的 itemRefreshMultiplier 表达“刷新速度倍率”，不是“等待时间倍率”。
     * 例如疲劳是 0.85，意思是刷新变慢，所以这里要用基础等待时间除以速度倍率。
     * 这样状态内容、画布货架计时条和下一手刷新时间才是同一套含义。
     */
    return Phaser.Math.Clamp(this.tuning.handRefreshMs / Math.max(0.4, refreshSpeedMultiplier), 4_200, 10_500);
  }

  private scenePressureReductionReason(now = performance.now()): "low-mood" | "target-clear" | null {
    const activeStatus = this.currentStatusEffect(now);
    if (activeStatus?.tags.includes("low-mood") || activeStatus?.name === "低落") {
      return "low-mood";
    }

    if (this.state.elapsedMs >= this.targetClearMs()) {
      return "target-clear";
    }

    return null;
  }

  private updateActiveStatus(now: number): void {
    if (!this.activeStatus || now < this.activeStatus.expiresAt) {
      return;
    }

    const endedName = this.activeStatus.effect.name;
    this.activeStatus = null;
    this.pushSystemFeed("状态结束", `${endedName} 的影响已经消退`);
  }

  private applyPendingPayments(now: number): void {
    const due = this.pendingPayments
      .filter((payment) => payment.executeAt <= now)
      .sort((first, second) => first.executeAt - second.executeAt);
    if (due.length === 0) {
      return;
    }

    this.pendingPayments = this.pendingPayments.filter((payment) => payment.executeAt > now);

    for (const payment of due) {
      this.releasePendingLines(payment.lines);
      this.applyPayment(payment.lines, payment.actionLabel, Math.min(payment.executeAt, now), payment.scene);
      if (this.state.status === "ended") {
        return;
      }
      if (payment.refreshHandAfterPayment) {
        this.drawNewHand({ now, force: true });
      }
    }
  }

  private clearDeferredPaymentState(): void {
    /*
     * VISA 和清空购物车都是“先占住商品，稍后才扣钱”的动作。pendingPayments 是还没有
     * 到执行时间的扣款队列，pendingItemCounts 是为了让 maxBuy=1 的一次性商品在等待扣款
     * 时也算作已经被占用。游戏结束后，这些未来扣款不应该再发生，也不应该继续让按钮、
     * 倒计时或限购判断以为本局还在结算，所以在统一终局入口清掉这两层状态。
     */
    for (const payment of this.pendingPayments) {
      this.releasePendingLines(payment.lines);
    }
    this.pendingPayments = [];
    this.pendingItemCounts.clear();
    this.waitingForCheckoutRefresh = false;
    this.handFrozenUntil = 0;
    this.checkoutLockedUntil = 0;
  }

  private updateActionTimers(now: number): void {
    const activeStatus = this.currentStatusEffect(now);
    this.state.visaCooldownMs = Math.max(0, this.visaAvailableAt - now);
    this.state.clearCooldownMs = Math.max(0, this.clearCartAvailableAt - now);
    this.state.visaPendingMs = this.nextPendingMs(VISA_PAYMENT_LABEL, now);
    this.state.clearPendingMs = this.nextPendingMs(CLEAR_CART_PAYMENT_LABEL, now);
    this.state.nextInterestMs = Math.max(0, this.nextInterestAt - now);
    this.state.handFrozen = now < this.handFrozenUntil;
    this.state.checkoutLockMs = Math.max(0, this.checkoutLockedUntil - now);
    this.state.nextHandRefreshMs = this.state.handFrozen
      ? Math.max(0, this.handFrozenUntil - now)
      : this.state.checkoutLockMs > 0
        ? this.state.checkoutLockMs
        : Math.max(0, this.nextHandRefreshAt - now);
    this.state.pendingPaymentCount = this.pendingPayments.length;
    this.state.activeStatusName = activeStatus?.name ?? null;
    this.state.activeStatusMs = this.activeStatus ? Math.max(0, this.activeStatus.expiresAt - now) : 0;

    /*
     * 这里把“按钮是否能按”也放进运行状态里，而不是让 DOM 层自己猜。前端页面只负责
     * 根据状态显示按钮，真正的业务判断仍在 Scene：VISA 必须找到一张当前余额买得起的
     * 消费卡；清空购物车必须能凑出 3 张当前余额买得起的消费卡。这样后续如果平衡算法
     * 改了，按钮和真实结算不会出现一个能按、一个又拒绝的割裂。
     */
    const busy = this.waitingForCheckoutRefresh || this.state.handFrozen || this.state.checkoutLockMs > 0;
    const running = this.state.status === "running";
    this.state.canUseVisa =
      running && !busy && this.state.visaCooldownMs === 0 && this.state.visaPendingMs === 0 && this.affordableSpendCards().length > 0;
    this.state.canUseClearCart =
      running &&
      !busy &&
      this.state.clearCooldownMs === 0 &&
      this.state.clearPendingMs === 0 &&
      this.affordableClearCartPreview().length >= Math.max(1, this.tuning.clearCartPickCount);
  }

  private hasPendingPayment(actionLabel: string): boolean {
    /*
     * DOM 按钮会根据 visaPendingMs 和 clearPendingMs 禁用，但 Scene 方法本身仍然是业务入口：
     * 键盘空格、未来调试脚本或后续 UI 都可能直接调用 authorizeVisa/clearCart。这里在 Scene
     * 内部再次检查同类待结算项，避免后端以后把冷却调得短于延迟时，玩家能在上一笔还没落账
     * 的情况下继续排队同一个技能，造成按钮状态和真实扣款队列不一致。
     */
    return this.pendingPayments.some((payment) => payment.actionLabel === actionLabel);
  }

  private nextPendingMs(actionLabel: string, now: number): number {
    const pending = this.pendingPayments
      .filter((payment) => payment.actionLabel === actionLabel)
      .sort((first, second) => first.executeAt - second.executeAt)[0];

    return pending ? Math.max(0, pending.executeAt - now) : 0;
  }

  private applyHandRefresh(now: number): void {
    if (this.waitingForCheckoutRefresh || now < this.handFrozenUntil || now < this.checkoutLockedUntil || now < this.nextHandRefreshAt) {
      return;
    }

    if (this.cards.some((card) => isHighSpend(card.item, this.tuning.highPriceThreshold))) {
      this.pushSystemFeed("错过机会", "这手高额消费已经刷新，下次不一定还在");
    }
    this.drawNewHand({ now, force: true });
  }

  private applyCheckoutRefresh(now: number): void {
    if (!this.waitingForCheckoutRefresh || now < this.checkoutLockedUntil || this.state.status !== "running") {
      return;
    }

    this.waitingForCheckoutRefresh = false;
    this.drawNewHand({ now, force: true });
  }

  private drawNewHand(options: { now?: number; force?: boolean } = {}): void {
    const now = options.now ?? performance.now();
    if (!options.force && now < this.handFrozenUntil) {
      return;
    }

    this.clearCards();
    const scene = this.currentScene();
    /*
     * 抽卡必须使用本次刷新发生时的业务时间 now，而不是在各个权重函数里重新读取
     * performance.now()。普通刷新时两者通常接近，但硬结算补处理、购物车延迟结算或浏览器
     * 掉帧时，业务时间可能代表“几秒前本该发生的刷新”。状态效果是否还有效，也应该按这个
     * 业务时间判断，否则同一笔结算会因为帧率不同拿到不同的高价卡权重。
     */
    const hand = this.pickHand(scene, now);

    this.cards = hand.map((item) => ({
      id: crypto.randomUUID(),
      item,
      multiplier: this.pickMultiplier(item),
      container: null,
      selected: false,
      hitBounds: new Phaser.Geom.Rectangle(0, 0, 0, 0),
      x: 0,
      y: 0
    }));
    this.nextHandRefreshAt = now + this.currentHandRefreshMs(now);
    this.renderHand();
    this.flashCheckout();
    this.renderHandTimer(now);
  }

  private pickHand(scene: Scene, now: number): Item[] {
    const currentBalance = Math.max(this.state.balance, 0);
    const availableItems = this.bootstrap.items.filter((item) => this.itemHasRemainingUses(item));
    const fallbackEligible = availableItems.filter((item) => item.minBalance <= currentBalance);
    const eligible = fallbackEligible.filter((item) => this.itemCanAppearInHand(item, currentBalance));
    const highCandidates = eligible.filter((item) => isHighSpend(item, this.tuning.highPriceThreshold));
    const regularCandidates = eligible.filter((item) => !isHighSpend(item, this.tuning.highPriceThreshold));
    const cards: Item[] = [];
    const usedItemIds = new Set<string>();
    const pacingAssistCard = this.pickPacingAssistCard(scene, eligible, usedItemIds, now);

    if (pacingAssistCard) {
      cards.push(pacingAssistCard);
    }

    const assistHighCount = cards.filter((item) => isHighSpend(item, this.tuning.highPriceThreshold)).length;
    const highCount = Math.max(0, this.pickHighCardCount(scene, highCandidates.length, now) - assistHighCount);

    /*
     * 普通场景每一手最多很低概率塞入一张大额卡，这样 9 张卡里绝大多数都是日常、
     * 交通、社交和小事故消费，不会像之前那样几秒就靠大额连点清空。进入拍卖预展、
     * 面具舞会这类特定场景时，才提高大额卡数量，让“稀有副本”有明显风险。
     */
    for (let remaining = highCount; remaining > 0; remaining -= 1) {
      cards.push(this.pickUniqueWeighted(highCandidates, scene, usedItemIds, now));
    }

    this.appendSceneAnchorCards(scene, regularCandidates, usedItemIds, cards, now);

    while (cards.length < CARD_COUNT) {
      const pool =
        regularCandidates.length > 0
          ? regularCandidates
            : eligible.length > 0
              ? eligible
              : fallbackEligible.length > 0
                ? fallbackEligible
                : availableItems.length > 0
                  ? availableItems
                  : this.bootstrap.items;
      cards.push(this.pickUniqueWeighted(pool, scene, usedItemIds, now));
    }

    return Phaser.Utils.Array.Shuffle(cards);
  }

  private appendSceneAnchorCards(scene: Scene, candidates: Item[], usedItemIds: Set<string>, cards: Item[], now: number): void {
    const targetCount = scene.rarity === "common" ? 2 : 3;
    const sceneCandidates = candidates.filter((item) => item.sceneId === scene.id);

    /*
     * sceneId 是后端内容表给商品写的“这张卡属于哪个消费场景”。如果抽卡只给它一点点
     * 权重，300 张卡会把场景专属内容稀释掉，玩家进入“医院走廊”也可能连续看到完全
     * 无关的普通账单。这里给每手货架保留少量普通价位的场景锚点卡；大额卡仍然走
     * pickHighCardCount 的限额，所以不会绕开普通场景大额概率限制。
     */
    for (let picked = 0; picked < targetCount && cards.length < CARD_COUNT; picked += 1) {
      if (sceneCandidates.every((item) => usedItemIds.has(item.id))) {
        return;
      }

      cards.push(this.pickUniqueWeighted(sceneCandidates, scene, usedItemIds, now));
    }
  }

  private pickPacingAssistCard(scene: Scene, candidates: Item[], usedItemIds: Set<string>, now: number): Item | null {
    const { behindRatio } = this.pacingGap();
    if (behindRatio < 0.2 || this.state.elapsedMs < this.tuning.interestStartDelayMs + 24_000) {
      return null;
    }

    const balance = Math.max(1, this.state.balance);
    const target = this.targetSpendPerSelection();
    const maxReasonablePrice = Math.min(balance * 0.28, Math.max(target * 1.95, target + this.tuning.highPriceThreshold));
    const canUseHighPacingAssist =
      scene.rarity !== "common" ||
      behindRatio >= MIN_BEHIND_RATIO_FOR_HIGH_PACING_ASSIST ||
      this.state.elapsedMs / this.targetClearMs() >= MIN_ELAPSED_RATIO_FOR_HIGH_PACING_ASSIST;
    const spendCandidates = candidates.filter(
      (item) =>
        !usedItemIds.has(item.id) &&
        isSpend(item) &&
        (canUseHighPacingAssist || !isHighSpend(item, this.tuning.highPriceThreshold)) &&
        item.price <= balance &&
        item.price <= maxReasonablePrice
    );

    if (spendCandidates.length === 0) {
      return null;
    }

    const lowerRatio = behindRatio > 0.45 ? 0.38 : 0.52;
    const upperRatio = behindRatio > 0.45 ? 1.95 : 1.65;
    const targetBand = spendCandidates.filter((item) => item.price >= target * lowerRatio && item.price <= target * upperRatio);
    const pool =
      targetBand.length > 0
        ? targetBand
        : [...spendCandidates]
            .sort((first, second) => this.targetDistance(first.price, target) - this.targetDistance(second.price, target))
            .slice(0, Math.min(5, spendCandidates.length));

    /*
     * 这张“追赶卡”不是新的必杀技能，而是给读卡策略补一个必要条件：当玩家已经落后时，
     * 系统不能一边计算出本轮应该花 8 万、10 万，一边只发几千块的小票。早期普通场景还
     * 没有严重落后时，只补中额可读卡；只有明显落后、进入后半局或本来就在特殊场景里，
     * 追赶卡才允许使用大额卡。这样它不会变成普通场景开局后的保底大额漏洞。
     */
    return this.pickUniqueWeighted(pool, scene, usedItemIds, now);
  }

  private targetDistance(price: number, target: number): number {
    return Math.abs(Math.log2(Math.max(0.025, price / Math.max(1, target))));
  }

  private itemCanAppearInHand(item: Item, currentBalance: number): boolean {
    if (!isSpend(item)) {
      return true;
    }

    if (item.price <= currentBalance) {
      return true;
    }

    /*
     * 货架可以偶尔出现“差一点买不起”的卡，这样用户会看到买不起反馈，也能感到余额
     * 变少后的压力。但这里不能让大量远超余额的商品占满 9 张卡，否则后期就不是选择，
     * 而是干等刷新。这个函数只放行接近当前余额的消费卡，真正能否扣款仍由
     * cardIsAffordable 和 applyPayment 做最后判断。
     */
    if (currentBalance < 1_000) {
      return item.price <= currentBalance + 240;
    }

    return item.price <= currentBalance * 1.12;
  }

  private pickHighCardCount(scene: Scene, candidateCount: number, now: number): number {
    if (candidateCount <= 0) {
      return 0;
    }

    if (this.state.balance < this.tuning.highPriceThreshold * 1.4) {
      return 0;
    }

    if (scene.rarity !== "common") {
      const pacing = this.pacingGap();
      const statusHighMultiplier = this.currentStatusEffect(now)?.highPriceMultiplier ?? 1;
      const baseCount = scene.rarity === "wild" || pacing.behindRatio > 0.18 ? this.tuning.specialHighCardCount : 1;
      return Math.min(Math.max(1, Math.round(baseCount * Math.sqrt(statusHighMultiplier))), candidateCount);
    }

    return Math.random() < this.dynamicNormalHighChance(now) ? 1 : 0;
  }

  private dynamicNormalHighChance(now: number): number {
    const { behindRatio, aheadRatio } = this.pacingGap();
    const lowBalancePenalty = this.state.balance < this.tuning.highPriceThreshold * 3 ? 0.35 : 1;
    const statusHighMultiplier = this.currentStatusEffect(now)?.highPriceMultiplier ?? 1;

    /*
     * 普通场景的大额卡概率可以随玩家进度轻微变化，但不能突破开发计划里的 10% 上限。
     * 真正需要追赶时，pickPacingAssistCard 会另外补一张接近目标金额的可买卡；这里如果
     * 也放到 14% 或更高，普通场景就会悄悄变成高压场景，玩家又会遇到几手大额卡过快结束。
     */
    return Phaser.Math.Clamp(
      (this.tuning.normalHighCardHandChance + behindRatio * 0.22 - aheadRatio * 0.12) * lowBalancePenalty * statusHighMultiplier,
      0.01,
      MAX_NORMAL_HIGH_CARD_HAND_CHANCE
    );
  }

  private pacingGap(): { expectedClearRatio: number; actualClearRatio: number; behindRatio: number; aheadRatio: number } {
    const expectedClearRatio = Phaser.Math.Clamp(this.state.elapsedMs / this.targetClearMs(), 0, 1);
    const actualClearRatio = Phaser.Math.Clamp(
      (this.bootstrap.config.initialBalance - this.state.balance) / this.bootstrap.config.initialBalance,
      0,
      1
    );

    return {
      expectedClearRatio,
      actualClearRatio,
      behindRatio: Math.max(0, expectedClearRatio - actualClearRatio),
      aheadRatio: Math.max(0, actualClearRatio - expectedClearRatio)
    };
  }

  private targetSpendPerSelection(): number {
    const remainingTargetMs = Math.max(this.tuning.selectionSettleMs, this.targetClearMs() - this.state.elapsedMs);
    const expectedDecisionMs = Math.max(EXPECTED_DECISION_MS, this.tuning.selectionSettleMs + 900);
    const remainingSelections = Math.max(6, remainingTargetMs / expectedDecisionMs);
    const { behindRatio, aheadRatio } = this.pacingGap();
    const paceMultiplier = Phaser.Math.Clamp(1 + behindRatio * 1.8 - aheadRatio * 0.55, 0.65, 1.85);
    const rawTarget = (Math.max(1, this.state.balance) / remainingSelections) * paceMultiplier;

    /*
     * 这里是金额算法的核心：系统每次抽牌都估算“如果要在 7 分钟左右清空，下一次选择
     * 大概应该花多少钱”。这个目标不是固定金额，而是跟当前余额、剩余目标时间和玩家
     * 领先/落后的程度一起变化。玩家落后时，算法会提高合适价位；玩家已经花得过快时，
     * 算法会降低高价权重，避免几张超大额卡直接把一局压到两分钟。
     */
    return Phaser.Math.Clamp(rawTarget, this.state.balance < 1_000 ? 25 : 380, Math.max(480, this.state.balance * 0.18));
  }

  private targetClearMs(): number {
    return Math.max(1, this.tuning.targetClearMs || this.tuning.stageDurationMs * this.tuning.stageCount);
  }

  private pickUniqueWeighted(candidates: Item[], scene: Scene, usedItemIds: Set<string>, now: number): Item {
    const unusedCandidates = candidates.filter((item) => !usedItemIds.has(item.id));
    const picked = this.pickWeighted(unusedCandidates.length > 0 ? unusedCandidates : candidates, scene, now);
    usedItemIds.add(picked.id);
    return picked;
  }

  private pickWeighted(candidates: Item[], scene: Scene, now: number): Item {
    const weighted = candidates.map((item) => {
      const sceneBoost = this.sceneAffinityWeight(item, scene);
      const incomePenalty = this.incomePressureWeight(item, scene);
      const unaffordablePenalty = isSpend(item) && item.price > this.state.balance ? 0.04 : 1;
      const commonScenePricePenalty = scene.rarity === "common" && isSpend(item) ? this.commonScenePricePenalty(item.price) : 1;
      const pacingWeight = this.pricePacingWeight(item);
      const statusHighWeight = isHighSpend(item, this.tuning.highPriceThreshold) ? (this.currentStatusEffect(now)?.highPriceMultiplier ?? 1) : 1;

      return {
        item,
        weight: Math.max(
          0.01,
          item.weight *
            TIER_WEIGHT[item.tier] *
            sceneBoost *
            incomePenalty *
            unaffordablePenalty *
            commonScenePricePenalty *
            pacingWeight *
            statusHighWeight
        )
      };
    });
    const totalWeight = weighted.reduce((sum, candidate) => sum + candidate.weight, 0);
    let roll = Math.random() * totalWeight;

    for (const candidate of weighted) {
      roll -= candidate.weight;
      if (roll <= 0) {
        return candidate.item;
      }
    }

    return candidates[0] ?? this.bootstrap.items[0];
  }

  private sceneAffinityWeight(item: Item, scene: Scene): number {
    const tagMatched = item.tags.some((tag) => scene.itemTags.includes(tag));

    if (item.sceneId === scene.id) {
      return 2.6 + scene.riskLevel * 0.34;
    }

    if (item.sceneId) {
      return tagMatched ? 0.58 : 0.28;
    }

    return tagMatched ? 1 + scene.riskLevel * 0.16 : 1;
  }

  private incomePressureWeight(item: Item, scene: Scene): number {
    if (item.tier !== "income") {
      return 1;
    }

    const { behindRatio, aheadRatio } = this.pacingGap();
    const sceneWantsIncome = scene.itemTags.includes("income") || scene.eventTags.includes("income");
    const sceneBase = sceneWantsIncome ? 0.9 : 0.34;
    const paceAdjustment = Phaser.Math.Clamp(1 + aheadRatio * 1.35 - behindRatio * 1.2, 0.32, 1.75);
    const hugeIncomePenalty = item.price > Math.max(12_000, this.state.balance * 0.1) ? 0.35 : 1;

    return Phaser.Math.Clamp(sceneBase * paceAdjustment * hugeIncomePenalty, 0.05, 1.2);
  }

  private pricePacingWeight(item: Item): number {
    if (!isSpend(item)) {
      return 1;
    }

    const price = Math.max(1, item.price);
    const balance = Math.max(1, this.state.balance);

    if (balance < 1_000) {
      if (price > balance) {
        return 0.05;
      }

      return price <= Math.max(120, balance * 0.35) ? 1.35 : 0.9;
    }

    if (price > balance) {
      return 0.06;
    }

    const target = this.targetSpendPerSelection();
    const ratio = price / Math.max(1, target);
    const distance = Math.abs(Math.log2(Math.max(0.025, ratio)));
    const fit = Math.pow(0.62, distance);
    const tinyPenalty = price < target * 0.08 && balance > target * 5 ? 0.35 : 1;
    const oversizedPenalty = price > target * 7 ? 0.1 : price > target * 4 ? 0.25 : price > target * 2.4 ? 0.55 : 1;
    const finishBoost = balance < target * 2.4 && price >= balance * 0.45 ? 1.55 : 1;

    return Phaser.Math.Clamp(fit * tinyPenalty * oversizedPenalty * finishBoost, 0.04, 1.75);
  }

  private commonScenePricePenalty(price: number): number {
    if (price >= this.tuning.highPriceThreshold) {
      return 0.16;
    }

    if (price >= this.tuning.highPriceThreshold * 0.6) {
      return 0.26;
    }

    if (price >= this.tuning.highPriceThreshold * 0.35) {
      return 0.48;
    }

    return 1;
  }

  private renderHand(): void {
    if (this.cards.length === 0) {
      return;
    }

    const width = this.scale.width;
    const height = this.scale.height;
    const gap = Phaser.Math.Clamp(Math.min(width, height) * 0.028, 16, 28);
    const cardWidth = Math.min(340, (width * 0.94 - gap * (CARD_COLUMNS - 1)) / CARD_COLUMNS);
    const cardHeight = Math.min(156, (height * 0.64 - gap * (CARD_ROWS - 1)) / CARD_ROWS);
    const gridWidth = cardWidth * CARD_COLUMNS + gap * (CARD_COLUMNS - 1);
    const startX = width / 2 - gridWidth / 2 + cardWidth / 2;
    const startY = height * 0.1 + cardHeight / 2;

    for (const [index, card] of this.cards.entries()) {
      const column = index % CARD_COLUMNS;
      const row = Math.floor(index / CARD_COLUMNS);
      card.x = startX + column * (cardWidth + gap);
      card.y = startY + row * (cardHeight + gap);
      const hitPadding = Math.min(10, gap * 0.3);
      card.hitBounds = new Phaser.Geom.Rectangle(
        card.x - cardWidth / 2 - hitPadding,
        card.y - cardHeight / 2 - hitPadding,
        cardWidth + hitPadding * 2,
        cardHeight + hitPadding * 2
      );
      card.container?.destroy();
      card.container = this.createCardContainer(card, cardWidth, cardHeight);
      this.animateCardIn(card.container, index);
    }
  }

  private createCardContainer(card: CardSlot, width: number, height: number): Phaser.GameObjects.Container {
    const item = card.item;
    const palette = TIER_COLORS[item.tier];
    const total = this.cardTotal(card);
    const container = this.add.container(card.x, card.y).setDepth(40);
    const backplate = this.add.graphics();
    const shape = this.add.graphics();
    const compactCard = width < 150;
    const titleSize = cardTitleFontSize(item.name, width);
    const priceSize = compactCard ? "18px" : total >= this.tuning.highPriceThreshold ? "22px" : "25px";
    const badgeX = width / 2 - (compactCard ? 14 : 34);
    const badgeY = -height / 2 + (compactCard ? 10 : 20);
    const badgeColor = card.multiplier.multiplier > 1 ? 0x57fff3 : palette.stroke;
    const multiplierBadge = this.add
      .text(badgeX, badgeY, card.multiplier.label, {
        align: "center",
        color: card.multiplier.multiplier > 1 ? "#bafff9" : "#fff0b5",
        fontFamily: "Impact, Haettenschweiler, 'Arial Narrow Bold', sans-serif",
        fontSize: compactCard ? "12px" : "17px",
        backgroundColor: "#120d08",
        padding: compactCard ? { left: 4, right: 4, top: 2, bottom: 2 } : { left: 7, right: 7, top: 3, bottom: 3 }
      })
      .setOrigin(0.5);
    const badgeFrame = this.add.graphics();
    const badgeWidth = multiplierBadge.displayWidth + (compactCard ? 4 : 7);
    const badgeHeight = multiplierBadge.displayHeight + (compactCard ? 3 : 5);
    badgeFrame.fillStyle(0x070605, 0.94);
    badgeFrame.fillRoundedRect(badgeX - badgeWidth / 2, badgeY - badgeHeight / 2, badgeWidth, badgeHeight, compactCard ? 3 : 5);
    badgeFrame.lineStyle(compactCard ? 1 : 2, badgeColor, 0.98);
    badgeFrame.strokeRoundedRect(badgeX - badgeWidth / 2, badgeY - badgeHeight / 2, badgeWidth, badgeHeight, compactCard ? 3 : 5);
    badgeFrame.lineStyle(1, palette.highlight, 0.52);
    badgeFrame.lineBetween(badgeX - badgeWidth * 0.3, badgeY - badgeHeight / 2 + 2, badgeX + badgeWidth * 0.18, badgeY - badgeHeight / 2 + 2);
    const label = this.add
      .text(0, -height * (compactCard ? 0.2 : 0.25), item.name, {
        align: "center",
        color: palette.text,
        fontFamily: "Impact, Haettenschweiler, 'Arial Narrow Bold', sans-serif",
        fontSize: titleSize,
        maxLines: 2,
        wordWrap: { width: width - 18, useAdvancedWrap: true }
      })
      .setOrigin(0.5);
    const price = this.add
      .text(0, height * (compactCard ? 0.12 : 0.07), `${item.tier === "income" ? "入账 +" : "消费 "}${formatMoney(total)}`, {
        align: "center",
        color: item.tier === "income" ? "#80ffab" : "#ffd65a",
        fontFamily: "'Arial Narrow', 'PingFang SC', sans-serif",
        fontSize: priceSize,
        fontStyle: "900"
      })
      .setOrigin(0.5);
    const meta = this.add
      .text(0, height * 0.34, `${item.category} · 单价 ${formatMoney(item.price)}`, {
        align: "center",
        color: "#fff4dc",
        fontFamily: "'Arial Narrow', 'PingFang SC', sans-serif",
        fontSize: width < 150 ? "10px" : "12px"
      })
      .setOrigin(0.5);

    /*
     * backplate 是卡片背后的实体底板。它比正面只多出 5 像素，并向下错开 5 像素，
     * 因而会露出一圈暗金边和一个较厚的下沿，看起来像原画被装进了金属卡座，而不是一张
     * 图片直接浮在黑色画布上。底板先加入 Container，之后才加入原画、文字和正面边框，
     * 所以它不会覆盖任何信息；点击仍然使用 renderHand 计算的原卡片矩形，不会因为装饰
     * 多出几像素就让相邻卡片的命中范围重叠。
     */
    const backplateLeft = -width / 2 - CARD_BACKPLATE_OVERHANG_PX;
    const backplateTop = -height / 2 - CARD_BACKPLATE_OVERHANG_PX + CARD_BACKPLATE_DROP_PX;
    const backplateWidth = width + CARD_BACKPLATE_OVERHANG_PX * 2;
    const backplateHeight = height + CARD_BACKPLATE_OVERHANG_PX * 2;
    backplate.fillStyle(0x020202, 0.82);
    backplate.fillRoundedRect(backplateLeft - 3, backplateTop + 4, backplateWidth + 6, backplateHeight + 4, 10);
    backplate.fillStyle(palette.shadow, 0.98);
    backplate.fillRoundedRect(backplateLeft, backplateTop, backplateWidth, backplateHeight, 9);
    backplate.lineStyle(2, palette.stroke, 0.66);
    backplate.strokeRoundedRect(backplateLeft + 1, backplateTop + 1, backplateWidth - 2, backplateHeight - 2, 8);
    backplate.lineStyle(1, palette.highlight, 0.42);
    backplate.lineBetween(backplateLeft + 12, backplateTop + 3, backplateLeft + backplateWidth * 0.44, backplateTop + 3);
    backplate.fillStyle(0x050403, 0.82);
    backplate.fillRoundedRect(backplateLeft + 8, height / 2 + 1, backplateWidth - 16, 7, 3);
    backplate.lineStyle(1, badgeColor, 0.46);
    backplate.lineBetween(backplateLeft + 16, height / 2 + 3, backplateLeft + backplateWidth - 16, height / 2 + 3);

    shape.fillStyle(palette.fill, 0.97);
    shape.fillRoundedRect(-width / 2, -height / 2, width, height, 8);
    shape.lineStyle(5, palette.shadow, 0.98);
    shape.strokeRoundedRect(-width / 2, -height / 2, width, height, 8);
    shape.lineStyle(2, palette.stroke, 0.92);
    shape.strokeRoundedRect(-width / 2 + 3, -height / 2 + 3, width - 6, height - 6, 7);
    shape.lineStyle(1, palette.highlight, 0.32);
    shape.strokeRoundedRect(-width / 2 + 8, -height / 2 + 8, width - 16, height - 16, 5);

    const artwork = this.createArtworkImage(item, width - ARTWORK_INSET_PX, height - ARTWORK_INSET_PX);
    if (artwork) {
      const finish = this.createArtworkFinish(width, height, item, card.multiplier.multiplier);
      const plaqueTrim = this.add.graphics();

      /*
       * 原画本身已经承担“这是什么消费”的视觉说明，所以普通卡不再把分类和单价压在图片
       * 底部。标题与本次金额分别收进左上、左下两个紧凑标签，中间区域完全留给原画；更完整
       * 的分类、单价、倍率和 flavor 仍会在点击后的账单背面出现，不会丢失信息。
       */
      label
        .setPosition(-width / 2 + 9, -height / 2 + 8)
        .setOrigin(0, 0)
        .setWordWrapWidth(width - (compactCard ? 42 : 82), true)
        .setBackgroundColor(ARTWORK_LABEL_BACKGROUND)
        .setPadding(compactCard ? 4 : 7, compactCard ? 2 : 3, compactCard ? 4 : 7, compactCard ? 2 : 3)
        .setStroke("#050505", compactCard ? 2 : 3);
      price
        .setPosition(-width / 2 + 9, height / 2 - 8)
        .setOrigin(0, 1)
        .setBackgroundColor(ARTWORK_PRICE_BACKGROUND)
        .setPadding(compactCard ? 4 : 7, compactCard ? 2 : 3, compactCard ? 4 : 7, compactCard ? 2 : 3)
        .setStroke("#050505", compactCard ? 2 : 3);
      plaqueTrim.lineStyle(compactCard ? 1 : 2, finish.accentColor, 0.94);
      plaqueTrim.lineBetween(
        -width / 2 + 9,
        label.y + label.displayHeight + 2,
        -width / 2 + 9 + Math.min(label.displayWidth, width * 0.58),
        label.y + label.displayHeight + 2
      );
      plaqueTrim.lineStyle(1, palette.highlight, 0.7);
      plaqueTrim.lineBetween(
        -width / 2 + 9,
        price.y - price.displayHeight - 2,
        -width / 2 + 9 + Math.min(price.displayWidth, width * 0.64),
        price.y - price.displayHeight - 2
      );
      meta.setVisible(false);
      container.add([backplate, shape, finish.aura, artwork, finish.light, finish.frame, plaqueTrim]);
    } else {
      container.add([backplate, shape]);
    }
    container.add([badgeFrame, multiplierBadge, label, price, meta]);
    container.setSize(width, height);

    return container;
  }

  private cardArtworkState(item: Item): "loaded" | "fallback" | "unmapped" {
    const asset = findItemArtwork(item.id);
    if (!asset) {
      return "unmapped";
    }

    return this.textures.exists(asset.textureKey) ? "loaded" : "fallback";
  }

  private createArtworkImage(item: Item, width: number, height: number): Phaser.GameObjects.Image | null {
    const asset = findItemArtwork(item.id);
    if (!asset || !this.textures.exists(asset.textureKey)) {
      return null;
    }

    const image = this.add.image(0, 0, asset.textureKey).setName(asset.textureKey);
    const sourceWidth = image.width;
    const sourceHeight = image.height;
    const sourceRatio = sourceWidth / sourceHeight;
    const targetRatio = width / height;
    let cropX = 0;
    let cropY = 0;
    let cropWidth = sourceWidth;
    let cropHeight = sourceHeight;

    /*
     * 普通货架卡和放大翻牌的宽高比例不同。如果直接 setDisplaySize，人物、手机和票据会被
     * 横向或纵向拉伸。这里先从原图中心裁出与目标区域相同的比例，再等比缩放到目标尺寸。
     * 两个卡面会各自创建一个 Image 显示对象，但它们使用同一个 textureKey，因此浏览器只
     * 下载、解码并上传一份纹理；“共用原画”不会变成两套资源或两次网络请求。
     */
    if (sourceRatio > targetRatio) {
      cropWidth = sourceHeight * targetRatio;
      cropX = (sourceWidth - cropWidth) / 2;
    } else if (sourceRatio < targetRatio) {
      cropHeight = sourceWidth / targetRatio;
      cropY = (sourceHeight - cropHeight) / 2;
    }

    image.setCrop(cropX, cropY, cropWidth, cropHeight);
    image.setScale(width / cropWidth, height / cropHeight);
    return image;
  }

  private artworkAccentColor(item: Item, multiplier: number): number {
    if (item.tier === "income") {
      return 0x5fff9c;
    }
    if (multiplier > 1) {
      return 0x57fff3;
    }
    return TIER_COLORS[item.tier].stroke;
  }

  private createArtworkFinish(
    width: number,
    height: number,
    item: Item,
    multiplier: number
  ): {
    aura: Phaser.GameObjects.Graphics;
    light: Phaser.GameObjects.Graphics;
    frame: Phaser.GameObjects.Graphics;
    accentColor: number;
  } {
    const tierPalette = TIER_COLORS[item.tier];
    const accentColor = this.artworkAccentColor(item, multiplier);
    const highlightColor = multiplier > 1 ? 0xe5fffc : tierPalette.highlight;
    const glowColor = multiplier > 1 ? 0x57fff3 : tierPalette.glow;
    const aura = this.add.graphics().setBlendMode(Phaser.BlendModes.ADD);
    const light = this.add.graphics().setBlendMode(Phaser.BlendModes.ADD);
    const frame = this.add.graphics();
    const inset = ARTWORK_INSET_PX / 2;
    const left = -width / 2 + 1;
    const top = -height / 2 + 1;
    const right = width / 2 - 1;
    const bottom = height / 2 - 1;
    const cornerLength = Phaser.Math.Clamp(width * 0.09, 11, 26);

    /*
     * 这三层只负责给已有原画增加卡牌质感，不修改图片像素。aura 在图片后面画两圈低透明
     * 辉光；light 在图片上方从左上向右下叠一层很淡的暖色高光；frame 最后重新画清晰边框，
     * 避免发光把卡片轮廓冲散。它们都是 Phaser Graphics，因此无图回退、金额算法和命中矩形
     * 不受影响，也不需要为每种商品再制作一张发光版本。
     */
    aura.lineStyle(20, glowColor, 0.09);
    aura.strokeRoundedRect(-width / 2 + inset, -height / 2 + inset, width - ARTWORK_INSET_PX, height - ARTWORK_INSET_PX, 9);
    aura.lineStyle(9, glowColor, 0.24);
    aura.strokeRoundedRect(-width / 2 + inset, -height / 2 + inset, width - ARTWORK_INSET_PX, height - ARTWORK_INSET_PX, 8);

    light.fillGradientStyle(highlightColor, accentColor, highlightColor, accentColor, 0.18, 0.055, 0, 0);
    light.fillRect(-width / 2 + inset, -height / 2 + inset, width - ARTWORK_INSET_PX, height - ARTWORK_INSET_PX);
    light.lineStyle(2, highlightColor, 0.78);
    light.lineBetween(-width / 2 + 12, -height / 2 + 7, width * 0.16, -height / 2 + 7);

    frame.lineStyle(7, tierPalette.shadow, 0.98);
    frame.strokeRoundedRect(left, top, width - 2, height - 2, 8);
    frame.lineStyle(4, accentColor, 0.98);
    frame.strokeRoundedRect(left + 2, top + 2, width - 6, height - 6, 7);
    frame.lineStyle(1, highlightColor, 0.92);
    frame.strokeRoundedRect(left + 6, top + 6, width - 14, height - 14, 5);
    frame.lineStyle(2, accentColor, 0.58);
    frame.strokeRoundedRect(left + 10, top + 10, width - 22, height - 22, 4);

    /*
     * 四角短线模拟珠宝盒和高级信用卡的金属包角。它们只画在线框内部，不覆盖标题、金额
     * 或原画主体；短线长度会随卡片宽度限制在 11 到 26 像素，移动端窄卡不会被角饰挤满。
     */
    frame.lineStyle(2, highlightColor, 0.9);
    frame.lineBetween(left + 8, top + 8, left + 8 + cornerLength, top + 8);
    frame.lineBetween(left + 8, top + 8, left + 8, top + 8 + cornerLength * 0.65);
    frame.lineBetween(right - 8 - cornerLength, top + 8, right - 8, top + 8);
    frame.lineBetween(right - 8, top + 8, right - 8, top + 8 + cornerLength * 0.65);
    frame.lineBetween(left + 8, bottom - 8, left + 8 + cornerLength, bottom - 8);
    frame.lineBetween(left + 8, bottom - 8 - cornerLength * 0.65, left + 8, bottom - 8);
    frame.lineBetween(right - 8 - cornerLength, bottom - 8, right - 8, bottom - 8);
    frame.lineBetween(right - 8, bottom - 8 - cornerLength * 0.65, right - 8, bottom - 8);

    return { aura, light, frame, accentColor };
  }

  private animateCardIn(container: Phaser.GameObjects.Container, index: number): void {
    container.setAlpha(0);
    container.setScale(0.08, 0.96);
    container.setY(container.y + 10);

    /*
     * 这里做的是一张一张翻进来的卡牌动画。它只改变容器显示，不改变 hitBounds，所以
     * 点击判定仍然由上面的矩形区域统一负责。这样用户看到的是类似 PPT 翻页/发牌的节奏，
     * 但交互逻辑不会因为动画缩放而变得忽大忽小。
     */
    this.tweens.add({
      targets: container,
      alpha: 1,
      scaleX: 1,
      scaleY: 1,
      y: container.y - 10,
      delay: index * 36,
      duration: 260,
      ease: "Back.easeOut"
    });
  }

  private showCardDetailFlip(card: CardSlot): void {
    this.cardDetailOverlay?.destroy();

    const item = card.item;
    const total = this.cardTotal(card);
    const width = Phaser.Math.Clamp(this.scale.width * 0.38, 320, 460);
    const height = Phaser.Math.Clamp(this.scale.height * 0.34, 210, 270);
    const centerX = this.scale.width / 2;
    const centerY = this.scale.height * 0.46;
    const overlay = this.add.container(0, 0).setDepth(150);
    const scrim = this.add
      .rectangle(centerX, centerY, this.scale.width * 1.12, this.scale.height * 1.12, 0x000000, 0.44)
      .setAlpha(0);
    const detailCard = this.add.container(card.x, card.y).setAlpha(0.35).setScale(0.42);
    const front = this.add.container(0, 0);
    const back = this.add.container(0, 0).setVisible(false);
    const frontShape = this.add.graphics();
    const backShape = this.add.graphics();
    const backOrnament = this.add.graphics();
    const frontPalette = TIER_COLORS[item.tier];
    const backAccent = item.tier === "income" ? 0x55f39a : frontPalette.stroke;
    const detailExitDelayMs = this.cardDetailExitDelayMs();

    frontShape.fillStyle(frontPalette.fill, 0.98);
    frontShape.fillRoundedRect(-width / 2, -height / 2, width, height, 10);
    frontShape.lineStyle(6, frontPalette.shadow, 0.98);
    frontShape.strokeRoundedRect(-width / 2, -height / 2, width, height, 10);
    frontShape.lineStyle(3, frontPalette.stroke, 0.94);
    frontShape.strokeRoundedRect(-width / 2 + 4, -height / 2 + 4, width - 8, height - 8, 8);
    frontShape.lineStyle(1, frontPalette.highlight, 0.48);
    frontShape.strokeRoundedRect(-width / 2 + 10, -height / 2 + 10, width - 20, height - 20, 6);

    const frontTitle = this.add
      .text(0, -height * 0.2, item.name, {
        align: "center",
        color: frontPalette.text,
        fontFamily: "Impact, Haettenschweiler, 'Arial Narrow Bold', sans-serif",
        fontSize: "30px",
        maxLines: 2,
        wordWrap: { width: width - 44, useAdvancedWrap: true }
      })
      .setOrigin(0.5);
    const frontPrice = this.add
      .text(0, height * 0.13, `${item.tier === "income" ? "入账" : "消费"} ${item.tier === "income" ? "+" : "-"}${formatMoney(total)}`, {
        align: "center",
        color: item.tier === "income" ? "#80ffab" : "#ffd65a",
        fontFamily: "Impact, Haettenschweiler, 'Arial Narrow Bold', sans-serif",
        fontSize: "34px",
        stroke: "#050505",
        strokeThickness: 4
      })
      .setOrigin(0.5);
    const frontMeta = this.add
      .text(0, height * 0.36, `${item.category} · ${card.multiplier.label}`, {
        align: "center",
        color: "#fff4dc",
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
        fontSize: "14px",
        fontStyle: "900"
      })
      .setOrigin(0.5);

    backShape.fillStyle(0x0b0908, 0.99);
    backShape.fillRoundedRect(-width / 2, -height / 2, width, height, 10);
    backShape.lineStyle(7, frontPalette.shadow, 0.98);
    backShape.strokeRoundedRect(-width / 2, -height / 2, width, height, 10);
    backShape.lineStyle(3, backAccent, 0.98);
    backShape.strokeRoundedRect(-width / 2 + 4, -height / 2 + 4, width - 8, height - 8, 8);
    backShape.lineStyle(1, frontPalette.highlight, 0.68);
    backShape.strokeRoundedRect(-width / 2 + 10, -height / 2 + 10, width - 20, height - 20, 6);

    backOrnament.lineStyle(1, backAccent, 0.52);
    backOrnament.lineBetween(-width * 0.34, -height * 0.12, width * 0.34, -height * 0.12);
    backOrnament.lineBetween(-width * 0.24, height * 0.27, width * 0.24, height * 0.27);
    backOrnament.lineStyle(2, frontPalette.highlight, 0.72);
    backOrnament.lineBetween(-width / 2 + 16, -height / 2 + 16, -width / 2 + 54, -height / 2 + 16);
    backOrnament.lineBetween(width / 2 - 54, -height / 2 + 16, width / 2 - 16, -height / 2 + 16);
    backOrnament.lineBetween(-width / 2 + 16, height / 2 - 16, -width / 2 + 54, height / 2 - 16);
    backOrnament.lineBetween(width / 2 - 54, height / 2 - 16, width / 2 - 16, height / 2 - 16);

    const backTitle = this.add
      .text(0, -height * 0.28, "账单详情", {
        align: "center",
        color: item.tier === "income" ? "#8affb5" : "#ffe28a",
        fontFamily: "Impact, Haettenschweiler, 'Arial Narrow Bold', sans-serif",
        fontSize: "28px"
      })
      .setOrigin(0.5);
    const backDetail = this.add
      .text(0, height * 0.02, `${item.flavor}\n${item.category} · 单价 ${formatMoney(item.price)} · 倍率 ${card.multiplier.label}\n本次${item.tier === "income" ? "入账" : "扣款"} ${formatMoney(total)}`, {
        align: "center",
        color: "#fff4dc",
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
        fontSize: "17px",
        lineSpacing: 8,
        wordWrap: { width: width - 48 }
      })
      .setOrigin(0.5);

    /*
     * front 是玩家点击后先看到的放大正面，它通过 createArtworkImage 与普通货架卡共用
     * 同一个 textureKey；back 仍然是程序绘制的账单详情。以后美术如果为每张商品再提供
     * 一张详情图，只需要替换 back 的背景，不能另建一套正面 id 映射。这样普通卡、放大
     * 正面和未来详情面的职责保持清楚，支付、命中、结算逻辑也不需要跟着图片格式变化。
     */
    const frontArtwork = this.createArtworkImage(item, width - ARTWORK_INSET_PX, height - ARTWORK_INSET_PX);
    if (frontArtwork) {
      const finish = this.createArtworkFinish(width, height, item, card.multiplier.multiplier);
      frontTitle
        .setPosition(-width / 2 + 16, -height / 2 + 14)
        .setOrigin(0, 0)
        .setWordWrapWidth(width - 118, true)
        .setBackgroundColor(ARTWORK_LABEL_BACKGROUND)
        .setPadding(9, 5, 9, 5)
        .setStroke("#050505", 4);
      frontPrice
        .setPosition(-width / 2 + 16, height / 2 - 14)
        .setOrigin(0, 1)
        .setBackgroundColor(ARTWORK_PRICE_BACKGROUND)
        .setPadding(10, 5, 10, 5);
      frontMeta.setVisible(false);
      front.add([frontShape, finish.aura, frontArtwork, finish.light, finish.frame]);
    } else {
      front.add(frontShape);
    }
    front.add([frontTitle, frontPrice, frontMeta]);
    back.add([backShape, backOrnament, backTitle, backDetail]);
    detailCard.add([front, back]);
    overlay.add([scrim, detailCard]);
    this.cardDetailOverlay = overlay;

    this.tweens.add({
      targets: scrim,
      alpha: 1,
      duration: 180,
      ease: "Cubic.easeOut"
    });
    this.tweens.add({
      targets: detailCard,
      x: centerX,
      y: centerY,
      alpha: 1,
      scaleX: 1,
      scaleY: 1,
      duration: 230,
      ease: "Cubic.easeOut"
    });
    this.tweens.add({
      targets: front,
      scaleX: 0.04,
      delay: 190,
      duration: CARD_DETAIL_FLIP_MS,
      ease: "Cubic.easeIn",
      onComplete: () => {
        front.setVisible(false);
        back.setScale(0.04, 1);
        back.setVisible(true);
        this.tweens.add({
          targets: back,
          scaleX: 1,
          duration: CARD_DETAIL_FLIP_MS,
          ease: "Cubic.easeOut"
        });
      }
    });
    this.tweens.add({
      targets: detailCard,
      alpha: 0,
      y: centerY - 18,
      delay: detailExitDelayMs,
      duration: CARD_DETAIL_EXIT_MS,
      ease: "Cubic.easeIn"
    });
    this.tweens.add({
      targets: scrim,
      alpha: 0,
      delay: detailExitDelayMs,
      duration: CARD_DETAIL_EXIT_MS,
      ease: "Cubic.easeIn",
      onComplete: () => {
        if (this.cardDetailOverlay === overlay) {
          this.cardDetailOverlay = null;
        }
        overlay.destroy();
      }
    });
  }

  private cardDetailExitDelayMs(): number {
    /*
     * 详情翻牌是玩家确认“刚才点中了哪张卡”的视觉反馈，不应该比收银锁晚很多消失。
     * 当前点击用的是原生 canvas 事件转发，而不是 Phaser 的交互层级；如果详情层还盖在画面上
     * 但收银锁已经结束，浏览器点击会直接穿透到新货架，玩家会看到“详情页没退场却又刷了一张”。
     * 这里让退场动画在收银锁结束时基本完成，同时保留一个最短展示时间，避免低配置或未来调短
     * selectionSettleMs 时详情卡一闪而过。
     */
    return Math.max(CARD_DETAIL_MIN_EXIT_DELAY_MS, this.tuning.selectionSettleMs - CARD_DETAIL_EXIT_MS);
  }

  private pickMultiplier(item: Item): MultiplierRule {
    const fallback = this.tuning.multiplierRules.find((rule) => rule.multiplier === 1) ?? DEFAULT_BALANCE_TUNING.multiplierRules[0];
    if (!item.batchable || item.tier === "income") {
      return fallback;
    }

    const eligible = this.tuning.multiplierRules.filter((rule) => {
      const total = item.price * rule.multiplier;
      return (
        this.state.balance >= rule.minBalance &&
        item.price <= rule.maxUnitPrice &&
        total <= rule.maxTotalPrice &&
        (rule.multiplier === 1 || total < this.state.balance)
      );
    });

    return this.pickWeightedMultiplier(eligible.length > 0 ? eligible : [fallback]);
  }

  private pickWeightedMultiplier(rules: MultiplierRule[]): MultiplierRule {
    const totalWeight = rules.reduce((sum, rule) => sum + Math.max(0.01, rule.weight), 0);
    let roll = Math.random() * totalWeight;

    for (const rule of rules) {
      roll -= Math.max(0.01, rule.weight);
      if (roll <= 0) {
        return rule;
      }
    }

    return rules[0];
  }

  private paymentLineFromCard(card: CardSlot): PaymentLine {
    return {
      item: card.item,
      multiplier: card.multiplier
    };
  }

  private cardTotal(card: CardSlot): number {
    return this.lineTotal(this.paymentLineFromCard(card));
  }

  private lineTotal(line: PaymentLine): number {
    return Math.round(line.item.price * line.multiplier.multiplier);
  }

  private describeLine(line: PaymentLine): string {
    return line.multiplier.multiplier > 1 ? `${line.item.name} ${line.multiplier.label}` : line.item.name;
  }

  private cardIsAffordable(card: CardSlot): boolean {
    return this.itemHasRemainingUses(card.item) && (!isSpend(card.item) || this.cardTotal(card) <= this.state.balance);
  }

  private maxBuyForItem(item: Item): number | null {
    if (item.maxBuy === null || item.maxBuy === undefined) {
      return null;
    }

    const maxBuy = Math.floor(item.maxBuy);
    return maxBuy > 0 ? maxBuy : null;
  }

  private itemUseCount(item: Item): number {
    return (this.purchasedItemCounts.get(item.id) ?? 0) + (this.pendingItemCounts.get(item.id) ?? 0);
  }

  private itemHasRemainingUses(item: Item): boolean {
    const maxBuy = this.maxBuyForItem(item);
    if (maxBuy === null) {
      return true;
    }

    return this.itemUseCount(item) < maxBuy;
  }

  private itemCanBeUsedInPayment(item: Item, localUseCounts: Map<string, number>): boolean {
    const maxBuy = this.maxBuyForItem(item);
    if (maxBuy === null) {
      return true;
    }

    return this.itemUseCount(item) + (localUseCounts.get(item.id) ?? 0) < maxBuy;
  }

  private incrementLocalUseCount(localUseCounts: Map<string, number>, item: Item): void {
    if (this.maxBuyForItem(item) === null) {
      return;
    }

    localUseCounts.set(item.id, (localUseCounts.get(item.id) ?? 0) + 1);
  }

  private addItemCount(counts: Map<string, number>, item: Item, delta: number): void {
    if (this.maxBuyForItem(item) === null) {
      return;
    }

    const nextCount = (counts.get(item.id) ?? 0) + delta;
    if (nextCount <= 0) {
      counts.delete(item.id);
      return;
    }
    counts.set(item.id, nextCount);
  }

  private reservePendingLines(lines: PaymentLine[]): boolean {
    const localUseCounts = new Map<string, number>();

    for (const line of lines) {
      if (!this.itemCanBeUsedInPayment(line.item, localUseCounts)) {
        return false;
      }
      this.incrementLocalUseCount(localUseCounts, line.item);
    }

    /*
     * VISA 和清空购物车都是延迟扣款。玩家按下技能后，卡片还没有真正扣钱，但这张
     * maxBuy=1 的一次性商品已经被本次待结算占住了。如果不先记录 pending，下一手货架
     * 可能又抽到同一张高价卡，最终让“最多一次”的数据库字段失效。
     */
    for (const line of lines) {
      this.addItemCount(this.pendingItemCounts, line.item, 1);
    }

    return true;
  }

  private releasePendingLines(lines: PaymentLine[]): void {
    for (const line of lines) {
      this.addItemCount(this.pendingItemCounts, line.item, -1);
    }
  }

  private recordPurchasedLine(line: PaymentLine): void {
    this.addItemCount(this.purchasedItemCounts, line.item, 1);
  }

  private affordableSpendCards(): CardSlot[] {
    return this.cards.filter((card) => !card.selected && isSpend(card.item) && this.cardIsAffordable(card));
  }

  private affordableClearCartPreview(): CardSlot[] {
    return this.pickClearCartCards({ deterministic: true });
  }

  private pickClearCartCards(options: { deterministic?: boolean } = {}): CardSlot[] {
    const targetCount = Math.max(1, this.tuning.clearCartPickCount);
    const source = this.cards.filter((card) => !card.selected && isSpend(card.item));
    const candidates = (options.deterministic ? [...source].sort((first, second) => this.cardTotal(first) - this.cardTotal(second)) : Phaser.Utils.Array.Shuffle([...source]));
    const picked: CardSlot[] = [];
    const localUseCounts = new Map<string, number>();
    let remainingBalance = this.state.balance;

    /*
     * 清空购物车不是“无视余额的必杀技”，它只是批量选择当前货架里的消费。这里按剩余余额
     * 逐张确认能不能支付，避免出现余额只剩几千元却还能强行刷走十几万的漏洞。随机尝试不够
     * 3 张时，再用最便宜的组合兜底一次，保证按钮禁用和实际执行都尽量稳定。
     */
    for (const card of candidates) {
      const total = this.cardTotal(card);
      if (this.itemCanBeUsedInPayment(card.item, localUseCounts) && total <= remainingBalance) {
        picked.push(card);
        this.incrementLocalUseCount(localUseCounts, card.item);
        remainingBalance -= total;
      }

      if (picked.length >= targetCount) {
        return picked;
      }
    }

    if (options.deterministic) {
      return picked;
    }

    return this.pickClearCartCards({ deterministic: true });
  }

  private handleCanvasPointerDown(x: number, y: number): void {
    const now = performance.now();
    if (
      this.state.status !== "running" ||
      this.cardDetailOverlay !== null ||
      this.stopIfPastRoundLimit(now) ||
      this.waitingForCheckoutRefresh ||
      now < this.checkoutLockedUntil
    ) {
      return;
    }

    const clickedCard = this.cards.find(
      (card) => !card.selected && Phaser.Geom.Rectangle.Contains(card.hitBounds, x, y)
    );
    if (!clickedCard) {
      return;
    }

    this.selectCard(clickedCard);
  }

  private selectCard(card: CardSlot): void {
    if (this.state.status !== "running" || card.selected) {
      return;
    }

    const now = performance.now();
    if (this.stopIfPastRoundLimit(now)) {
      return;
    }
    if (now < this.handFrozenUntil) {
      this.pushSystemFeed("购物车结算中", "当前货架已锁定，结算完成后才会刷新");
      return;
    }

    if (!this.itemHasRemainingUses(card.item)) {
      this.showLimitReached(card);
      this.pushSystemFeed("已达上限", `${card.item.name} 是一次性消费，本局已经买过或正在结算`);
      this.updateActionTimers(now);
      this.emitState();
      return;
    }

    if (!this.cardIsAffordable(card)) {
      this.showCannotAfford(card);
      this.pushSystemFeed(
        "买不起",
        `${card.item.name} 需要 ${formatMoney(this.cardTotal(card))}，当前余额 ${formatMoney(this.state.balance)}`
      );
      this.updateActionTimers(now);
      this.emitState();
      return;
    }

    card.selected = true;
    card.container?.setAlpha(0.62);
    this.showCardDetailFlip(card);
    this.checkoutLockedUntil = now + this.tuning.selectionSettleMs;
    this.waitingForCheckoutRefresh = true;
    this.applyPayment([this.paymentLineFromCard(card)], "刷卡付款");
    if (this.state.status !== "running") {
      return;
    }
    this.updateActionTimers(now);
    this.renderHandTimer(now);
  }

  private markCardPending(card: CardSlot, label: string): void {
    card.container?.setAlpha(0.58);

    const overlay = this.add
      .text(card.x, card.y, label, {
        align: "center",
        color: "#050505",
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
        fontSize: "15px",
        fontStyle: "900",
        backgroundColor: "#ffd15e",
        padding: { left: 9, right: 9, top: 5, bottom: 5 },
        wordWrap: { width: 150 }
      })
      .setOrigin(0.5)
      .setDepth(130);

    this.tweens.add({
      targets: overlay,
      alpha: 0,
      y: overlay.y - 20,
      duration: 900,
      ease: "Cubic.easeOut",
      onComplete: () => overlay.destroy()
    });
  }

  private showCannotAfford(card: CardSlot): void {
    const missing = Math.max(1, this.cardTotal(card) - this.state.balance);
    const overlay = this.add
      .text(card.x, card.y, `买不起\n还差 ${formatMoney(missing)}`, {
        align: "center",
        color: "#050505",
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
        fontSize: "15px",
        fontStyle: "900",
        backgroundColor: "#ffdd78",
        padding: { left: 10, right: 10, top: 6, bottom: 6 },
        wordWrap: { width: 170 }
      })
      .setOrigin(0.5)
      .setDepth(135);

    if (card.container) {
      this.tweens.add({
        targets: card.container,
        x: {
          from: card.x - 8,
          to: card.x
        },
        duration: 95,
        yoyo: true,
        repeat: 2,
        ease: "Sine.easeInOut"
      });
    }

    this.tweens.add({
      targets: overlay,
      alpha: 0,
      y: overlay.y - 18,
      duration: 760,
      ease: "Cubic.easeOut",
      onComplete: () => overlay.destroy()
    });
  }

  private showLimitReached(card: CardSlot): void {
    const overlay = this.add
      .text(card.x, card.y, "已买过\n本局限购", {
        align: "center",
        color: "#050505",
        fontFamily: "'PingFang SC', 'Microsoft YaHei', sans-serif",
        fontSize: "15px",
        fontStyle: "900",
        backgroundColor: "#57fff3",
        padding: { left: 10, right: 10, top: 6, bottom: 6 },
        wordWrap: { width: 170 }
      })
      .setOrigin(0.5)
      .setDepth(135);

    if (card.container) {
      this.tweens.add({
        targets: card.container,
        angle: {
          from: -2,
          to: 2
        },
        duration: 80,
        yoyo: true,
        repeat: 3,
        ease: "Sine.easeInOut",
        onComplete: () => card.container?.setAngle(0)
      });
    }

    this.tweens.add({
      targets: overlay,
      alpha: 0,
      y: overlay.y - 18,
      duration: 760,
      ease: "Cubic.easeOut",
      onComplete: () => overlay.destroy()
    });
  }

  private applyPayment(lines: PaymentLine[], actionLabel: string, effectiveNow = performance.now(), scene = this.currentScene()): void {
    let spent = 0;
    let income = 0;
    const names: string[] = [];
    const skippedNames: string[] = [];
    const paidLines: PaymentLine[] = [];
    const localUseCounts = new Map<string, number>();
    let availableBalance = this.state.balance;

    for (const line of lines) {
      const item = line.item;
      const total = this.lineTotal(line);
      if (!this.itemCanBeUsedInPayment(item, localUseCounts)) {
        skippedNames.push(`${this.describeLine(line)} 已达本局购买上限`);
        continue;
      }

      if (item.tier === "income") {
        income += total;
        availableBalance += total;
        names.push(this.describeLine(line));
        paidLines.push(line);
        this.incrementLocalUseCount(localUseCounts, item);
      } else {
        if (total <= availableBalance) {
          spent += total;
          availableBalance -= total;
          names.push(this.describeLine(line));
          paidLines.push(line);
          this.incrementLocalUseCount(localUseCounts, item);
        } else {
          skippedNames.push(`${this.describeLine(line)} 余额不足`);
        }
      }
    }

    if (paidLines.length === 0) {
      this.pushSystemFeed(`${actionLabel}失败`, `${skippedNames.join("、")}，未扣款`);
      this.emitState();
      return;
    }

    if (income > 0) {
      this.state.balance += income;
      this.state.totalIncome += income;
    }

    if (spent > 0) {
      this.state.balance -= spent;
      this.state.totalSpent += spent;
      /*
       * maxSingleSpend 用于排行榜和结算摘要，应该表达“一次结算动作最多花了多少”，而不是
       * 只看这次动作里某一张卡的金额。普通刷卡和 VISA 只有一条 line，所以结果不变；清空
       * 购物车会把 3 张消费卡作为一次延迟结算，此时用合计扣款才能和玩家看到的账单一致。
       */
      this.state.maxSingleSpend = Math.max(this.state.maxSingleSpend, spent);
    }

    this.settlementStats.paymentCount += 1;
    for (const line of paidLines) {
      this.recordPurchasedLine(line);
      this.recordPaymentLine(line, actionLabel);
    }
    this.rememberPaymentActionHighlight(paidLines, actionLabel, spent, income);

    const net = spent - income;
    const kind: GameFeedEvent["kind"] = net >= 0 ? "spend" : "income";
    this.callbacks.onTone(net < 0 ? "income" : spent >= this.tuning.highPriceThreshold ? "danger" : "spend");
    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title: actionLabel,
      detail: `${names.join("、")} ${net >= 0 ? "-" : "+"}${formatMoney(Math.abs(net))}`,
      kind
    });
    if (skippedNames.length > 0) {
      this.pushSystemFeed("部分未扣款", `${skippedNames.join("、")} 未扣款`);
    }
    this.showCheckoutPulse(net);
    this.emitState();

    if (this.state.balance <= 0) {
      this.endRound("balance_zero", undefined, effectiveNow);
      return;
    }

    /*
     * 普通刷卡会马上结算，所以默认使用当前场景即可。VISA 和清空购物车会延迟扣款，
     * 这时到期时间可能已经跨过 35 秒阶段线；如果继续读取 currentScene()，就会让旧场景
     * 货架里的商品触发新场景的事件、状态或终局。这里把付款发起时的场景一路传下来，
     * 保证数据库里“商品属于哪个场景”和“事件按哪个场景匹配”保持一致。
     */
    this.maybeTriggerChaosEvent(paidLines, effectiveNow, scene);
    this.maybeTriggerStatusEffect(paidLines, effectiveNow, scene);
    this.maybeTriggerTerminalEvent(paidLines, effectiveNow, scene);
  }

  private showCheckoutPulse(netAmount: number): void {
    const isIncome = netAmount < 0;
    const amount = Math.abs(netAmount);
    const targetX = this.scale.width * PAYMENT_LANE_X_RATIO;
    const targetY = this.scale.height * PAYMENT_LANE_Y_RATIO;

    this.paymentGraphics.clear();
    this.paymentGraphics.fillStyle(isIncome ? 0x65ff9b : 0xffd15e, 0.18);
    this.paymentGraphics.fillRoundedRect(
      targetX - this.scale.width * 0.14,
      targetY - this.scale.height * 0.055,
      this.scale.width * 0.28,
      this.scale.height * 0.11,
      10
    );
    this.paymentGraphics.lineStyle(2, isIncome ? 0x65ff9b : 0xffd15e, 0.55);
    this.paymentGraphics.strokeRoundedRect(
      targetX - this.scale.width * 0.14,
      targetY - this.scale.height * 0.055,
      this.scale.width * 0.28,
      this.scale.height * 0.11,
      10
    );
    this.tweens.add({
      targets: this.paymentGraphics,
      alpha: 0,
      duration: 240,
      onComplete: () => {
        this.paymentGraphics.clear();
        this.paymentGraphics.setAlpha(1);
      }
    });

    const floatingText = this.add
      .text(targetX, targetY - 24, `${isIncome ? "入账" : "已付"} ${isIncome ? "+" : "-"}${formatMoney(amount)}`, {
        color: isIncome ? "#7cff9d" : "#ffd45a",
        fontFamily: "Impact, Haettenschweiler, 'Arial Narrow Bold', sans-serif",
        fontSize: "30px",
        stroke: "#050505",
        strokeThickness: 5
      })
      .setOrigin(0.5)
      .setDepth(120);

    this.tweens.add({
      targets: floatingText,
      y: floatingText.y - 72,
      alpha: 0,
      duration: 780,
      ease: "Cubic.easeOut",
      onComplete: () => floatingText.destroy()
    });
  }

  private maybeTriggerChaosEvent(lines: PaymentLine[], now = performance.now(), scene = this.currentScene()): void {
    const spendLines = lines.filter((line) => isSpend(line.item));
    if (spendLines.length === 0 || this.bootstrap.events.length === 0 || this.state.status !== "running") {
      return;
    }

    const itemTags = new Set(spendLines.flatMap((line) => line.item.tags));
    const candidates = Phaser.Utils.Array.Shuffle(this.bootstrap.events).filter(
      (event) => (this.eventCooldownUntil.get(event.id) ?? 0) <= now && this.eventCanTrigger(event, scene, itemTags)
    );

    if (candidates.length === 0 || Math.random() > this.chaosEventTriggerChance(candidates, scene, itemTags, now)) {
      return;
    }

    this.applyChaosEvent(this.pickChaosEvent(candidates, scene, itemTags), now);
  }

  private eventCanTrigger(event: GameEvent, scene: Scene, itemTags: Set<string>): boolean {
    return this.eventAffinityScore(event, scene, itemTags) > 0;
  }

  private chaosEventTriggerChance(candidates: GameEvent[], scene: Scene, itemTags: Set<string>, now: number): number {
    const statusMultiplier = this.currentStatusEffect(now)?.eventMultiplier ?? 1;
    const strongestMatch = Math.max(...candidates.map((event) => this.eventAffinityScore(event, scene, itemTags)));
    const contextBonus = Math.min(this.tuning.eventMatchBonus, strongestMatch * this.tuning.eventMatchBonus * 0.18);
    const chance = this.tuning.eventBaseChance + scene.riskLevel * this.tuning.eventRiskBonus + contextBonus;

    /*
     * 混沌事件是“本次购买后可能发生一件事”，不是“80 条事件每条都独立掷骰”。旧逻辑会让
     * 不匹配的事件也拿到最低 4% 概率，再乘以事件数量后几乎每次购买都会触发，而且容易触发
     * 与当前商品/场景无关的内容。这里先过滤上下文，再对整次购买掷一次全局概率，最后按事件
     * 自身 probability 和匹配强度挑一条，保持随机但不让事件表数量直接放大触发率。
     */
    return Phaser.Math.Clamp(chance * statusMultiplier, 0.04, 0.52);
  }

  private pickChaosEvent(candidates: GameEvent[], scene: Scene, itemTags: Set<string>): GameEvent {
    const weighted = candidates.map((event) => ({
      event,
      weight: Math.max(0.001, event.probability * this.eventAffinityScore(event, scene, itemTags))
    }));
    const totalWeight = weighted.reduce((sum, candidate) => sum + candidate.weight, 0);
    let roll = Math.random() * totalWeight;

    for (const candidate of weighted) {
      roll -= candidate.weight;
      if (roll <= 0) {
        return candidate.event;
      }
    }

    return candidates[0];
  }

  private eventAffinityScore(event: GameEvent, scene: Scene, itemTags: Set<string>): number {
    return event.tags.reduce((score, tag) => {
      if (itemTags.has(tag)) {
        return score + 2.2;
      }
      if (scene.eventTags.includes(tag)) {
        return score + 1.7;
      }
      if (scene.itemTags.includes(tag)) {
        return score + 1.1;
      }

      return score;
    }, 0);
  }

  private maybeTriggerStatusEffect(lines: PaymentLine[], now = performance.now(), scene = this.currentScene()): void {
    const spendLines = lines.filter((line) => isSpend(line.item));
    if (spendLines.length === 0 || this.bootstrap.statuses.length === 0 || this.state.status !== "running") {
      return;
    }

    if (this.activeStatus && now < this.activeStatus.expiresAt) {
      return;
    }

    const itemTags = new Set(spendLines.flatMap((line) => line.item.tags));
    const biggestSpend = Math.max(...spendLines.map((line) => this.lineTotal(line)));
    const statusChance = Phaser.Math.Clamp(0.075 + scene.riskLevel * 0.012 + (biggestSpend >= this.tuning.highPriceThreshold ? 0.045 : 0), 0.06, 0.2);

    if (Math.random() > statusChance) {
      return;
    }

    this.activateStatus(this.pickStatusEffect(scene, itemTags, biggestSpend), now);
  }

  private statusContextTags(scene: Scene, itemTags: Set<string>, biggestSpend: number): Set<string> {
    const contextTags = new Set<string>([
      ...itemTags,
      ...scene.itemTags,
      ...scene.eventTags
    ]);

    if (scene.riskLevel >= 4) {
      contextTags.add("high-risk");
    }
    if (scene.itemTags.includes("income") || scene.eventTags.includes("income") || scene.eventTags.includes("refund")) {
      contextTags.add("income-scene");
    }
    if (biggestSpend >= this.tuning.highPriceThreshold) {
      contextTags.add("big-spend");
    }
    if (this.state.balance <= this.bootstrap.config.initialBalance * 0.12) {
      contextTags.add("low-balance");
    }
    if (this.state.elapsedMs >= this.targetClearMs() * 0.85) {
      contextTags.add("late-game");
    }

    return contextTags;
  }

  private pickStatusEffect(scene: Scene, itemTags: Set<string>, biggestSpend: number): StatusEffect {
    const contextTags = this.statusContextTags(scene, itemTags, biggestSpend);
    const weighted = this.bootstrap.statuses.map((effect) => {
      let weight = 1;
      let matchedTagCount = 0;

      /*
       * 状态效果现在由数据库 tags 参与匹配。itemTags 是这次付款商品自带的标签，
       * scene.itemTags / scene.eventTags 是当前消费场景给出的环境标签，low-balance、
       * late-game、big-spend 这些则是运行时派生出的节奏标签。这样“生气”“好运”
       * “回光返照”这类状态不再靠前端写死状态 id，而是通过内容包描述自己更适合在哪些
       * 上下文里出现。后续改状态倾向时，优先调数据库 seed 或后台内容，不需要重发前端。
       */
      for (const tag of effect.tags ?? []) {
        if (contextTags.has(tag)) {
          matchedTagCount += 1;
        }
      }

      if (matchedTagCount > 0) {
        weight += Math.min(6, matchedTagCount * 2.2);
      }

      return { effect, matchedTagCount, weight: Math.max(0.05, weight) };
    });
    /*
     * 状态效果应该像“这一段时间的情绪和身体状态”，而不是完全随机弹出的文案。上面已经
     * 用商品、场景和运行时节奏生成了 contextTags；如果至少有一个状态能匹配这些标签，
     * 就只在匹配状态里选择。只有当数据库 tags 真的没有覆盖当前上下文时，才回退到全状态
     * 池，保证游戏不会因为内容漏配而崩溃，同时也让正常内容下的状态和消费场景更连贯。
     */
    const contextWeighted = weighted.filter((candidate) => candidate.matchedTagCount > 0);
    const pool = contextWeighted.length > 0 ? contextWeighted : weighted;
    const totalWeight = pool.reduce((sum, candidate) => sum + candidate.weight, 0);
    let roll = Math.random() * totalWeight;

    for (const candidate of pool) {
      roll -= candidate.weight;
      if (roll <= 0) {
        return candidate.effect;
      }
    }

    return pool[0].effect;
  }

  private activateStatus(effect: StatusEffect, now: number): void {
    this.activeStatus = {
      effect,
      expiresAt: now + effect.durationSec * 1000
    };
    this.updateActionTimers(now);
    this.pushSystemFeed("状态变化", `${effect.name}：${effect.description}`);
    this.emitState();
  }

  private maybeTriggerTerminalEvent(lines: PaymentLine[], now = performance.now(), scene = this.currentScene()): void {
    const endings = this.bootstrap.endings ?? [];
    const spendLines = lines.filter((line) => isSpend(line.item));
    if (spendLines.length === 0 || endings.length === 0 || this.state.status !== "running") {
      return;
    }

    const itemTags = new Set(spendLines.flatMap((line) => line.item.tags));
    const candidates = Phaser.Utils.Array.Shuffle(endings).filter((ending) => this.terminalEventCanTrigger(ending, scene, itemTags));

    for (const ending of candidates) {
      if (Math.random() <= this.terminalEventChance(ending, scene)) {
        this.triggerTerminalEvent(ending, now);
        return;
      }
    }
  }

  private terminalEventCanTrigger(ending: TerminalEvent, scene: Scene, itemTags: Set<string>): boolean {
    if (this.state.elapsedMs < ending.minElapsedMs) {
      return false;
    }

    if (ending.maxBalance !== null && this.state.balance > ending.maxBalance) {
      return false;
    }

    if (scene.riskLevel < ending.minRiskLevel) {
      return false;
    }

    const meaningfulTags = ending.tags.filter((tag) => tag !== "ending");
    if (meaningfulTags.length === 0) {
      return true;
    }

    return meaningfulTags.some((tag) => itemTags.has(tag) || scene.itemTags.includes(tag) || scene.eventTags.includes(tag));
  }

  private terminalEventChance(ending: TerminalEvent, scene: Scene): number {
    const riskMultiplier = 1 + scene.riskLevel * 0.1;

    /*
     * 终局事件必须很少见，否则会抢走“自己刷完 250 万”的主体验。数据库只给基础概率，
     * 前端只按场景风险做轻微放大。普通状态的 eventMultiplier 只影响混沌事件，不影响
     * 终局事件；否则“好运”“倒霉”这种普通状态也会一起提高提前停表概率，和隐藏终局
     * 必须偶发的规则冲突。
     */
    return Phaser.Math.Clamp(ending.probability * riskMultiplier, 0, 0.035);
  }

  private triggerTerminalEvent(ending: TerminalEvent, now = performance.now()): void {
    if (ending.balanceEffect === "zero" && this.state.balance > 0) {
      const spent = this.state.balance;
      this.state.totalSpent += spent;
      this.state.maxSingleSpend = Math.max(this.state.maxSingleSpend, spent);
      this.state.balance = 0;
      this.settlementStats.eventCount += 1;
      this.recordSettlementHighlight({
        title: ending.title,
        detail: ending.description,
        amount: spent,
        kind: "spend",
        source: "terminal"
      });
      this.showCheckoutPulse(spent);
      this.callbacks.onTone("danger");
    } else {
      this.callbacks.onTone("danger");
    }

    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title: ending.title,
      detail: ending.description,
      kind: "system"
    });
    this.endRound("terminal_event", ending, now);
  }

  private applyChaosEvent(event: GameEvent, now: number): void {
    const delta = event.delta ?? 0;
    let actualDelta = delta;
    this.eventCooldownUntil.set(event.id, now + event.cooldownSec * 1000);

    if (delta === 0) {
      this.pushSystemFeed(event.title, event.description);
      return;
    }

    if (delta > 0) {
      this.state.balance += delta;
      this.state.totalIncome += delta;
      this.callbacks.onTone("income");
    } else {
      const requestedSpend = Math.abs(delta);
      const spent = Math.min(requestedSpend, this.state.balance);
      if (spent <= 0) {
        return;
      }
      actualDelta = -spent;
      this.state.balance -= spent;
      this.state.totalSpent += spent;
      this.state.maxSingleSpend = Math.max(this.state.maxSingleSpend, spent);
      this.callbacks.onTone(spent >= this.tuning.highPriceThreshold ? "danger" : "spend");
    }

    this.settlementStats.eventCount += 1;
    this.recordSettlementHighlight({
      title: event.title,
      detail: event.description,
      amount: Math.abs(actualDelta),
      kind: actualDelta > 0 ? "income" : "spend",
      source: "event"
    });
    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title: event.title,
      detail: `${event.description} ${actualDelta > 0 ? "+" : "-"}${formatMoney(Math.abs(actualDelta))}`,
      kind: actualDelta > 0 ? "income" : "spend"
    });
    this.showCheckoutPulse(-actualDelta);
    this.emitState();

    if (this.state.balance <= 0) {
      this.endRound("balance_zero", undefined, now);
    }
  }

  private currentScene(stage = this.activeStage): Scene {
    const selectedScene = this.selectSceneForStage(stage);
    if (stage === this.activeStage && this.activeScene?.id === selectedScene.id) {
      return this.activeScene;
    }

    return selectedScene;
  }

  private pushSceneChangeFeed(stageChanged: boolean, stage: number, scene: Scene, now: number): void {
    if (stageChanged) {
      this.pushSystemFeed("场景切换", `第 ${stage}/${this.state.totalStages} 段进入「${scene.name}」`);
      return;
    }

    const pressureReductionReason = this.scenePressureReductionReason(now);
    if (pressureReductionReason === "low-mood") {
      this.pushSystemFeed("场景降压", `低落状态让限时高压场景退场，进入「${scene.name}」`);
      return;
    }
    if (pressureReductionReason === "target-clear") {
      this.pushSystemFeed("特殊场景结束", `${formatDuration(this.targetClearMs())} 主线目标时间已过，限时高压场景退场，进入「${scene.name}」等待硬结算`);
      return;
    }

    this.pushSystemFeed("场景恢复", `状态影响结束，重新进入「${scene.name}」`);
  }

  private selectSceneForStage(stage: number): Scene {
    const commonScenes = this.bootstrap.scenes.filter((scene) => scene.rarity === "common" && scene.minBalance <= this.state.balance);
    const commonIndex = (stage - 1 + this.sceneRotationOffset) % Math.max(1, commonScenes.length);
    const common = commonScenes[commonIndex] ?? this.bootstrap.scenes[0];
    const specialScenes = this.bootstrap.scenes.filter((scene) => scene.rarity !== "common" && scene.minBalance <= this.state.balance);
    const pressureReductionReason = this.scenePressureReductionReason();

    /*
     * 特殊场景是阶段压力，不是永远覆盖当前货架的模式。这里有两种情况会把 rare/wild
     * 降回 common：第一，主线目标时间已经走完，第 12 阶段只保留阶段编号，不再让高压场景
     * 从 6:25 一直常驻到 11:00；第二，玩家进入“低落”状态。开发计划里低落明确写了
     * “商品刷新变慢，场景减少”，所以低落期间减少的是高压场景插入，而不是把普通场景也
     * 删掉。这样现有 tags 就能表达状态语义，不需要为了一个状态立刻扩展 API 字段。
     */
    if (specialScenes.length === 0 || stage % 4 !== 0 || pressureReductionReason !== null) {
      return common;
    }

    /*
     * 一局只有第 4、8、12 阶段会插入特殊场景，但数据库现在有十多个 rare/wild 场景。
     * 如果总是从 specialScenes[0] 开始取，后面的旅行、职场、高端散场等内容即使在
     * 数据库里存在，也几乎不会进入真实玩法。这里用本局 chaosSeed 生成偏移，再按特殊
     * 场景槽位跨步轮换；同一局仍然稳定可复现，不同局会看到不同的高压场景组合。
     */
    const specialSlot = Math.max(0, Math.floor(stage / 4) - 1);
    const specialStep = Math.max(1, Math.floor(specialScenes.length / Math.max(1, Math.floor(this.state.totalStages / 4))));
    const specialIndex = (this.sceneRotationOffset + specialSlot * specialStep) % specialScenes.length;
    return specialScenes[specialIndex] ?? common;
  }

  private hashSceneSeed(seed: string): number {
    let hash = 0;
    for (let index = 0; index < seed.length; index += 1) {
      hash = (hash * 31 + seed.charCodeAt(index)) >>> 0;
    }

    return hash;
  }

  private redrawArena(): void {
    const width = this.scale.width;
    const height = this.scale.height;
    const outerInset = 16;
    const innerInset = 22;
    const cornerLength = Phaser.Math.Clamp(width * 0.055, 34, 72);

    this.arenaGraphics.clear();
    this.arenaGraphics.fillGradientStyle(0x160d0b, 0x070707, 0x071316, 0x050505, 1);
    this.arenaGraphics.fillRect(0, 0, width, height);
    this.arenaGraphics.fillStyle(0x8b1724, 0.035);
    this.arenaGraphics.fillRect(0, 0, width * 0.34, height);
    this.arenaGraphics.fillStyle(0x0b7e78, 0.03);
    this.arenaGraphics.fillRect(width * 0.66, 0, width * 0.34, height);

    this.arenaGraphics.lineStyle(7, 0x3b2108, 0.9);
    this.arenaGraphics.strokeRoundedRect(outerInset, outerInset, width - outerInset * 2, height - outerInset * 2, 8);
    this.arenaGraphics.lineStyle(2, 0xe7b64e, 0.68);
    this.arenaGraphics.strokeRoundedRect(outerInset + 2, outerInset + 2, width - outerInset * 2 - 4, height - outerInset * 2 - 4, 7);
    this.arenaGraphics.lineStyle(1, 0xffedb0, 0.28);
    this.arenaGraphics.strokeRoundedRect(innerInset, innerInset, width - innerInset * 2, height - innerInset * 2, 5);

    this.arenaGraphics.lineStyle(2, 0xffe19a, 0.72);
    this.arenaGraphics.lineBetween(innerInset, innerInset, innerInset + cornerLength, innerInset);
    this.arenaGraphics.lineBetween(innerInset, innerInset, innerInset, innerInset + cornerLength * 0.58);
    this.arenaGraphics.lineBetween(width - innerInset - cornerLength, innerInset, width - innerInset, innerInset);
    this.arenaGraphics.lineBetween(width - innerInset, innerInset, width - innerInset, innerInset + cornerLength * 0.58);
    this.arenaGraphics.lineBetween(innerInset, height - innerInset, innerInset + cornerLength, height - innerInset);
    this.arenaGraphics.lineBetween(innerInset, height - innerInset - cornerLength * 0.58, innerInset, height - innerInset);
    this.arenaGraphics.lineBetween(width - innerInset - cornerLength, height - innerInset, width - innerInset, height - innerInset);
    this.arenaGraphics.lineBetween(width - innerInset, height - innerInset - cornerLength * 0.58, width - innerInset, height - innerInset);

    for (let shelf = 1; shelf <= 3; shelf += 1) {
      const y = height * (0.1 + shelf * 0.18);
      this.arenaGraphics.lineStyle(5, 0x231406, 0.52);
      this.arenaGraphics.lineBetween(28, y + 2, width - 28, y + 2);
      this.arenaGraphics.lineStyle(1, shelf === 2 ? 0x6fffe8 : 0xd8a845, 0.22);
      this.arenaGraphics.lineBetween(30, y, width - 30, y);
    }
    this.arenaGraphics.fillStyle(0x160f08, 0.86);
    this.arenaGraphics.fillRoundedRect(width * 0.36, height * 0.77, width * 0.28, height * 0.13, 10);
    this.arenaGraphics.lineStyle(5, 0x422609, 0.82);
    this.arenaGraphics.strokeRoundedRect(width * 0.36, height * 0.77, width * 0.28, height * 0.13, 10);
    this.arenaGraphics.lineStyle(2, 0xe8b64c, 0.48);
    this.arenaGraphics.strokeRoundedRect(width * 0.36 + 4, height * 0.77 + 4, width * 0.28 - 8, height * 0.13 - 8, 7);
    this.arenaGraphics.fillStyle(0x57fff3, 0.22);
    this.arenaGraphics.fillRect(width * 0.39, height * 0.83, width * 0.22, 5);
    this.arenaGraphics.fillStyle(0xffffff, 0.28);
    this.arenaGraphics.fillRect(width * 0.39, height * 0.83, width * 0.07, 1);
  }

  private renderHandTimer(now: number): void {
    if (this.state.status !== "running") {
      this.handTimerGraphics?.clear();
      if (this.handTimerText) {
        this.handTimerText.setText("");
      }
      return;
    }

    const frozen = now < this.handFrozenUntil;
    const checkoutLocked = now < this.checkoutLockedUntil;
    const remaining = frozen
      ? Math.max(0, this.handFrozenUntil - now)
      : checkoutLocked
        ? Math.max(0, this.checkoutLockedUntil - now)
        : Math.max(0, this.nextHandRefreshAt - now);
    const total = frozen ? this.tuning.clearCartDelayMs : checkoutLocked ? this.tuning.selectionSettleMs : this.currentHandRefreshMs(now);
    const ratio = Phaser.Math.Clamp(remaining / Math.max(1, total), 0, 1);
    const width = this.scale.width;
    const barX = 26;
    const barY = 22;
    const barWidth = Math.min(360, width - 52);
    const barHeight = 8;

    this.handTimerGraphics.clear();
    this.handTimerGraphics.fillStyle(0x000000, 0.55);
    this.handTimerGraphics.fillRoundedRect(barX, barY, barWidth, barHeight, 4);
    this.handTimerGraphics.fillStyle(frozen || checkoutLocked ? 0x57fff3 : 0xffd15e, 0.95);
    this.handTimerGraphics.fillRoundedRect(barX, barY, barWidth * ratio, barHeight, 4);
    this.handTimerText.setPosition(barX, barY + 12);
    this.handTimerText.setColor(frozen || checkoutLocked ? "#57fff3" : "#ffd15e");
    this.handTimerText.setText(
      frozen
        ? `购物车结算锁定 ${Math.ceil(remaining / 1000)}s`
        : checkoutLocked
          ? `收银结算 ${Math.ceil(remaining / 1000)}s`
          : `货架刷新 ${Math.ceil(remaining / 1000)}s`
    );
  }

  private flashCheckout(): void {
    const flash = this.add
      .rectangle(this.scale.width / 2, this.scale.height * 0.44, this.scale.width * 0.9, this.scale.height * 0.62, 0xffd15e, 0.05)
      .setDepth(20);

    this.tweens.add({
      targets: flash,
      alpha: 0,
      duration: CARD_REFRESH_FLASH_MS,
      onComplete: () => flash.destroy()
    });
  }

  private endRound(endedBy: RunSubmission["endedBy"], ending?: TerminalEvent, endedAt = performance.now()): void {
    if (this.state.status === "ended") {
      return;
    }

    const timeoutSavedRatio = this.state.balance / Math.max(1, this.bootstrap.config.initialBalance);
    const finalFeedTitle =
      ending !== undefined
        ? "终局事件提前停表"
        : endedBy === "balance_zero"
          ? "清空成功"
          : timeoutSavedRatio >= 0.35
            ? "隐藏结局"
            : "硬结算到时";
    const timeoutDetail =
      timeoutSavedRatio >= 0.35
        ? `${formatDuration(this.bootstrap.config.roundLimitMs)} 硬结算触发，剩余 ${formatMoney(this.state.balance)}，进入省钱路线战报`
        : `${formatDuration(this.bootstrap.config.roundLimitMs)} 硬结算触发，剩余 ${formatMoney(this.state.balance)}，进入时间到战报`;

    this.clearDeferredPaymentState();
    this.state.status = "ended";
    this.state.endedBy = endedBy;
    this.syncClock(endedBy === "timeout" ? Math.min(endedAt, this.roundLimitAt()) : endedAt);
    this.updateActionTimers(endedAt);
    this.emitState();
    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title: finalFeedTitle,
      detail:
        ending
          ? `${ending.title}：${ending.description}`
          : endedBy === "balance_zero"
            ? `${this.state.username} 把额度刷到了 0`
            : timeoutDetail,
      kind: "system"
    });
    this.callbacks.onRoundEnd({
      username: this.state.username,
      durationMs: Math.round(this.state.elapsedMs),
      maxSingleSpend: Math.round(this.state.maxSingleSpend),
      finalBalance: Math.round(this.state.balance),
      totalSpent: Math.round(this.state.totalSpent),
      totalIncome: Math.round(this.state.totalIncome),
      endedBy,
      chaosSeed: this.chaosSeed,
      contentVersion: this.bootstrap.config.contentVersion,
      endingId: ending?.id,
      endingTitle: ending?.title,
      endingDetail: ending?.description,
      settlementStats: cloneSettlementStats(this.settlementStats)
    });
  }

  private clearCards(): void {
    this.cards.forEach((card) => card.container?.destroy());
    this.cards = [];
    this.paymentGraphics?.clear();
  }

  private pushSystemFeed(title: string, detail: string): void {
    this.callbacks.onFeedEvent({
      id: crypto.randomUUID(),
      title,
      detail,
      kind: "system"
    });
  }

  private emitState(): void {
    this.callbacks.onStateChange({ ...this.state });
  }
}
