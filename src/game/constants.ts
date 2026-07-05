import type { BalanceTuning } from "./types";

export const INITIAL_BALANCE = 2_500_000;

export const DEFAULT_MODE = "chaos-life";

export const ROUND_LIMIT_MS = 660_000;

export const TARGET_CLEAR_MS = 420_000;

export const GAME_CANVAS_WIDTH = 1280;

export const GAME_CANVAS_HEIGHT = 720;

export const DEFAULT_BALANCE_TUNING: BalanceTuning = {
  stageCount: 12,
  stageDurationMs: TARGET_CLEAR_MS / 12,
  targetClearMs: TARGET_CLEAR_MS,
  handRefreshMs: 6_500,
  selectionSettleMs: 1_700,
  interestStartDelayMs: 60_000,
  interestIntervalMs: 10_000,
  interestRate: 0.03,
  interestBands: [
    { minBalance: (INITIAL_BALANCE * 4) / 5, rate: 0.012 },
    { minBalance: INITIAL_BALANCE / 2, rate: 0.018 },
    { minBalance: INITIAL_BALANCE / 5, rate: 0.028 },
    { minBalance: INITIAL_BALANCE / 20, rate: 0.042 },
    { minBalance: 0, rate: 0.065 }
  ],
  visaDelayMs: Math.round((TARGET_CLEAR_MS / 12) * (2 / 3)),
  visaCooldownMs: TARGET_CLEAR_MS / 12,
  clearCartDelayMs: 7_000,
  clearCartCooldownMs: TARGET_CLEAR_MS / 12,
  clearCartPickCount: 3,
  normalHighCardHandChance: 0.03,
  specialHighCardCount: 2,
  highPriceThreshold: INITIAL_BALANCE / 160,
  eventBaseChance: 0.18,
  eventRiskBonus: 0.025,
  eventMatchBonus: 0.08,
  multiplierRules: [
    {
      id: "x1",
      label: "x1",
      multiplier: 1,
      minBalance: 0,
      maxUnitPrice: INITIAL_BALANCE,
      maxTotalPrice: INITIAL_BALANCE,
      weight: 18
    },
    {
      id: "x3",
      label: "x3",
      multiplier: 3,
      minBalance: INITIAL_BALANCE / 20,
      maxUnitPrice: INITIAL_BALANCE / 1_000,
      maxTotalPrice: INITIAL_BALANCE / 180,
      weight: 5
    },
    {
      id: "x5",
      label: "x5",
      multiplier: 5,
      minBalance: INITIAL_BALANCE / 5,
      maxUnitPrice: INITIAL_BALANCE / 2_200,
      maxTotalPrice: INITIAL_BALANCE / 220,
      weight: 2
    },
    {
      id: "x10",
      label: "x10",
      multiplier: 10,
      minBalance: INITIAL_BALANCE / 2,
      maxUnitPrice: INITIAL_BALANCE / 8_000,
      maxTotalPrice: INITIAL_BALANCE / 600,
      weight: 0.75
    },
    {
      id: "x20",
      label: "x20",
      multiplier: 20,
      minBalance: (INITIAL_BALANCE * 4) / 5,
      maxUnitPrice: INITIAL_BALANCE / 20_000,
      maxTotalPrice: INITIAL_BALANCE / 900,
      weight: 0.25
    }
  ]
};

export const BALANCE_PHASES = [
  { id: "rich", min: 700_000 },
  { id: "middle", min: 300_000 },
  { id: "low", min: 50_000 },
  { id: "tight", min: 5_000 },
  { id: "change", min: 1_000 },
  { id: "coin", min: 0 }
] as const;
