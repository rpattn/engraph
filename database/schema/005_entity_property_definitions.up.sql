CREATE TABLE IF NOT EXISTS entity_property_definitions (
    id SERIAL PRIMARY KEY,
    org_id UUID NOT NULL,
    entity_type TEXT NOT NULL,
    property_name TEXT NOT NULL,
    property_type TEXT NOT NULL,
    ref_target_type TEXT,
    ui_label TEXT,
    is_filterable BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (org_id, entity_type, property_name)
);
CREATE INDEX IF NOT EXISTS entity_property_definitions_org_idx
    ON entity_property_definitions (org_id, entity_type);
