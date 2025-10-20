package terminus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"yourapp/internal/models"
)

type Config struct {
	BaseURL  string
	Team     string
	Database string
	Branch   string
	Token    string
	User     string
	Password string
	Timeout  time.Duration
}

type ListEntitiesParams struct {
	OrgID  uuid.UUID
	Filter models.EntityFilter
	Limit  int
	Offset int
}

type Store interface {
	ListEntities(ctx context.Context, params ListEntitiesParams) ([]models.Entity, error)
	GetEntity(ctx context.Context, orgID uuid.UUID, id string) (models.Entity, error)
	CreateEntity(ctx context.Context, orgID uuid.UUID, input models.EntityInput) (models.Entity, error)
	UpdateEntity(ctx context.Context, orgID uuid.UUID, existing models.Entity, input models.EntityInput) (models.Entity, error)
	ListPropertyDefinitions(ctx context.Context, orgID uuid.UUID, entityType string) ([]models.PropertyDefinition, error)
}

type Client struct {
	httpClient *http.Client
	cfg        Config
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("terminus base url required")
	}
	if cfg.Team == "" {
		return nil, fmt.Errorf("terminus team required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("terminus database required")
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.Token == "" && (cfg.User == "" || cfg.Password == "") {
		return nil, fmt.Errorf("terminus authentication required: provide token or username/password")
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		cfg:        cfg,
	}, nil
}

func (c *Client) ListEntities(ctx context.Context, params ListEntitiesParams) ([]models.Entity, error) {
	endpoint, err := c.documentURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("branch", c.cfg.Branch)
	q.Set("type", "Entity")
	q.Set("prefixed", "false")
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		q.Set("skip", strconv.Itoa(params.Offset))
	}
	req.URL.RawQuery = q.Encode()
	c.attachAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("terminus list entities: %s: %s", resp.Status, string(body))
	}
	var docs []entityDocument
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		return nil, err
	}
	entities := make([]models.Entity, 0, len(docs))
	for _, doc := range docs {
		entity, err := doc.toModel()
		if err != nil {
			return nil, err
		}
		if entity.OrgID != params.OrgID {
			continue
		}
		if params.Filter.Type != nil && *params.Filter.Type != "" && entity.Type != *params.Filter.Type {
			continue
		}
		if params.Filter.NameContains != nil && *params.Filter.NameContains != "" {
			if !strings.Contains(strings.ToLower(entity.Name), strings.ToLower(*params.Filter.NameContains)) {
				continue
			}
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (c *Client) GetEntity(ctx context.Context, orgID uuid.UUID, id string) (models.Entity, error) {
	endpoint, err := c.documentURL()
	if err != nil {
		return models.Entity{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return models.Entity{}, err
	}
	q := req.URL.Query()
	q.Set("branch", c.cfg.Branch)
	q.Set("id", id)
	q.Set("prefixed", "false")
	req.URL.RawQuery = q.Encode()
	c.attachAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.Entity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return models.Entity{}, fmt.Errorf("entity not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return models.Entity{}, fmt.Errorf("terminus get entity: %s: %s", resp.Status, string(body))
	}
	var doc entityDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return models.Entity{}, err
	}
	entity, err := doc.toModel()
	if err != nil {
		return models.Entity{}, err
	}
	if entity.OrgID != orgID {
		return models.Entity{}, fmt.Errorf("entity not found")
	}
	return entity, nil
}

func (c *Client) CreateEntity(ctx context.Context, orgID uuid.UUID, input models.EntityInput) (models.Entity, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	doc := entityDocument{
		Class:       "Entity",
		OrgID:       orgID.String(),
		EntityType:  input.Type,
		Name:        input.Name,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	doc.CustomProperties = buildCustomPropertyDocs(orgID, nil, input.CustomProperties)
	payload, err := json.Marshal([]entityDocument{doc})
	if err != nil {
		return models.Entity{}, err
	}
	entityDoc, err := c.writeDocuments(ctx, http.MethodPost, payload)
	if err != nil {
		return models.Entity{}, err
	}
	entity, err := entityDoc.toModel()
	if err != nil {
		return models.Entity{}, err
	}
	return entity, nil
}

func (c *Client) UpdateEntity(ctx context.Context, orgID uuid.UUID, existing models.Entity, input models.EntityInput) (models.Entity, error) {
	doc := entityDocument{
		ID:          existing.ID,
		Class:       "Entity",
		OrgID:       orgID.String(),
		EntityType:  input.Type,
		Name:        input.Name,
		Description: input.Description,
		CreatedAt:   existing.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	doc.CustomProperties = buildCustomPropertyDocs(orgID, &existing, input.CustomProperties)
	payload, err := json.Marshal([]entityDocument{doc})
	if err != nil {
		return models.Entity{}, err
	}
	entityDoc, err := c.writeDocuments(ctx, http.MethodPut, payload)
	if err != nil {
		return models.Entity{}, err
	}
	entity, err := entityDoc.toModel()
	if err != nil {
		return models.Entity{}, err
	}
	return entity, nil
}

func (c *Client) ListPropertyDefinitions(ctx context.Context, orgID uuid.UUID, entityType string) ([]models.PropertyDefinition, error) {
	endpoint, err := c.documentURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("branch", c.cfg.Branch)
	q.Set("type", "PropertyDefinition")
	q.Set("prefixed", "false")
	req.URL.RawQuery = q.Encode()
	c.attachAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("terminus list property definitions: %s: %s", resp.Status, string(body))
	}
	docs, err := decodePropertyDefinitionDocuments(resp.Body)
	if err != nil {
		return nil, err
	}
	defs := make([]models.PropertyDefinition, 0, len(docs))
	for _, doc := range docs {
		def, err := doc.toModel()
		if err != nil {
			return nil, err
		}
		if def.OrgID != orgID {
			continue
		}
		if entityType != "" && !strings.EqualFold(def.EntityType, entityType) {
			continue
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func (c *Client) writeDocuments(ctx context.Context, method string, payload []byte) (entityDocument, error) {
	endpoint, err := c.documentURL()
	if err != nil {
		return entityDocument{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return entityDocument{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	q := req.URL.Query()
	q.Set("branch", c.cfg.Branch)
	req.URL.RawQuery = q.Encode()
	c.attachAuth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return entityDocument{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return entityDocument{}, fmt.Errorf("terminus write document: %s: %s", resp.Status, string(body))
	}
	var docs []entityDocument
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		return entityDocument{}, err
	}
	if len(docs) == 0 {
		return entityDocument{}, fmt.Errorf("terminus: empty response")
	}
	return docs[0], nil
}

func decodePropertyDefinitionDocuments(r io.Reader) ([]propertyDefinitionDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	switch trimmed[0] {
	case '[':
		var docs []propertyDefinitionDocument
		if err := json.Unmarshal(trimmed, &docs); err != nil {
			return nil, err
		}
		return docs, nil
	case '{':
		var doc propertyDefinitionDocument
		if err := json.Unmarshal(trimmed, &doc); err != nil {
			return nil, err
		}
		return []propertyDefinitionDocument{doc}, nil
	default:
		return nil, fmt.Errorf("terminus: unexpected property definition response: %s", string(trimmed))
	}
}

func (c *Client) documentURL(extra ...string) (string, error) {
	base, err := url.Parse(strings.TrimRight(c.cfg.BaseURL, "/"))
	if err != nil {
		return "", err
	}
	segments := []string{"api", "document", url.PathEscape(c.cfg.Team), url.PathEscape(c.cfg.Database)}
	segments = append(segments, extra...)
	base.Path = path.Join(append([]string{base.Path}, segments...)...)
	return base.String(), nil
}

func (c *Client) attachAuth(req *http.Request) {
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
		return
	}
	if c.cfg.User != "" || c.cfg.Password != "" {
		req.SetBasicAuth(c.cfg.User, c.cfg.Password)
	}
}

func buildCustomPropertyDocs(orgID uuid.UUID, existing *models.Entity, inputs []models.PropertyInput) []customPropertyDocument {
	if len(inputs) == 0 {
		return nil
	}
	existingByName := map[string]models.CustomProperty{}
	if existing != nil {
		for _, prop := range existing.CustomProperties {
			existingByName[strings.ToLower(prop.Name)] = prop
		}
	}
	docs := make([]customPropertyDocument, 0, len(inputs))
	for _, input := range inputs {
		lower := strings.ToLower(input.Name)
		doc := customPropertyDocument{
			Class: "CustomProperty",
			Name:  input.Name,
			OrgID: orgID.String(),
		}
		if existingProp, ok := existingByName[lower]; ok {
			doc.ID = existingProp.ID
		}
		if input.RefValueID != nil && *input.RefValueID != "" {
			doc.RefValue = &referenceValue{ID: *input.RefValueID}
		} else if input.Value != nil {
			doc.Value = &literalValue{Value: *input.Value}
		}
		docs = append(docs, doc)
	}
	return docs
}

func (d propertyDefinitionDocument) toModel() (models.PropertyDefinition, error) {
	def := models.PropertyDefinition{
		ID:            d.ID,
		EntityType:    d.EntityType,
		PropertyName:  d.PropertyName,
		PropertyType:  d.PropertyType,
		RefTargetType: d.RefTargetType,
		UILabel:       d.UILabel,
		IsFilterable:  true,
	}
	if d.IsFilterable != nil {
		def.IsFilterable = *d.IsFilterable
	}
	if d.OrgID == "" {
		return models.PropertyDefinition{}, fmt.Errorf("property definition %s missing org_id", d.ID)
	}
	uid, err := uuid.Parse(d.OrgID)
	if err != nil {
		return models.PropertyDefinition{}, fmt.Errorf("invalid org id on property definition %s: %w", d.ID, err)
	}
	def.OrgID = uid
	if d.CreatedAt != "" {
		if ts, err := time.Parse(time.RFC3339Nano, d.CreatedAt); err == nil {
			def.CreatedAt = &ts
		}
	}
	return def, nil
}

func (d entityDocument) toModel() (models.Entity, error) {
	entity := models.Entity{
		ID:          d.ID,
		Type:        d.EntityType,
		Name:        d.Name,
		Description: d.Description,
	}
	if d.OrgID != "" {
		uid, err := uuid.Parse(d.OrgID)
		if err != nil {
			return models.Entity{}, fmt.Errorf("invalid org id on entity %s: %w", d.ID, err)
		}
		entity.OrgID = uid
	}
	if t, err := time.Parse(time.RFC3339Nano, d.CreatedAt); err == nil {
		entity.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, d.UpdatedAt); err == nil {
		entity.UpdatedAt = t
	}
	if len(d.CustomProperties) > 0 {
		props := make([]models.CustomProperty, 0, len(d.CustomProperties))
		for _, prop := range d.CustomProperties {
			modelProp := models.CustomProperty{
				ID:   prop.ID,
				Name: prop.Name,
			}
			if prop.OrgID != "" {
				if uid, err := uuid.Parse(prop.OrgID); err == nil {
					modelProp.OrgID = uid
				}
			} else {
				modelProp.OrgID = entity.OrgID
			}
			if prop.Value != nil && prop.Value.Value != "" {
				val := prop.Value.Value
				modelProp.Value = &val
			}
			if prop.RefValue != nil && prop.RefValue.ID != "" {
				ref := prop.RefValue.ID
				modelProp.RefValueID = &ref
			}
			props = append(props, modelProp)
		}
		entity.CustomProperties = props
	}
	if len(d.Relationships) > 0 {
		rels := make([]models.Relationship, 0, len(d.Relationships))
		for _, rel := range d.Relationships {
			modelRel := models.Relationship{
				ID:       rel.ID,
				Name:     rel.Name,
				TargetID: rel.Target.ID,
			}
			if rel.OrgID != "" {
				if uid, err := uuid.Parse(rel.OrgID); err == nil {
					modelRel.OrgID = uid
				}
			} else {
				modelRel.OrgID = entity.OrgID
			}
			rels = append(rels, modelRel)
		}
		entity.Relationships = rels
	}
	return entity, nil
}
