export type MoneyDelta = number;

export type PriceTier =
  | "coin"
  | "small"
  | "daily"
  | "premium"
  | "large"
  | "heavy"
  | "shock"
  | "income";

export type GameMode = "chaos-life";

export type MultiplierRule = {
  id: string;
  label: string;
  multiplier: number;
  minBalance: number;
  maxUnitPrice: number;
  maxTotalPrice: number;
  weight: number;
};

export type InterestBand = {
  minBalance: number;
  rate: number;
};

export type BalanceTuning = {
  stageCount: number;
  stageDurationMs: number;
  targetClearMs: number;
  handRefreshMs: number;
  selectionSettleMs: number;
  interestStartDelayMs: number;
  interestIntervalMs: number;
  interestRate: number;
  interestBands: InterestBand[];
  visaDelayMs: number;
  visaCooldownMs: number;
  clearCartDelayMs: number;
  clearCartCooldownMs: number;
  clearCartPickCount: number;
  normalHighCardHandChance: number;
  specialHighCardCount: number;
  highPriceThreshold: number;
  eventBaseChance: number;
  eventRiskBonus: number;
  eventMatchBonus: number;
  multiplierRules: MultiplierRule[];
};

export type Item = {
  id: string;
  name: string;
  category: string;
  sceneId: string | null;
  price: number;
  tier: PriceTier;
  maxBuy: number | null;
  batchable: boolean;
  weight: number;
  minBalance: number;
  modes: GameMode[];
  tags: string[];
  flavor: string;
};

export type Scene = {
  id: string;
  name: string;
  entryCost: number;
  durationSec: number;
  minBalance: number;
  rarity: "common" | "rare" | "wild";
  riskLevel: 1 | 2 | 3 | 4 | 5;
  itemTags: string[];
  eventTags: string[];
  modes: GameMode[];
};

export type EventOption = {
  label: string;
  delta: MoneyDelta;
  delaySec?: number;
};

export type GameEvent = {
  id: string;
  title: string;
  description: string;
  delta?: MoneyDelta;
  options?: EventOption[];
  probability: number;
  cooldownSec: number;
  tags: string[];
  modes: GameMode[];
  settlementTag: string;
};

export type TerminalEvent = {
  id: string;
  title: string;
  description: string;
  probability: number;
  minElapsedMs: number;
  maxBalance: number | null;
  minRiskLevel: number;
  balanceEffect: "none" | "zero";
  tags: string[];
  modes: GameMode[];
  settlementTag: string;
};

export type StatusEffect = {
  id: string;
  name: string;
  durationSec: number;
  itemRefreshMultiplier: number;
  highPriceMultiplier: number;
  eventMultiplier: number;
  tags: string[];
  description: string;
};

export type AudioTrack = {
  id: string;
  title: string;
  mood: "menu" | "rush" | "danger" | "settlement";
  src: string;
  license: "CC0" | "MIT" | "custom";
  sourceUrl: string;
};

export type GameBootstrap = {
  config: {
    initialBalance: number;
    roundLimitMs: number;
    defaultMode: GameMode;
    balanceTuning?: BalanceTuning;
    contentVersion: string;
  };
  items: Item[];
  scenes: Scene[];
  events: GameEvent[];
  endings: TerminalEvent[];
  statuses: StatusEffect[];
  audioTracks: AudioTrack[];
};

export type LeaderboardEntry = {
  rank: number;
  username: string;
  durationMs: number;
  maxSingleSpend: number;
};

export type UserReservation = {
  username: string;
  reserved: boolean;
  reservationToken?: string;
  message?: string;
};

export type RunSubmission = {
  username: string;
  durationMs: number;
  maxSingleSpend: number;
  finalBalance: number;
  totalSpent: number;
  totalIncome: number;
  endedBy: "balance_zero" | "timeout" | "manual" | "terminal_event";
  chaosSeed: string;
  contentVersion?: string;
  endingId?: string;
  endingTitle?: string;
  endingDetail?: string;
  settlementStats?: RoundSettlementStats;
};

/*
 * RoundSettlementStats 是“一局结束后给玩家看的战报统计”，它和排行榜不是同一件事。
 * 排行榜只需要稳定、可信、方便排序的字段，例如用户名、用时和最大单笔消费；这些字段会
 * 提交给 Go 后端并写入数据库。战报统计则是前端在本局运行过程中从真实扣款、返钱、利息
 * 和事件里顺手记录出来的解释性信息，用来告诉玩家“这局最烦人的返钱是什么”“最荒诞的
 * 扣款是什么”。所以它可以保存在浏览器最近战报里，但不能被当作服务端可信成绩。
 * 这里的 paymentCount 沿用早期字段名，实际表示成功处理过几次结算动作；一次结算可能是
 * 普通刷卡、VISA 延迟扣款、清空购物车，也可能是玩家点到一张入账卡。
 */
export type SettlementHighlight = {
  title: string;
  detail: string;
  amount: number;
  kind: "spend" | "income";
  source: "payment" | "event" | "interest" | "terminal";
};

export type RoundSettlementStats = {
  mostAnnoyingIncome: SettlementHighlight | null;
  mostAbsurdSpend: SettlementHighlight | null;
  paymentCount: number;
  spendCount: number;
  incomeCount: number;
  eventCount: number;
  interestCount: number;
};

export type RunResult = {
  accepted: boolean;
  entry: LeaderboardEntry;
  message?: string;
};

export type ApiErrorResponse = {
  code: string;
  message: string;
};
