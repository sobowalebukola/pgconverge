CREATE EXTENSION IF NOT EXISTS "pgcrypto";CREATE TABLE IF NOT EXISTS "users" ("id" uuid DEFAULT gen_random_uuid(),
  "email" varchar(150),
  "updated_at" TIMESTAMP DEFAULT now(),
  "origin_node" VARCHAR(50),
  PRIMARY KEY ("id"),
  UNIQUE ("email"));CREATE INDEX IF NOT EXISTS "users_email_idx" ON "users" ("email");
				-- 1. Set replica identity
				ALTER TABLE users REPLICA IDENTITY FULL;

				-- 2. Auto-update timestamp on changes
				CREATE OR REPLACE FUNCTION users_set_updated_at() RETURNS TRIGGER AS $$
				BEGIN
					NEW.updated_at = NOW();
					RETURN NEW;
				END;
				$$ LANGUAGE plpgsql;

				CREATE TRIGGER set_updated_at_users
				BEFORE INSERT OR UPDATE ON users
				FOR EACH ROW EXECUTE FUNCTION users_set_updated_at();

				-- 3. Resolve conflicts based on timestamp
				CREATE OR REPLACE FUNCTION users_resolve_conflict() RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'UPDATE' THEN
						-- Only apply if newer (with small tolerance for clock skew)
						IF NEW.updated_at > OLD.updated_at + interval '1 second' THEN
							RETURN NEW;
						ELSE
							RETURN NULL;  -- Skip, keep existing
						END IF;
					END IF;
					RETURN NEW;
				END;
				$$ LANGUAGE plpgsql;

				CREATE OR REPLACE TRIGGER conflict_resolution_users 
				BEFORE UPDATE ON users
				FOR EACH ROW EXECUTE FUNCTION users_resolve_conflict();
