INSERT INTO entity_property_definitions (
    org_id,
    entity_type,
    property_name,
    property_type,
    ref_target_type,
    ui_label
)
SELECT o.id,
       'Part' AS entity_type,
       'color' AS property_name,
       'string' AS property_type,
       NULL AS ref_target_type,
       'Color' AS ui_label
FROM organisations o
ON CONFLICT (org_id, entity_type, property_name) DO NOTHING;

INSERT INTO entity_property_definitions (
    org_id,
    entity_type,
    property_name,
    property_type,
    ref_target_type,
    ui_label
)
SELECT o.id,
       'Part' AS entity_type,
       'supplier' AS property_name,
       'ref' AS property_type,
       'Entity' AS ref_target_type,
       'Supplier' AS ui_label
FROM organisations o
ON CONFLICT (org_id, entity_type, property_name) DO NOTHING;
