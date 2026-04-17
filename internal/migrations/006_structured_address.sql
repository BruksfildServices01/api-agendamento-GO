-- Migration 006: structured address fields
-- Purely additive — existing accounts keep their current Address value untouched.

ALTER TABLE barbershops
  ADD COLUMN IF NOT EXISTS cep           VARCHAR(9),
  ADD COLUMN IF NOT EXISTS street_name   VARCHAR(255),
  ADD COLUMN IF NOT EXISTS street_number VARCHAR(20),
  ADD COLUMN IF NOT EXISTS complement    VARCHAR(100),
  ADD COLUMN IF NOT EXISTS neighborhood  VARCHAR(100),
  ADD COLUMN IF NOT EXISTS city          VARCHAR(100),
  ADD COLUMN IF NOT EXISTS state         VARCHAR(2);
