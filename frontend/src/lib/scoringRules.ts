/**
 * Scoring Rules Module
 *
 * Pure, deterministic functions for converting sprint data into team and individual
 * quarterly scores. No side effects, no external dependencies.
 */

// ============================================================================
// CONSTANTS
// ============================================================================

/**
 * Fixed quarterly weights for 6 sprints that sum to 100%
 * Sprint 1→6: 10%, 10%, 15%, 15%, 20%, 30%
 * Later sprints have higher weight (recency curve)
 */
export const QUARTERLY_WEIGHTS = [10, 10, 15, 15, 20, 30] as const;

// ============================================================================
// TYPES
// ============================================================================

export interface SprintGoal {
  weight: number;  // 1-5 importance rating
  done: boolean;   // completion status
}

export interface BonusTier {
  tier: 'A' | 'B' | 'C';
  min_score: number;
  max_score: number;
  salary_multiplier: number;
}

export type BucketType = 'A' | 'B' | 'C';

// ============================================================================
// TEAM SCORING FUNCTIONS
// ============================================================================

/**
 * Team Sprint Score (TSS)
 *
 * Calculates team performance for a single sprint based on weighted goal completion.
 *
 * @param goals - Array of sprint goals with weights (1-5) and done flags
 * @returns Integer 0-100 representing team sprint performance
 *
 * Formula: TSS = round((sum of weights for done goals) ÷ (sum of all weights) × 100)
 * Edge case: If total weight is 0, returns 0
 *
 * Example: goals with weights [3 (done), 2 (not done)] → TSS = round((3/5) × 100) = 60
 */
export function calculateTeamSprintScore(goals: SprintGoal[]): number {
  if (goals.length === 0) return 0;

  const totalWeight = goals.reduce((sum, goal) => sum + goal.weight, 0);
  if (totalWeight === 0) return 0;

  const completedWeight = goals
    .filter(goal => goal.done)
    .reduce((sum, goal) => sum + goal.weight, 0);

  return Math.round((completedWeight / totalWeight) * 100);
}

/**
 * Team Quarterly Index (TQI)
 *
 * Calculates weighted average team performance across 6 sprints.
 *
 * @param tssValues - Array of up to 6 TSS values (one per sprint)
 * @returns Integer 0-100 representing quarterly team performance
 *
 * Uses quarterly weights with recency curve. Missing sprints treated as 0.
 * Result is clamped to [0,100] range.
 *
 * Example: TSS = [60, 70, 80, 0, 90, 100] →
 * TQI = 0.10×60 + 0.10×70 + 0.15×80 + 0.15×0 + 0.20×90 + 0.30×100 = 73
 */
export function calculateTeamQuarterlyIndex(tssValues: number[]): number {
  // Pad with zeros if fewer than 6 sprints (teams expected to plan every sprint)
  const paddedTSS = [...tssValues];
  while (paddedTSS.length < 6) {
    paddedTSS.push(0);
  }

  // Take only first 6 values if more than 6 provided
  const sixTSS = paddedTSS.slice(0, 6);

  // Calculate weighted average
  const weightedSum = sixTSS.reduce((sum, tss, index) => {
    return sum + (tss * QUARTERLY_WEIGHTS[index] / 100);
  }, 0);

  // Clamp to [0, 100] and round
  return Math.max(0, Math.min(100, Math.round(weightedSum)));
}

// ============================================================================
// INDIVIDUAL SCORING FUNCTIONS
// ============================================================================

/**
 * Individual Sprint Score (ISS) with variable checks
 *
 * Calculates individual performance for a single sprint based on yes/no checks.
 *
 * @param yesCount - Number of "yes" responses (≥ 0)
 * @param totalCount - Total number of checks (≥ 0)
 * @returns Integer 0-100 if totalCount > 0, or null if no checks (missing sprint)
 *
 * Formula: ISS = round((yesCount ÷ totalCount) × 100)
 * Edge case: If totalCount = 0, returns null (missing - not counted in quarterly)
 * Input sanitization: Clamps yesCount to [0, totalCount], negatives to 0
 *
 * Examples:
 * - yes=4, total=5 → 80
 * - yes=0, total=5 → 0
 * - total=0 → null (missing)
 */
export function calculateIndividualSprintScore(yesCount: number, totalCount: number): number | null {
  // Input validation
  if (typeof yesCount !== 'number' || typeof totalCount !== 'number') {
    console.warn('calculateIndividualSprintScore: yesCount and totalCount must be numbers');
    return null;
  }

  if (isNaN(yesCount) || isNaN(totalCount)) {
    console.warn('calculateIndividualSprintScore: yesCount and totalCount cannot be NaN');
    return null;
  }

  // Sanitize inputs
  const sanitizedTotal = Math.max(0, Math.floor(totalCount));
  const sanitizedYes = Math.max(0, Math.min(Math.floor(yesCount), sanitizedTotal));

  // Missing sprint if no checks
  if (sanitizedTotal === 0) return null;

  return Math.round((sanitizedYes / sanitizedTotal) * 100);
}

/**
 * Individual Quarterly Index (IQI) with renormalization
 *
 * Calculates weighted average individual performance across sprints, handling missing data.
 *
 * @param issValues - Array of up to 6 ISS values (null = missing sprint)
 * @returns Integer 0-100 representing quarterly individual performance
 *
 * Only includes sprints with ISS data, renormalizing weights to sum to 100%.
 * If all sprints missing, returns 0. Result clamped to [0,100].
 *
 * Example: ISS = [80, null, 100, null, 60, 90] where null = missing
 * Available sprints: 1,3,5,6 with weights [10, 15, 20, 30] (sum=75)
 * Renormalized weights: [13.3, 20, 26.7, 40] (sum=100)
 * IQI = 0.133×80 + 0.20×100 + 0.267×60 + 0.40×90 = 82.6 → 83
 */
export function calculateIndividualQuarterlyIndex(issValues: (number | null)[]): number {
  // Input validation
  if (!Array.isArray(issValues)) {
    console.warn('calculateIndividualQuarterlyIndex: issValues must be an array');
    return 0;
  }

  // Filter out missing sprints and get corresponding weights
  const validEntries: Array<{ iss: number; weight: number }> = [];

  for (let i = 0; i < Math.min(issValues.length, 6); i++) {
    const iss = issValues[i];
    if (iss !== null && iss !== undefined && typeof iss === 'number' && !isNaN(iss)) {
      // Clamp ISS values to valid range [0, 100]
      const clampedIss = Math.max(0, Math.min(100, iss));
      validEntries.push({ iss: clampedIss, weight: QUARTERLY_WEIGHTS[i] });
    }
  }

  // If all sprints missing, return 0
  if (validEntries.length === 0) return 0;

  // Calculate sum of weights for renormalization
  const totalWeight = validEntries.reduce((sum, entry) => sum + entry.weight, 0);

  // Calculate weighted average with renormalized weights
  const weightedSum = validEntries.reduce((sum, entry) => {
    const normalizedWeight = entry.weight / totalWeight; // Renormalize to sum to 1
    return sum + (entry.iss * normalizedWeight);
  }, 0);

  // Clamp to [0, 100] and round
  return Math.max(0, Math.min(100, Math.round(weightedSum)));
}

// ============================================================================
// BUCKET ASSIGNMENT
// ============================================================================

/**
 * Performance Bucket Assignment
 *
 * Assigns performance bucket based on final score from team and individual performance.
 *
 * @param tqi - Team Quarterly Index (0-100)
 * @param iqi - Individual Quarterly Index (0-100)
 * @param bonusTiers - Configurable bonus tier settings with score ranges
 * @returns Bucket: "A" (Tier A), "B" (Tier B), or "C" (Tier C)
 *
 * Logic:
 * - Final Score = (TQI × 0.3) + (IQI × 0.7)  [30% team, 70% individual by default]
 * - Tier assigned based on which bonus tier range includes the final score
 *
 * Examples (assuming standard 80-100%/60-79%/0-59% ranges with default 30% team weight):
 * - TQI=80, IQI=90 → Final Score=87 → Tier A
 * - TQI=65, IQI=70 → Final Score=69 → Tier B
 * - TQI=50, IQI=60 → Final Score=57 → Tier C
 */
export function assignPerformanceBucket(tqi: number, iqi: number, bonusTiers: BonusTier[], teamWeight: number = 30): BucketType {
  // Calculate final score with configurable weighting
  const finalScore = calculateCompositeScore(tqi, iqi, teamWeight);

  // Find matching tier based on final score
  const matchingTier = bonusTiers.find(tier =>
    finalScore >= tier.min_score && finalScore <= tier.max_score
  );

  // Return matching tier or default to C if no match found
  return matchingTier?.tier || 'C';
}

/**
 * Calculate final performance score from team and individual scores
 *
 * @param tqi - Team Quarterly Index (0-100)
 * @param iqi - Individual Quarterly Index (0-100)
 * @param teamWeight - Percentage weight for team performance (0-100), default 30
 * @returns Final score (0-100) with configurable weighting
 */
export function calculateCompositeScore(tqi: number, iqi: number, teamWeight: number = 30): number {
  const teamWeightDecimal = teamWeight / 100;
  const individualWeightDecimal = (100 - teamWeight) / 100;

  return Math.round((tqi * teamWeightDecimal) + (iqi * individualWeightDecimal));
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

/**
 * Currency Formatter
 *
 * Formats number as USD with 0 decimals and thousands separators.
 *
 * @param amount - Numeric amount to format
 * @returns Formatted string like "$50,000,000"
 *
 * Uses Intl.NumberFormat with fallback to manual formatting if unavailable.
 */
export function formatCurrency(amount: number): string {
  try {
    // Standard formatter
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  } catch {
    // Fallback: manual thousands separators
    const rounded = Math.round(amount);
    const withCommas = rounded.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    return `$${withCommas}`;
  }
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

/**
 * Validates sprint goal data
 */
export function isValidSprintGoal(goal: any): goal is SprintGoal {
  return (
    typeof goal === 'object' &&
    goal !== null &&
    typeof goal.weight === 'number' &&
    goal.weight >= 1 &&
    goal.weight <= 5 &&
    typeof goal.done === 'boolean'
  );
}

/**
 * Validates individual check data
 */
export function isValidCheckData(yesCount: any, totalCount: any): boolean {
  return (
    typeof yesCount === 'number' &&
    typeof totalCount === 'number' &&
    yesCount >= 0 &&
    totalCount >= 0 &&
    yesCount <= totalCount
  );
}