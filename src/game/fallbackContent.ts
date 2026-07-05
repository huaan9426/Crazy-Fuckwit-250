import { DEFAULT_BALANCE_TUNING, DEFAULT_MODE, INITIAL_BALANCE, ROUND_LIMIT_MS } from "./constants";
import type { AudioTrack, GameBootstrap, GameEvent, GameMode, Item, LeaderboardEntry, Scene, StatusEffect, TerminalEvent } from "./types";

/*
 * 这个文件保存的是浏览器端兜底内容。正常情况下，商品、场景、事件、状态和音轨入口
 * 都应该来自 Go 后端的 `/api/content/bootstrap`。但是前端开发经常会遇到后端还没启动、
 * 本机还没装 Go、或者 API 临时不可用的情况，如果这时页面直接空白，UI 和交互就无法继续
 * 调试。所以这里保留一小份和后端样例数据一致的内容，让游戏在离线状态下仍然能跑起来。
 *
 * 这不是第二套正式内容系统。后续内容变多以后，应该优先扩展后端内容包和数据库种子数据，
 * 这里只保留足够启动和验收核心交互的最小样例。
 */
const modes: GameMode[] = [DEFAULT_MODE];

const items: Item[] = [
  {
    id: "subway-extra-stop",
    name: "地铁坐过站补票",
    category: "找零小额",
    sceneId: null,
    price: 1,
    tier: "coin",
    maxBuy: null,
    batchable: true,
    weight: 18,
    minBalance: 0,
    modes,
    tags: ["coin", "traffic", "daily"],
    flavor: "最后一元也要能花出去，离线兜底不能卡在个位余额。"
  },
  {
    id: "parking-half-hour",
    name: "停车半小时补费",
    category: "找零小额",
    sceneId: null,
    price: 68,
    tier: "small",
    maxBuy: null,
    batchable: true,
    weight: 17,
    minBalance: 0,
    modes,
    tags: ["small", "traffic", "fee"],
    flavor: "不是大钱，但很适合测试十倍和二十倍的小额连刷。"
  },
  {
    id: "daily-breakfast-stack",
    name: "便利店早餐十连",
    category: "日常小额",
    sceneId: null,
    price: 180,
    tier: "daily",
    maxBuy: null,
    batchable: true,
    weight: 16,
    minBalance: 0,
    modes,
    tags: ["daily", "batch"],
    flavor: "看起来便宜，刷起来很密。"
  },
  {
    id: "milk-tea-office",
    name: "奶茶全员请客",
    category: "社交压力",
    sceneId: null,
    price: 486,
    tier: "daily",
    maxBuy: null,
    batchable: true,
    weight: 15,
    minBalance: 0,
    modes,
    tags: ["social", "drink"],
    flavor: "小额但高频，堆起来会很疼。"
  },
  {
    id: "ride-surge",
    name: "跨城打车溢价",
    category: "交通",
    sceneId: null,
    price: 1280,
    tier: "premium",
    maxBuy: null,
    batchable: true,
    weight: 12,
    minBalance: 0,
    modes,
    tags: ["traffic", "surge"],
    flavor: "临时赶路，价格也临时起飞。"
  },
  {
    id: "concert-chain",
    name: "演唱会前排连锁",
    category: "娱乐",
    sceneId: null,
    price: 8_860,
    tier: "large",
    maxBuy: null,
    batchable: true,
    weight: 10,
    minBalance: 8_000,
    modes,
    tags: ["fun", "chain"],
    flavor: "票、酒店、车费一起入场。"
  },
  {
    id: "phone-replace",
    name: "手机碎屏换新",
    category: "数码意外",
    sceneId: null,
    price: 9_999,
    tier: "large",
    maxBuy: null,
    batchable: false,
    weight: 9,
    minBalance: 5_000,
    modes,
    tags: ["repair", "digital"],
    flavor: "维修报价让人直接换新。"
  },
  {
    id: "pet-emergency",
    name: "宠物急诊押金",
    category: "宠物",
    sceneId: null,
    price: 16_800,
    tier: "large",
    maxBuy: null,
    batchable: false,
    weight: 8,
    minBalance: 10_000,
    modes,
    tags: ["pet", "medical"],
    flavor: "不做医学建议，只做账单提醒。"
  },
  {
    id: "renovation-rework",
    name: "装修返工增项",
    category: "大件现实",
    sceneId: null,
    price: 68_000,
    tier: "heavy",
    maxBuy: null,
    batchable: false,
    weight: 7,
    minBalance: 40_000,
    modes,
    tags: ["renovation", "rework"],
    flavor: "敲开墙，也敲开预算。"
  },
  {
    id: "auction-mistap",
    name: "拍卖误举牌",
    category: "高端误操作",
    sceneId: "auction-night",
    price: 188_000,
    tier: "shock",
    maxBuy: 1,
    batchable: false,
    weight: 4,
    minBalance: 120_000,
    modes,
    tags: ["auction", "mistap"],
    flavor: "手举起来，钱落下去。"
  },
  {
    id: "yacht-cleaning",
    name: "游艇清洁赔补",
    category: "富人体验",
    sceneId: "masked-party",
    price: 96_000,
    tier: "heavy",
    maxBuy: 1,
    batchable: false,
    weight: 5,
    minBalance: 80_000,
    modes,
    tags: ["luxury", "service-fee"],
    flavor: "租赁没买贵，服务费贵。"
  },
  {
    id: "castle-deposit",
    name: "古堡酒店定金",
    category: "富人体验",
    sceneId: "masked-party",
    price: 128_000,
    tier: "shock",
    maxBuy: 1,
    batchable: false,
    weight: 4,
    minBalance: 100_000,
    modes,
    tags: ["luxury", "deposit"],
    flavor: "住不起没关系，定金先行。"
  },
  {
    id: "tax-refund",
    name: "退税突然到账",
    category: "反向进账",
    sceneId: null,
    price: 22_000,
    tier: "income",
    maxBuy: null,
    batchable: false,
    weight: 6,
    minBalance: 0,
    modes,
    tags: ["income", "refund"],
    flavor: "不是奖励，是阻碍清空。"
  },
  {
    id: "hotel-compensation",
    name: "酒店超售赔付",
    category: "赔付",
    sceneId: null,
    price: 8_000,
    tier: "income",
    maxBuy: null,
    batchable: false,
    weight: 7,
    minBalance: 0,
    modes,
    tags: ["income", "compensation"],
    flavor: "钱回来了，通关远了。"
  }
];

const scenes: Scene[] = [
  {
    id: "daily-loop",
    name: "工作日循环",
    entryCost: 0,
    durationSec: 35,
    minBalance: 0,
    rarity: "common",
    riskLevel: 2,
    itemTags: ["daily", "traffic", "social"],
    eventTags: ["refund", "surge"],
    modes
  },
  {
    id: "auction-night",
    name: "拍卖预展",
    entryCost: 12_000,
    durationSec: 35,
    minBalance: 120_000,
    rarity: "wild",
    riskLevel: 5,
    itemTags: ["auction", "luxury"],
    eventTags: ["mistap"],
    modes
  },
  {
    id: "masked-party",
    name: "面具舞会",
    entryCost: 18_000,
    durationSec: 35,
    minBalance: 80_000,
    rarity: "rare",
    riskLevel: 4,
    itemTags: ["luxury", "deposit", "service-fee"],
    eventTags: ["compensation", "fee"],
    modes
  }
];

const events: GameEvent[] = [
  {
    id: "refund-sting",
    title: "反向进账",
    description: "退款到账，清空进度被拖回。",
    delta: 8_000,
    probability: 0.12,
    cooldownSec: 8,
    tags: ["income", "refund"],
    modes,
    settlementTag: "最烦人返钱"
  },
  {
    id: "rush-fee",
    title: "加急服务费",
    description: "高压状态下追加服务费。",
    delta: -12_000,
    probability: 0.16,
    cooldownSec: 6,
    tags: ["fee", "rush"],
    modes,
    settlementTag: "最荒诞扣款"
  }
];

const endings: TerminalEvent[] = [
  {
    id: "heart-alarm-blackout",
    title: "心脏报警停表",
    description: "系统提示你先离开收银台处理身体报警。本局提前停表，只记录账单，不提供医学建议。",
    probability: 0.0005,
    minElapsedMs: 240_000,
    maxBalance: null,
    minRiskLevel: 4,
    balanceEffect: "none",
    tags: ["medical", "health", "ending"],
    modes,
    settlementTag: "心脏报警终局"
  },
  {
    id: "bankruptcy-zero",
    title: "破产式清零",
    description: "一连串费用把剩余额度直接归零。主线目标达成，但结算会标记为特殊终局。",
    probability: 0.0007,
    minElapsedMs: 300_000,
    maxBalance: 280_000,
    minRiskLevel: 3,
    balanceEffect: "zero",
    tags: ["fee", "legal", "late-fee", "ending"],
    modes,
    settlementTag: "破产式清零"
  }
];

const statuses: StatusEffect[] = [
  {
    id: "rage-buy",
    name: "生气",
    durationSec: 12,
    itemRefreshMultiplier: 1.15,
    highPriceMultiplier: 1.3,
    eventMultiplier: 1.1,
    tags: ["impulse", "auction", "high-risk", "big-spend"],
    description: "高价消费更容易出现。"
  },
  {
    id: "low-mood",
    name: "低落",
    durationSec: 14,
    itemRefreshMultiplier: 0.8,
    highPriceMultiplier: 0.9,
    eventMultiplier: 1.05,
    tags: ["low-mood", "daily", "income", "refund", "low-balance"],
    description: "刷新变慢，限时高压场景会临时退回普通货架。"
  },
  {
    id: "hype-spree",
    name: "上头",
    durationSec: 10,
    itemRefreshMultiplier: 1.25,
    highPriceMultiplier: 1.35,
    eventMultiplier: 1.2,
    tags: ["impulse", "mistap", "platform", "high-risk", "rush"],
    description: "批量和误操作风险上升，货架节奏变得更急。"
  },
  {
    id: "lucky-backfire",
    name: "好运",
    durationSec: 10,
    itemRefreshMultiplier: 1,
    highPriceMultiplier: 0.8,
    eventMultiplier: 1.4,
    tags: ["income", "refund", "compensation", "income-scene"],
    description: "返钱和赔付更容易出现。"
  }
];

export const audioTracks: AudioTrack[] = [
  {
    id: "rush",
    title: "内置合成收银循环",
    mood: "rush",
    src: "",
    license: "custom",
    sourceUrl: ""
  },
  {
    id: "danger",
    title: "内置合成高压循环",
    mood: "danger",
    src: "",
    license: "custom",
    sourceUrl: ""
  },
  {
    id: "settlement",
    title: "内置合成结算循环",
    mood: "settlement",
    src: "",
    license: "custom",
    sourceUrl: ""
  }
];

export const fallbackBootstrap: GameBootstrap = {
  config: {
    initialBalance: INITIAL_BALANCE,
    roundLimitMs: ROUND_LIMIT_MS,
    defaultMode: DEFAULT_MODE,
    balanceTuning: DEFAULT_BALANCE_TUNING,
    contentVersion: "fallback-local-v1"
  },
  items,
  scenes,
  events,
  endings,
  statuses,
  audioTracks
};

export const fallbackLeaderboard: LeaderboardEntry[] = [
  { rank: 1, username: "冷静不了一点", durationMs: 161_000, maxSingleSpend: 188_000 },
  { rank: 2, username: "今晚就花完", durationMs: 189_000, maxSingleSpend: 162_000 },
  { rank: 3, username: "退款杀我", durationMs: 237_000, maxSingleSpend: 128_000 }
];
