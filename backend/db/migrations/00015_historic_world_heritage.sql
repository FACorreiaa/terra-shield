-- +goose Up
-- L4 step 2 (ADR 30): general landmarks join the historic layer, and every
-- site says whether it is a UNESCO World Heritage Site, as a filter. A view
-- holding more sites than it can show keeps the most notable: those with
-- the most Wikipedia and other wiki pages (sitelinks).
ALTER TABLE historic_sites ADD COLUMN world_heritage boolean NOT NULL DEFAULT false;
ALTER TABLE historic_sites ADD COLUMN sitelinks integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE historic_sites DROP COLUMN sitelinks;
ALTER TABLE historic_sites DROP COLUMN world_heritage;
