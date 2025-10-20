package terminus

import (
	"encoding/json"
	"fmt"
)

type literalValue struct {
	Value string
}

func (v literalValue) MarshalJSON() ([]byte, error) {
	if v.Value == "" {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]string{"@value": v.Value})
}

func (v *literalValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		v.Value = ""
		return nil
	}
	if data[0] == '{' {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return err
		}
		if val, ok := obj["@value"].(string); ok {
			v.Value = val
			return nil
		}
		return fmt.Errorf("literalValue: missing @value in %s", string(data))
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	v.Value = str
	return nil
}

type referenceValue struct {
	ID string
}

func (r referenceValue) MarshalJSON() ([]byte, error) {
	if r.ID == "" {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]string{"@id": r.ID})
}

func (r *referenceValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		r.ID = ""
		return nil
	}
	if data[0] == '{' {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return err
		}
		if val, ok := obj["@id"].(string); ok {
			r.ID = val
			return nil
		}
		return fmt.Errorf("referenceValue: missing @id in %s", string(data))
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	r.ID = str
	return nil
}

type entityDocument struct {
	ID               string                   `json:"@id,omitempty"`
	Class            string                   `json:"@type"`
	OrgID            string                   `json:"org_id"`
	EntityType       string                   `json:"type"`
	Name             string                   `json:"name"`
	Description      *string                  `json:"description,omitempty"`
	CustomProperties []customPropertyDocument `json:"custom_properties,omitempty"`
	Relationships    []relationshipDocument   `json:"relationships,omitempty"`
	CreatedAt        string                   `json:"created_at"`
	UpdatedAt        string                   `json:"updated_at"`
}

type customPropertyDocument struct {
	ID       string          `json:"@id,omitempty"`
	Class    string          `json:"@type"`
	Name     string          `json:"name"`
	Value    *literalValue   `json:"value,omitempty"`
	RefValue *referenceValue `json:"ref_value,omitempty"`
	OrgID    string          `json:"org_id"`
}

type relationshipDocument struct {
	ID     string         `json:"@id,omitempty"`
	Class  string         `json:"@type"`
	Name   string         `json:"name"`
	Target referenceValue `json:"target"`
	OrgID  string         `json:"org_id"`
}
