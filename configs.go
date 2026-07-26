// Device configuration: read, write, and persist settings.

package ultimate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ConfigsService reads and writes device settings.
type ConfigsService struct {
	client *Client
}

// ConfigMap is category -> setting name -> current value.
type ConfigMap map[string]map[string]any

// lookup returns the raw value for category+item, or nil if not found.
func (m ConfigMap) lookup(category, item string) any {
	if cat, ok := m[category]; ok {
		return cat[item]
	}
	return nil
}

// String returns the value as a string. Returns false if not found.
func (m ConfigMap) String(category, item string) (string, bool) {
	v := m.lookup(category, item)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	if ok {
		return s, true
	}
	return fmt.Sprint(v), true
}

// Bool parses a config value as a boolean. Recognises "Enabled"/"Disabled",
// "Yes"/"No", "ON"/"OFF" (case-insensitive), and numeric 1/0.
func (m ConfigMap) Bool(category, item string) (bool, bool) {
	v := m.lookup(category, item)
	if v == nil {
		return false, false
	}
	switch s := strings.ToLower(fmt.Sprint(v)); s {
	case "enabled", "yes", "on", "1":
		return true, true
	case "disabled", "no", "off", "0":
		return false, true
	default:
		if n, err := strconv.Atoi(s); err == nil {
			return n != 0, true
		}
		return false, false
	}
}

// Int parses a config value as an integer. Handles JSON numbers, plain numeric
// strings, and strings with unit suffixes (e.g. "2 MB", " 0 dB").
func (m ConfigMap) Int(category, item string) (int, bool) {
	v := m.lookup(category, item)
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case string:
		trimmed := strings.TrimSpace(n)
		if idx := strings.IndexFunc(trimmed, func(r rune) bool {
			return r != '-' && r != '+' && (r < '0' || r > '9')
		}); idx > 0 {
			trimmed = trimmed[:idx]
		}
		if i, err := strconv.Atoi(trimmed); err == nil {
			return i, true
		}
	}
	return 0, false
}

// ConfigItem is the metadata for one device setting.
type ConfigItem struct {
	Current any      `json:"current"`           // Current value (string or number)
	Min     int      `json:"min,omitempty"`     // Minimum value (for numeric settings)
	Max     int      `json:"max,omitempty"`     // Maximum value (for numeric settings)
	Format  string   `json:"format,omitempty"`  // Printf-style display format
	Default any      `json:"default,omitempty"` // Factory default value
	Values  []string `json:"values,omitempty"`  // Allowed values (for choices/enums)
	Presets []string `json:"presets,omitempty"` // Available presets
}

// ConfigItems is category -> setting name -> ConfigItem metadata.
type ConfigItems map[string]map[string]*ConfigItem

// Get retrieves a ConfigItem by category and item name. Returns false if not found.
func (m ConfigItems) Get(category, item string) (*ConfigItem, bool) {
	if cat, ok := m[category]; ok {
		val, ok := cat[item]
		return val, ok
	}
	return nil, false
}

// List returns the names of all setting categories.
func (s *ConfigsService) List(ctx context.Context) ([]string, error) {
	var resp struct {
		Categories []string `json:"categories"`
		Errors     []string `json:"errors"`
	}
	if err := s.client.getJSON(ctx, http.MethodGet, "/v1/configs", nil, "", &resp); err != nil {
		return nil, err
	}
	return resp.Categories, nil
}

// Get returns settings for category. Supports * wildcards in the path.
func (s *ConfigsService) Get(ctx context.Context, category string) (ConfigMap, error) {
	path := "/v1/configs/" + encodePath(category)
	return s.parseConfigMap(ctx, path)
}

// GetItem returns detailed specifications for a setting (supports '*' wildcards).
func (s *ConfigsService) GetItem(ctx context.Context, category, item string) (ConfigItems, error) {
	path := "/v1/configs/" + encodePath(category) + "/" + encodePath(item)
	data, err := s.client.getRaw(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	if errs := extractErrors(data); len(errs) > 0 {
		return nil, &APIError{Errors: errs}
	}
	// Parse the response: { "<category>": { "<item>": {current, min, ...} } }
	raw, err := unmarshalStringMap(data)
	if err != nil {
		return nil, fmt.Errorf("ultimate: parse config item response: %w", err)
	}
	out := make(ConfigItems, len(raw))
	for cat, catJSON := range raw {
		if cat == "errors" {
			continue
		}
		items, err := unmarshalStringMap(catJSON)
		if err != nil {
			return nil, fmt.Errorf("ultimate: parse category %q: %w", cat, err)
		}
		itemMap := make(map[string]*ConfigItem, len(items))
		for name, itemJSON := range items {
			var ci ConfigItem
			if err := json.Unmarshal(itemJSON, &ci); err != nil {
				return nil, fmt.Errorf("ultimate: parse item %q in %q: %w", name, cat, err)
			}
			itemMap[name] = &ci
		}
		out[cat] = itemMap
	}
	return out, nil
}

// Set updates a single setting.
func (s *ConfigsService) Set(ctx context.Context, category, item, value string) error {
	q := url.Values{}
	q.Set("value", value)
	path := "/v1/configs/" + encodePath(category) + "/" + encodePath(item) + "?" + q.Encode()
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// Bool reads a single setting and parses it as a boolean.
// Uses the same parsing rules as ConfigMap.Bool.
func (s *ConfigsService) Bool(ctx context.Context, category, item string) (bool, error) {
	cfg, err := s.Get(ctx, category)
	if err != nil {
		return false, err
	}
	v, ok := cfg.Bool(category, item)
	if !ok {
		return false, fmt.Errorf("ultimate: setting %q / %q not found", category, item)
	}
	return v, nil
}

// SetBool writes a boolean setting using the device's "Enabled"/"Disabled" convention.
func (s *ConfigsService) SetBool(ctx context.Context, category, item string, value bool) error {
	v := "Disabled"
	if value {
		v = "Enabled"
	}
	return s.Set(ctx, category, item, v)
}

// SetInt writes an integer setting.
func (s *ConfigsService) SetInt(ctx context.Context, category, item string, value int) error {
	return s.Set(ctx, category, item, strconv.Itoa(value))
}

// Apply updates multiple settings at once using a ConfigMap.
func (s *ConfigsService) Apply(ctx context.Context, cfg ConfigMap) error {
	body, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("ultimate: marshal config: %w", err)
	}
	return s.client.getJSON(ctx, http.MethodPost, "/v1/configs", bytes.NewReader(body), "application/json", nil)
}

// ConfigOptions limits flash operations to one category.
type ConfigOptions struct {
	Category string // Leave empty to act on all categories
}

// SaveToFlash saves the current settings to non-volatile flash storage.
func (s *ConfigsService) SaveToFlash(ctx context.Context, opts ConfigOptions) error {
	return s.flash(ctx, "save_to_flash", opts)
}

// LoadFromFlash restores settings from flash storage.
func (s *ConfigsService) LoadFromFlash(ctx context.Context, opts ConfigOptions) error {
	return s.flash(ctx, "load_from_flash", opts)
}

// ResetToDefault resets settings to factory defaults.
// Changes are not persisted until SaveToFlash is called.
func (s *ConfigsService) ResetToDefault(ctx context.Context, opts ConfigOptions) error {
	return s.flash(ctx, "reset_to_default", opts)
}

func (s *ConfigsService) flash(ctx context.Context, command string, opts ConfigOptions) error {
	path := "/v1/configs"
	if opts.Category != "" {
		path += "/" + encodePath(opts.Category)
	}
	path += ":" + command
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// parseConfigMap fetches a settings endpoint and returns a ConfigMap.
func (s *ConfigsService) parseConfigMap(ctx context.Context, path string) (ConfigMap, error) {
	data, err := s.client.getRaw(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	if errs := extractErrors(data); len(errs) > 0 {
		return nil, &APIError{Errors: errs}
	}
	raw, err := unmarshalStringMap(data)
	if err != nil {
		return nil, fmt.Errorf("ultimate: parse config response: %w", err)
	}
	out := make(ConfigMap, len(raw))
	for cat, catJSON := range raw {
		if cat == "errors" {
			continue
		}
		items, err := unmarshalStringMap(catJSON)
		if err != nil {
			return nil, fmt.Errorf("ultimate: parse category %q: %w", cat, err)
		}
		values := make(map[string]any, len(items))
		for name, v := range items {
			var val any
			if err := json.Unmarshal(v, &val); err != nil {
				return nil, fmt.Errorf("ultimate: parse item %q in %q: %w", name, cat, err)
			}
			values[name] = val
		}
		out[cat] = values
	}
	return out, nil
}

// extractErrors pulls the "errors" array from a device JSON payload, if present.
func extractErrors(data []byte) []string {
	var ar apiResponse
	if json.Unmarshal(data, &ar) == nil {
		return ar.Errors
	}
	return nil
}

// unmarshalStringMap unmarshals a JSON object preserving raw message values.
func unmarshalStringMap(data []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
