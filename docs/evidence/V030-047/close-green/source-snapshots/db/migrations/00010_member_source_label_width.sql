-- +goose Up
-- Existing accepted FR002 account limit is254 Unicode characters. Source
-- labels preserve the entire authoritative account and its tombstone.
ALTER TABLE applications.member_sources ALTER COLUMN label TYPE varchar(254);
-- Supports isolated databases that applied the earlier unreleased hot8 draft.
-- Hot8 also preserves254 at initial registry seed; released migrations unchanged.
-- No destructive Down.
