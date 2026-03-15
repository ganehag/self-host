BEGIN;

CREATE OR REPLACE FUNCTION update_dataset_content_change()
RETURNS TRIGGER AS $$
BEGIN
   NEW.size = length(NEW.content);
   RETURN NEW;
END;
$$ language 'plpgsql';

COMMIT;
