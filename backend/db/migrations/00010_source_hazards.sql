-- +goose Up
-- Whether a source's events are hazards (ADR 27). A country's coverage
-- lists hazard sources only: a conflicts feed covering a country says
-- nothing about how well its hazards are watched.
ALTER TABLE sources ADD COLUMN hazards boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE sources DROP COLUMN hazards;
