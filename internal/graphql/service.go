package graphql

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	gqlhandler "github.com/graphql-go/handler"

	"yourapp/internal/auth"
	"yourapp/internal/models"
	"yourapp/internal/terminus"
)

type Service struct {
	store  terminus.Store
	schema graphql.Schema

	entityType         *graphql.Object
	customPropertyType *graphql.Object
	relationshipType   *graphql.Object
	entityFilter       *graphql.InputObject
	propertyInput      *graphql.InputObject
	entityInput        *graphql.InputObject
}

func NewService(store terminus.Store) (*Service, error) {
	svc := &Service{store: store}
	svc.initTypes()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    svc.queryType(),
		Mutation: svc.mutationType(),
	})
	if err != nil {
		return nil, err
	}
	svc.schema = schema
	return svc, nil
}

func (s *Service) Handler() http.Handler {
	return gqlhandler.New(&gqlhandler.Config{
		Schema:   &s.schema,
		Pretty:   true,
		GraphiQL: false,
	})
}

func (s *Service) PlaygroundHandler() http.Handler {
	const playgroundHTML = `<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>GraphQL Playground</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
    <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
    <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
    <style>
      body {
        margin: 0;
        background: #172a3a;
      }
      #root {
        height: 100vh;
      }
    </style>
  </head>
  <body>
    <div id="root"></div>
    <script>
      window.addEventListener('load', function () {
        const url = new URL(window.location.href);
        const basePath = url.pathname.replace(/\/playground$/, '');
        const endpoint = basePath === '' ? '/graphql' : basePath;
        GraphQLPlayground.init(document.getElementById('root'), {
          endpoint,
          settings: {
            'request.credentials': 'include'
          }
        });
      });
    </script>
  </body>
</html>`

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(playgroundHTML))
	})
}

func (s *Service) initTypes() {
	s.entityType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Entity",
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"id": &graphql.Field{Type: graphql.NewNonNull(graphql.ID), Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					return entity.ID, nil
				}},
				"orgId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID), Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					return entity.OrgID.String(), nil
				}},
				"type": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					return entity.Type, nil
				}},
				"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					return entity.Name, nil
				}},
				"description": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					if entity.Description == nil {
						return nil, nil
					}
					return *entity.Description, nil
				}},
				"customProperties": &graphql.Field{Type: graphql.NewList(s.customPropertyType), Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					return entity.CustomProperties, nil
				}},
				"relationships": &graphql.Field{Type: graphql.NewList(s.relationshipType), Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					return entity.Relationships, nil
				}},
				"createdAt": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					if entity.CreatedAt.IsZero() {
						return nil, nil
					}
					return entity.CreatedAt.UTC().Format(time.RFC3339), nil
				}},
				"updatedAt": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
					entity, err := asEntity(p.Source)
					if err != nil {
						return nil, err
					}
					if entity.UpdatedAt.IsZero() {
						return nil, nil
					}
					return entity.UpdatedAt.UTC().Format(time.RFC3339), nil
				}},
			}
		}),
	})

	s.customPropertyType = graphql.NewObject(graphql.ObjectConfig{
		Name: "CustomProperty",
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"id": &graphql.Field{Type: graphql.NewNonNull(graphql.ID), Resolve: func(p graphql.ResolveParams) (any, error) {
					prop, err := asCustomProperty(p.Source)
					if err != nil {
						return nil, err
					}
					return prop.ID, nil
				}},
				"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
					prop, err := asCustomProperty(p.Source)
					if err != nil {
						return nil, err
					}
					return prop.Name, nil
				}},
				"value": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
					prop, err := asCustomProperty(p.Source)
					if err != nil {
						return nil, err
					}
					if prop.Value == nil {
						return nil, nil
					}
					return *prop.Value, nil
				}},
				"refValue": &graphql.Field{Type: s.entityType, Resolve: func(p graphql.ResolveParams) (any, error) {
					prop, err := asCustomProperty(p.Source)
					if err != nil {
						return nil, err
					}
					if prop.RefValueID == nil || *prop.RefValueID == "" {
						return nil, nil
					}
					sess, ok := auth.SessionFromContext(p.Context)
					if !ok || sess == nil {
						return nil, errors.New("missing session in context")
					}
					entity, err := s.store.GetEntity(p.Context, sess.ActiveOrg, *prop.RefValueID)
					if err != nil {
						return nil, err
					}
					return entity, nil
				}},
			}
		}),
	})

	s.relationshipType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Relationship",
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"id": &graphql.Field{Type: graphql.NewNonNull(graphql.ID), Resolve: func(p graphql.ResolveParams) (any, error) {
					rel, err := asRelationship(p.Source)
					if err != nil {
						return nil, err
					}
					return rel.ID, nil
				}},
				"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
					rel, err := asRelationship(p.Source)
					if err != nil {
						return nil, err
					}
					return rel.Name, nil
				}},
				"target": &graphql.Field{Type: s.entityType, Resolve: func(p graphql.ResolveParams) (any, error) {
					rel, err := asRelationship(p.Source)
					if err != nil {
						return nil, err
					}
					if rel.TargetID == "" {
						return nil, nil
					}
					sess, ok := auth.SessionFromContext(p.Context)
					if !ok || sess == nil {
						return nil, errors.New("missing session in context")
					}
					entity, err := s.store.GetEntity(p.Context, sess.ActiveOrg, rel.TargetID)
					if err != nil {
						return nil, err
					}
					return entity, nil
				}},
			}
		}),
	})

	s.entityFilter = graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "EntityFilter",
		Fields: graphql.InputObjectConfigFieldMap{
			"type":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"nameContains": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})

	s.propertyInput = graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "PropertyInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"name":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"value":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"refValueId": &graphql.InputObjectFieldConfig{Type: graphql.ID},
		},
	})

	s.entityInput = graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "EntityInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"type":             &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"name":             &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"description":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"customProperties": &graphql.InputObjectFieldConfig{Type: graphql.NewList(s.propertyInput)},
		},
	})
}

func (s *Service) queryType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"entities": &graphql.Field{
				Type: graphql.NewList(s.entityType),
				Args: graphql.FieldConfigArgument{
					"filter": &graphql.ArgumentConfig{Type: s.entityFilter},
					"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
					"offset": &graphql.ArgumentConfig{Type: graphql.Int},
				},
				Resolve: s.resolveEntities,
			},
			"entity": &graphql.Field{
				Type: s.entityType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: s.resolveEntity,
			},
		},
	})
}

func (s *Service) mutationType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createEntity": &graphql.Field{
				Type: s.entityType,
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(s.entityInput)},
				},
				Resolve: s.resolveCreateEntity,
			},
			"updateEntity": &graphql.Field{
				Type: s.entityType,
				Args: graphql.FieldConfigArgument{
					"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(s.entityInput)},
				},
				Resolve: s.resolveUpdateEntity,
			},
		},
	})
}

func (s *Service) resolveEntities(p graphql.ResolveParams) (any, error) {
	sess, ok := auth.SessionFromContext(p.Context)
	if !ok || sess == nil {
		return nil, errors.New("missing session in context")
	}
	var filter models.EntityFilter
	if rawFilter, ok := p.Args["filter"].(map[string]any); ok {
		if v, ok := rawFilter["type"].(string); ok && strings.TrimSpace(v) != "" {
			trimmed := strings.TrimSpace(v)
			filter.Type = &trimmed
		}
		if v, ok := rawFilter["nameContains"].(string); ok && strings.TrimSpace(v) != "" {
			trimmed := strings.TrimSpace(v)
			filter.NameContains = &trimmed
		}
	}
	limit := 50
	if v, ok := p.Args["limit"].(int); ok && v > 0 {
		limit = v
	}
	if v, ok := p.Args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}
	if limit > 200 {
		limit = 200
	}
	offset := 0
	if v, ok := p.Args["offset"].(int); ok && v >= 0 {
		offset = v
	}
	if v, ok := p.Args["offset"].(float64); ok && v >= 0 {
		offset = int(v)
	}
	entities, err := s.store.ListEntities(p.Context, terminus.ListEntitiesParams{
		OrgID:  sess.ActiveOrg,
		Filter: filter,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *Service) resolveEntity(p graphql.ResolveParams) (any, error) {
	sess, ok := auth.SessionFromContext(p.Context)
	if !ok || sess == nil {
		return nil, errors.New("missing session in context")
	}
	id, _ := p.Args["id"].(string)
	if id == "" {
		return nil, errors.New("id is required")
	}
	entity, err := s.store.GetEntity(p.Context, sess.ActiveOrg, id)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *Service) resolveCreateEntity(p graphql.ResolveParams) (any, error) {
	sess, ok := auth.SessionFromContext(p.Context)
	if !ok || sess == nil {
		return nil, errors.New("missing session in context")
	}
	input, err := s.parseEntityInput(p.Args["input"])
	if err != nil {
		return nil, err
	}
	sanitized, err := s.validateEntityInput(p.Context, sess.ActiveOrg, input)
	if err != nil {
		return nil, err
	}
	entity, err := s.store.CreateEntity(p.Context, sess.ActiveOrg, sanitized)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *Service) resolveUpdateEntity(p graphql.ResolveParams) (any, error) {
	sess, ok := auth.SessionFromContext(p.Context)
	if !ok || sess == nil {
		return nil, errors.New("missing session in context")
	}
	id, _ := p.Args["id"].(string)
	if id == "" {
		return nil, errors.New("id is required")
	}
	existing, err := s.store.GetEntity(p.Context, sess.ActiveOrg, id)
	if err != nil {
		return nil, err
	}
	input, err := s.parseEntityInput(p.Args["input"])
	if err != nil {
		return nil, err
	}
	sanitized, err := s.validateEntityInput(p.Context, sess.ActiveOrg, input)
	if err != nil {
		return nil, err
	}
	updated, err := s.store.UpdateEntity(p.Context, sess.ActiveOrg, existing, sanitized)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) parseEntityInput(raw any) (models.EntityInput, error) {
	inputMap, ok := raw.(map[string]any)
	if !ok {
		return models.EntityInput{}, errors.New("invalid entity input")
	}
	typeVal, ok := inputMap["type"].(string)
	if !ok || strings.TrimSpace(typeVal) == "" {
		return models.EntityInput{}, errors.New("type is required")
	}
	nameVal, ok := inputMap["name"].(string)
	if !ok || strings.TrimSpace(nameVal) == "" {
		return models.EntityInput{}, errors.New("name is required")
	}
	input := models.EntityInput{
		Type: strings.TrimSpace(typeVal),
		Name: strings.TrimSpace(nameVal),
	}
	if descRaw, ok := inputMap["description"].(string); ok {
		trimmed := strings.TrimSpace(descRaw)
		if trimmed != "" {
			input.Description = &trimmed
		}
	}
	if propsRaw, ok := inputMap["customProperties"].([]any); ok {
		props := make([]models.PropertyInput, 0, len(propsRaw))
		for _, item := range propsRaw {
			propMap, ok := item.(map[string]any)
			if !ok {
				return models.EntityInput{}, errors.New("invalid property input")
			}
			name, ok := propMap["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return models.EntityInput{}, errors.New("property name is required")
			}
			prop := models.PropertyInput{Name: strings.TrimSpace(name)}
			if val, ok := propMap["value"].(string); ok {
				trimmed := strings.TrimSpace(val)
				if trimmed != "" {
					prop.Value = &trimmed
				}
			}
			if val, ok := propMap["refValueId"].(string); ok {
				trimmed := strings.TrimSpace(val)
				if trimmed != "" {
					prop.RefValueID = &trimmed
				}
			}
			props = append(props, prop)
		}
		input.CustomProperties = props
	}
	return input, nil
}

func (s *Service) validateEntityInput(ctx context.Context, orgID uuid.UUID, input models.EntityInput) (models.EntityInput, error) {
	defs, err := s.store.ListPropertyDefinitions(ctx, orgID, input.Type)
	if err != nil {
		return models.EntityInput{}, err
	}
	defByName := make(map[string]models.PropertyDefinition, len(defs))
	for _, def := range defs {
		defByName[strings.ToLower(def.PropertyName)] = def
	}
	seen := map[string]struct{}{}
	sanitized := input
	sanitized.CustomProperties = make([]models.PropertyInput, 0, len(input.CustomProperties))
	for _, prop := range input.CustomProperties {
		key := strings.ToLower(prop.Name)
		if _, exists := seen[key]; exists {
			return models.EntityInput{}, fmt.Errorf("duplicate property %s", prop.Name)
		}
		seen[key] = struct{}{}
		def, ok := defByName[key]
		if !ok {
			return models.EntityInput{}, fmt.Errorf("property %s is not defined for type %s", prop.Name, input.Type)
		}
		switch strings.ToLower(def.PropertyType) {
		case "ref":
			if prop.RefValueID == nil || *prop.RefValueID == "" {
				return models.EntityInput{}, fmt.Errorf("property %s requires refValueId", prop.Name)
			}
			prop.Value = nil
		case "number":
			if prop.Value == nil || *prop.Value == "" {
				return models.EntityInput{}, fmt.Errorf("property %s requires value", prop.Name)
			}
			if _, err := strconv.ParseFloat(*prop.Value, 64); err != nil {
				return models.EntityInput{}, fmt.Errorf("property %s value must be numeric", prop.Name)
			}
			prop.RefValueID = nil
		default:
			if prop.Value == nil || *prop.Value == "" {
				return models.EntityInput{}, fmt.Errorf("property %s requires value", prop.Name)
			}
			prop.RefValueID = nil
		}
		sanitized.CustomProperties = append(sanitized.CustomProperties, prop)
	}
	return sanitized, nil
}

func asEntity(src any) (models.Entity, error) {
	if entity, ok := src.(models.Entity); ok {
		return entity, nil
	}
	if entityPtr, ok := src.(*models.Entity); ok && entityPtr != nil {
		return *entityPtr, nil
	}
	return models.Entity{}, errors.New("invalid entity")
}

func asCustomProperty(src any) (models.CustomProperty, error) {
	if prop, ok := src.(models.CustomProperty); ok {
		return prop, nil
	}
	if propPtr, ok := src.(*models.CustomProperty); ok && propPtr != nil {
		return *propPtr, nil
	}
	return models.CustomProperty{}, errors.New("invalid custom property")
}

func asRelationship(src any) (models.Relationship, error) {
	if rel, ok := src.(models.Relationship); ok {
		return rel, nil
	}
	if relPtr, ok := src.(*models.Relationship); ok && relPtr != nil {
		return *relPtr, nil
	}
	return models.Relationship{}, errors.New("invalid relationship")
}
