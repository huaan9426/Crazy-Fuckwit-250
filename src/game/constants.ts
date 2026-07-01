export const INITIAL_BALANCE = 2_500_000;

export const DEFAULT_MODE = "chaos-life";

export const BALANCE_PHASES = [
  { id: "rich", min: 700_000 },
  { id: "middle", min: 300_000 },
  { id: "low", min: 50_000 },
  { id: "tight", min: 5_000 },
  { id: "change", min: 1_000 },
  { id: "coin", min: 0 }
] as const;
