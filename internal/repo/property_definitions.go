package repo

import (
	"context"

	"github.com/google/uuid"

	db "yourapp/internal/db/gen"
	"yourapp/internal/models"
)

func (r *pgRepo) ListEntityPropertyDefinitions(ctx context.Context, orgID uuid.UUID, entityType string) ([]models.PropertyDefinition, error) {
	defs, err := r.q.ListEntityPropertyDefinitions(ctx, db.ListEntityPropertyDefinitionsParams{
		OrgID:      toPgUUID(orgID),
		EntityType: entityType,
	})
	if err != nil {
		return nil, err
	}
	out := make([]models.PropertyDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, convertPropertyDefinition(def))
	}
	return out, nil
}

func (r *pgRepo) GetEntityPropertyDefinition(ctx context.Context, orgID uuid.UUID, entityType, propertyName string) (models.PropertyDefinition, error) {
	def, err := r.q.GetEntityPropertyDefinition(ctx, db.GetEntityPropertyDefinitionParams{
		OrgID:        toPgUUID(orgID),
		EntityType:   entityType,
		PropertyName: propertyName,
	})
	if err != nil {
		return models.PropertyDefinition{}, err
	}
	return convertPropertyDefinition(def), nil
}

func convertPropertyDefinition(d db.EntityPropertyDefinition) models.PropertyDefinition {
	return models.PropertyDefinition{
		ID:            d.ID,
		OrgID:         toUUID(d.OrgID),
		EntityType:    d.EntityType,
		PropertyName:  d.PropertyName,
		PropertyType:  d.PropertyType,
		RefTargetType: stringPtrFromText(d.RefTargetType),
		UILabel:       stringPtrFromText(d.UiLabel),
		IsFilterable:  boolFromPgBool(d.IsFilterable, true),
		CreatedAt:     toTime(d.CreatedAt),
	}
}
