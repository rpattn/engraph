-- name: ListEntityPropertyDefinitions :many
SELECT id, org_id, entity_type, property_name, property_type, ref_target_type, ui_label, is_filterable, created_at
FROM entity_property_definitions
WHERE org_id = $1 AND entity_type = $2
ORDER BY property_name;

-- name: GetEntityPropertyDefinition :one
SELECT id, org_id, entity_type, property_name, property_type, ref_target_type, ui_label, is_filterable, created_at
FROM entity_property_definitions
WHERE org_id = $1 AND entity_type = $2 AND property_name = $3;
