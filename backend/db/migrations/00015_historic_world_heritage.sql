-- +goose Up
-- L4 step 2 (ADR 30): general landmarks join the historic layer, and every
-- site says whether it is a UNESCO World Heritage Site, as a filter.
ALTER TABLE historic_sites ADD COLUMN world_heritage boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE historic_sites DROP COLUMN world_heritage;
