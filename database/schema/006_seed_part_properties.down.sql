DELETE FROM entity_property_definitions
WHERE entity_type = 'Part'
  AND property_name IN ('color', 'supplier');
