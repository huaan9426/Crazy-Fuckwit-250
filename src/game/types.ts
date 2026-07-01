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

export type StatusEffect = {
  id: string;
  name: string;
  durationSec: number;
  itemRefreshMultiplier: number;
  highPriceMultiplier: number;
  eventMultiplier: number;
  description: string;
};
