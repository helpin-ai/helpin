/**
 * Derived Values Layer
 *
 * Pure, dependency-free layer that takes in-memory state and returns all computed
 * numbers the UI needs. Built on top of the scoring rules module.
 */

import {
  calculateTeamSprintScore,
  calculateTeamQuarterlyIndex,
  calculateIndividualSprintScore,
  calculateIndividualQuarterlyIndex,
  assignPerformanceBucket,
  type SprintGoal as ScoringSprintGoal,
  type BucketType
} from './scoringRules';

// Re-export BucketType for convenience
export type { BucketType } from './scoringRules';

// ============================================================================
// INPUT TYPES
// ============================================================================

export interface Employee {
  id: string;
  teamId: string;
  name: string;
  baseSalary: number;
}

export interface Team {
  id: string;
  name: string;
}

export interface SprintGoal {
  id: string;
  weight: number; // 1-5
  done: boolean;
}

export interface TeamSprint {
  teamId: string;
  sprintNumber: number; // 1-6
  goals: SprintGoal[];
}

export interface EmployeeSprintChecks {
  employeeId: string;
  sprintNumber: number; // 1-6
  yes_count: number; // >= 0
  total_count: number; // >= 0, yes_count <= total_count
}

export interface FinanceInputs {
  // Salary-based pool (new logic)
  budgetFactor?: number; // e.g., 0.60 means 60% of total basic salary
  totalBasicSalary?: number; // snapshot/live sum for the quarter
  // Legacy (ignored)
  bonusPoolPercentage?: number;
  mrrStart?: number;
  mrrEnd?: number;
  maxBonusPool?: number;

  // Legacy fields (kept for backward compatibility, will be removed)
  kPrime?: number;
  capMultiplier?: number;
  useCap?: boolean;
}

export interface DerivedValuesInput {
  teams: Team[];
  employees: Employee[];
  teamSprints: TeamSprint[]; // 6 sprints per team
  employeeChecks: EmployeeSprintChecks[]; // up to 6 entries per employee
  finance: FinanceInputs;
  bonusTiers: BonusTier[]; // Dynamic bonus tier settings
  teamWeight: number; // Percentage weight for team performance (0-100)
  overrides?: Record<string, BucketType>; // employeeId → bucket override
}

export interface BonusTier {
  tier: 'A' | 'B' | 'C';
  min_score: number;
  max_score: number;
  salary_multiplier: number;
  description: string;
  editable: boolean;
}

// ============================================================================
// OUTPUT TYPES
// ============================================================================

export interface EmployeePayoutRow {
  employeeId: string;
  employeeName: string;
  teamId: string;
  baseSalary: number;
  bucket: BucketType;
  nominalRate: number; // 0.015 for HI, 0.005 for S, 0 for G
  nominalAmount: number; // baseSalary * nominalRate
  finalAmount: number; // nominalAmount * scale
  effectiveRate: number; // finalAmount / baseSalary
  isOverride: boolean; // true if bucket came from override
  hiBlockedByTeamGate: boolean; // true if would be HI but team TQI < 60
}

export interface PoolCalculation {
  nnmrr: number; // max(0, mrrEnd - mrrStart) - MRR growth
  mrrPool: number; // pool from MRR percentage
  salaryCap: number; // totalBasicSalary * budgetFactor
  poolBase: number; // alias of mrrPool for backward compatibility
  pool: number; // final available pool = min(mrrPool, salaryCap)
  usedNominal: number; // sum of all target bonuses
  scale: number; // pool / usedNominal - multiplier to fit available money
  finalTotal: number; // sum of all final bonuses paid out
}

export interface GuardrailSummary {
  hiShare: number; // % of headcount with bucket "HI" (0-100)
  totalEmployees: number;
  hiCount: number;
}

export interface DerivedValuesOutput {
  teamTQI: Record<string, number>; // teamId → TQI (0-100)
  empIQI: Record<string, number>; // employeeId → IQI (0-100)
  buckets: Record<string, BucketType>; // employeeId → final bucket (after overrides)
  pool: PoolCalculation;
  payouts: EmployeePayoutRow[];
  guardrails: GuardrailSummary;
}

// ============================================================================
// CONSTANTS
// ============================================================================

// Dynamic nominal rates based on bonus tiers (direct mapping)
function getNominalRates(bonusTiers: BonusTier[]) {
  const rates: Record<string, number> = {};

  bonusTiers.forEach(tier => {
    // Convert salary_multiplier to percentage (e.g., 1.5 -> 0.015 = 1.5%)
    rates[tier.tier] = tier.salary_multiplier / 100;
  });

  return rates;
}

// ============================================================================
// MAIN DERIVED VALUES FUNCTION
// ============================================================================

/**
 * Computes all derived values from input state
 *
 * Pure function that transforms raw data into all UI-ready computed values.
 * Handles edge cases like division by zero, missing data, and negative inputs.
 *
 * @param input - All input data (teams, employees, sprints, checks, finance, overrides)
 * @returns Complete set of derived values for UI consumption
 */
export function computeDerivedValues(input: DerivedValuesInput): DerivedValuesOutput {
  // Sanitize inputs
  const sanitizedInput = sanitizeInputs(input);

  // 1. Calculate Team TQI
  const teamTQI = calculateTeamTQIs(sanitizedInput);

  // 2. Calculate Employee IQI
  const empIQI = calculateEmployeeIQIs(sanitizedInput);

  // 3. Determine buckets (auto + overrides)
  const buckets = calculateBuckets(teamTQI, empIQI, sanitizedInput);

  // 4. Calculate pool
  const pool = calculatePool(sanitizedInput.finance);

  // 5. Calculate payouts
  const payouts = calculatePayouts(sanitizedInput, buckets, teamTQI, empIQI, pool);

  // 6. Calculate guardrails
  const guardrails = calculateGuardrails(sanitizedInput.employees, buckets);

  return {
    teamTQI,
    empIQI,
    buckets,
    pool,
    payouts,
    guardrails
  };
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

/**
 * Sanitizes inputs to handle edge cases
 */
function sanitizeInputs(input: DerivedValuesInput): DerivedValuesInput {
  return {
    ...input,
    employees: input.employees.map(emp => ({
      ...emp,
      baseSalary: Math.max(0, emp.baseSalary)
    })),
    employeeChecks: input.employeeChecks.map(check => {
      const sanitizedTotal = Math.max(0, check.total_count);
      const sanitizedYes = Math.min(Math.max(0, check.yes_count), sanitizedTotal);
      return {
        ...check,
        yes_count: sanitizedYes,
        total_count: sanitizedTotal
      };
    }),
    finance: {
      bonusPoolPercentage: Math.max(0, Number(input.finance.bonusPoolPercentage ?? 0)),
      mrrStart: Math.max(0, Number(input.finance.mrrStart ?? 0)),
      mrrEnd: Math.max(0, Number(input.finance.mrrEnd ?? 0)),
      maxBonusPool: Math.max(0, Number(input.finance.maxBonusPool ?? 0)),
      kPrime: Math.max(0, Number(input.finance.kPrime ?? 0)),
      capMultiplier: Math.max(0, Number(input.finance.capMultiplier ?? 0)),
      budgetFactor: Math.max(0, Number(input.finance.budgetFactor ?? 0)),
      totalBasicSalary: Math.max(0, Number(input.finance.totalBasicSalary ?? 0))
    }
  };
}

/**
 * Calculates TQI for all teams
 */
function calculateTeamTQIs(input: DerivedValuesInput): Record<string, number> {
  const teamTQI: Record<string, number> = {};

  for (const team of input.teams) {
    // Get all sprints for this team (should be 6)
    const teamSprints = input.teamSprints
      .filter(sprint => sprint.teamId === team.id)
      .sort((a, b) => a.sprintNumber - b.sprintNumber);

    // Calculate TSS for each sprint
    const tssValues: number[] = [];

    for (let sprintNum = 1; sprintNum <= 6; sprintNum++) {
      const sprint = teamSprints.find(s => s.sprintNumber === sprintNum);
      if (sprint) {
        // Convert to scoring format
        const scoringGoals: ScoringSprintGoal[] = sprint.goals.map(goal => ({
          weight: goal.weight,
          done: goal.done
        }));
        const tss = calculateTeamSprintScore(scoringGoals);
        tssValues.push(tss);
      } else {
        // Missing sprint counts as 0
        tssValues.push(0);
      }
    }

    teamTQI[team.id] = calculateTeamQuarterlyIndex(tssValues);
  }

  return teamTQI;
}

/**
 * Calculates IQI for all employees with renormalization
 */
function calculateEmployeeIQIs(input: DerivedValuesInput): Record<string, number> {
  const empIQI: Record<string, number> = {};

  for (const employee of input.employees) {
    // Get all checks for this employee (up to 6 sprints)
    const employeeChecks = input.employeeChecks
      .filter(check => check.employeeId === employee.id)
      .sort((a, b) => a.sprintNumber - b.sprintNumber);

    // Calculate ISS for each sprint
    const issValues: (number | null)[] = [];

    for (let sprintNum = 1; sprintNum <= 6; sprintNum++) {
      const check = employeeChecks.find(c => c.sprintNumber === sprintNum);
      if (check) {
        const iss = calculateIndividualSprintScore(check.yes_count, check.total_count);
        issValues.push(iss);
      } else {
        // No data for this sprint
        issValues.push(null);
      }
    }

    empIQI[employee.id] = calculateIndividualQuarterlyIndex(issValues);
  }

  return empIQI;
}

/**
 * Determines final buckets (auto calculation + overrides)
 */
function calculateBuckets(
  teamTQI: Record<string, number>,
  empIQI: Record<string, number>,
  input: DerivedValuesInput
): Record<string, BucketType> {
  const buckets: Record<string, BucketType> = {};

  for (const employee of input.employees) {
    const tqi = teamTQI[employee.teamId] || 0;
    const iqi = empIQI[employee.id] || 0;

    // Auto bucket calculation using final score and bonus tiers with configurable weighting
    const autoBucket = assignPerformanceBucket(tqi, iqi, input.bonusTiers, input.teamWeight);

    // Apply override if present
    const override = input.overrides?.[employee.id];
    buckets[employee.id] = override || autoBucket;
  }

  return buckets;
}

/**
 * Calculates quarterly pool from finance inputs
 */
function calculatePool(finance: FinanceInputs): PoolCalculation {
  // 1) MRR-based pool
  const mrrStart = Math.max(0, Number(finance.mrrStart ?? 0));
  const mrrEnd = Math.max(0, Number(finance.mrrEnd ?? 0));
  const nnmrr = Math.max(0, mrrEnd - mrrStart);
  const bonusPct = Math.max(0, Number(finance.bonusPoolPercentage ?? 0)) / 100;
  const mrrPool = Math.max(0, bonusPct * nnmrr);

  // 2) Salary-based cap
  const budgetFactor = Math.max(0, Number(finance.budgetFactor ?? 0));
  const totalBasicSalary = Math.max(0, Number(finance.totalBasicSalary ?? 0));
  const salaryCap = Math.max(0, totalBasicSalary * budgetFactor);

  // 3) Final pool is the minimum of the two (if salaryCap provided)
  const hasSalaryCap = salaryCap > 0;
  const pool = hasSalaryCap ? Math.min(mrrPool, salaryCap) : mrrPool;

  return {
    nnmrr,
    mrrPool,
    salaryCap,
    poolBase: mrrPool,
    pool: Math.max(0, pool),
    usedNominal: 0,
    scale: 0,
    finalTotal: 0
  };
}

/**
 * Calculates salary-weighted payouts for all employees
 */
function calculatePayouts(
  input: DerivedValuesInput,
  buckets: Record<string, BucketType>,
  teamTQI: Record<string, number>,
  empIQI: Record<string, number>,
  poolCalc: PoolCalculation
): EmployeePayoutRow[] {
  const payouts: EmployeePayoutRow[] = [];
  let usedNominal = 0;

  // Get dynamic rates from bonus tiers
  const nominalRates = getNominalRates(input.bonusTiers);

  // Calculate nominal amounts
  for (const employee of input.employees) {
    const bucket = buckets[employee.id];
    const nominalRate = nominalRates[bucket] || 0;
    const nominalAmount = employee.baseSalary * nominalRate;
    usedNominal += nominalAmount;

    // Check if A is blocked by team gate (using final score logic)
    const tqi = teamTQI[employee.teamId] || 0;
    const iqi = empIQI[employee.id] || 0;
    const autoBucket = assignPerformanceBucket(tqi, iqi, input.bonusTiers, input.teamWeight);
    const hiBlockedByTeamGate = autoBucket === 'A' && tqi < 60;

    payouts.push({
      employeeId: employee.id,
      employeeName: employee.name,
      teamId: employee.teamId,
      baseSalary: employee.baseSalary,
      bucket,
      nominalRate,
      nominalAmount,
      finalAmount: 0, // Will be calculated after scale
      effectiveRate: 0, // Will be calculated after scale
      isOverride: !!(input.overrides?.[employee.id]),
      hiBlockedByTeamGate
    });
  }

  // PAY WHAT'S EARNED: No scaling, just pay target amounts up to pool limit
  // If total earned exceeds pool, we cap proportionally
  const scale = usedNominal > poolCalc.pool ? poolCalc.pool / usedNominal : 1.0;

  // Calculate final amounts (either full earned amount or capped)
  let finalTotal = 0;
  for (const payout of payouts) {
    // Pay exactly what was earned (or proportionally less if pool insufficient)
    payout.finalAmount = payout.nominalAmount * scale;
    payout.effectiveRate = payout.baseSalary > 0 ? payout.finalAmount / payout.baseSalary : 0;
    finalTotal += payout.finalAmount;
  }

  // Update pool calculation
  poolCalc.usedNominal = usedNominal;
  poolCalc.scale = scale;
  poolCalc.finalTotal = finalTotal;

  return payouts;
}

/**
 * Calculates guardrail summaries
 */
function calculateGuardrails(
  employees: Employee[],
  buckets: Record<string, BucketType>
): GuardrailSummary {
  const totalEmployees = employees.length;
  const hiCount = employees.filter(emp => buckets[emp.id] === 'A').length;
  const hiShare = totalEmployees > 0 ? (hiCount / totalEmployees) * 100 : 0;

  return {
    hiShare: Math.round(hiShare * 100) / 100, // Round to 2 decimal places
    totalEmployees,
    hiCount
  };
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

/**
 * Currency formatter (re-exported from scoring rules for convenience)
 */
export { formatCurrency } from './scoringRules';

/**
 * Helper to get employee by ID
 */
export function findEmployee(employees: Employee[], employeeId: string): Employee | undefined {
  return employees.find(emp => emp.id === employeeId);
}

/**
 * Helper to get team by ID
 */
export function findTeam(teams: Team[], teamId: string): Team | undefined {
  return teams.find(team => team.id === teamId);
}

/**
 * Helper to group employees by team
 */
export function groupEmployeesByTeam(employees: Employee[]): Record<string, Employee[]> {
  const grouped: Record<string, Employee[]> = {};

  for (const employee of employees) {
    if (!grouped[employee.teamId]) {
      grouped[employee.teamId] = [];
    }
    grouped[employee.teamId].push(employee);
  }

  return grouped;
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

/**
 * Validates input data structure
 */
export function validateInput(input: DerivedValuesInput): string[] {
  const errors: string[] = [];

  // Check teams
  if (!input.teams || input.teams.length === 0) {
    errors.push('At least one team is required');
  }

  // Check employees
  if (!input.employees || input.employees.length === 0) {
    errors.push('At least one employee is required');
  }

  // Check employee team references
  const teamIds = new Set(input.teams.map(t => t.id));
  for (const emp of input.employees) {
    if (!teamIds.has(emp.teamId)) {
      errors.push(`Employee ${emp.id} references non-existent team ${emp.teamId}`);
    }
  }

  // Check sprint data integrity
  const employeeIds = new Set(input.employees.map(e => e.id));
  for (const check of input.employeeChecks) {
    if (!employeeIds.has(check.employeeId)) {
      errors.push(`Check references non-existent employee ${check.employeeId}`);
    }
    if (check.sprintNumber < 1 || check.sprintNumber > 6) {
      errors.push(`Invalid sprint number ${check.sprintNumber} (must be 1-6)`);
    }
  }

  return errors;
}