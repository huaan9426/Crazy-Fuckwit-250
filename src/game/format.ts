export function formatMoney(value: number): string {
  return `¥${Math.round(value).toLocaleString("zh-CN")}`;
}

export function formatDuration(durationMs: number): string {
  const totalCentiseconds = Math.max(0, Math.floor(durationMs / 10));
  const minutes = Math.floor(totalCentiseconds / 6000);
  const seconds = Math.floor((totalCentiseconds % 6000) / 100);
  const centiseconds = totalCentiseconds % 100;

  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}.${String(
    centiseconds
  ).padStart(2, "0")}`;
}
