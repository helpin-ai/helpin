import type { UpgradeRequiredReason } from "@/edition/contracts";
export type {
  UpgradeRequiredKind,
  UpgradeRequiredReason,
} from "@/edition/contracts";
export function getUpgradeRequiredReason(
  _error: unknown,
): UpgradeRequiredReason | null {
  return null;
}
export function isUpgradeRequiredError(_error: unknown): boolean {
  return false;
}
