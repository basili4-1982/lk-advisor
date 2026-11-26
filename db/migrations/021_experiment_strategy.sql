-- An experiment can be scoped to one recommendation strategy.
-- Empty strategy means "all strategies" for backward compatibility.
ALTER TABLE experiments
    ADD COLUMN IF NOT EXISTS strategy text NOT NULL DEFAULT '';
