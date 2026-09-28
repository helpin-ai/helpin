-- Percentage measures use the same baseline-to-target achievement as numbers.
-- Recompute stored progress without changing values or their update attribution.
UPDATE pm_key_results
SET progress = CASE
    WHEN target_value = initial_value THEN 0
    WHEN (current_value - initial_value) / (target_value - initial_value) <= 0 THEN 0
    WHEN (current_value - initial_value) / (target_value - initial_value) >= 1 THEN 100
    ELSE 100.0 * (current_value - initial_value) / (target_value - initial_value)
END
WHERE result_type = 'percent';
