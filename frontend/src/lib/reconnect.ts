/** Exponential backoff for WebSocket reconnect loops: 1s, 2s, 4s, 8s, capped at 10s. */
export function backoffMs(attempt: number, capMs = 10000): number {
  return Math.min(1000 * 2 ** attempt, capMs);
}
