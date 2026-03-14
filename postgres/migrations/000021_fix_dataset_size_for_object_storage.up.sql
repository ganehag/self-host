BEGIN;

CREATE OR REPLACE FUNCTION update_dataset_content_change()
RETURNS TRIGGER AS $$
BEGIN
   IF NEW.storage_backend = 'inline' THEN
      NEW.size = length(NEW.content);
   END IF;
   NEW.updated = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

COMMIT;
