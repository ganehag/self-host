BEGIN;

ALTER TABLE group_policies
ADD CONSTRAINT group_policies_priority_nonnegative_chk
CHECK (priority >= 0),
ADD CONSTRAINT group_policies_resource_shape_chk
CHECK (
	resource <> ''
	AND btrim(resource) = resource
	AND position('/' in resource) <> 1
	AND right(resource, 1) <> '/'
	AND position('//' in resource) = 0
	AND resource !~ '[[:space:]]'
);

COMMIT;
