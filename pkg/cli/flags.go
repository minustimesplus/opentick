// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// stdinOnce reads stdin the first time a `-` asks for it, so several `-` share one input.
type stdinOnce struct {
	r    io.Reader
	read bool
	text string
}

func (s *stdinOnce) get() (string, error) {
	if s.read {
		return s.text, nil
	}
	s.read = true
	if s.r == nil {
		return "", nil
	}
	raw, err := io.ReadAll(s.r)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	s.text = string(raw)
	return s.text, nil
}

// schemaType is a property's JSON type, ignoring "null" in a type list.
func schemaType(prop map[string]any) string {
	switch t := prop["type"].(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" {
				return s
			}
		}
	}
	return "string"
}

func itemType(prop map[string]any) string {
	items, _ := prop["items"].(map[string]any)
	if items == nil {
		return "string"
	}
	return schemaType(items)
}

func properties(schema map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	props, _ := schema["properties"].(map[string]any)
	for name, raw := range props {
		if p, ok := raw.(map[string]any); ok {
			out[name] = p
		}
	}
	return out
}

// parseArgs maps `--contact-ids 1 --dry-run positional` onto the tool's JSON-schema parameters.
func parseArgs(t *toolInfo, args []string, stdin *stdinOnce, prog string) (map[string]any, error) {
	props := properties(t.InputSchema)
	out := map[string]any{}
	positional := t.Positional
	hint := fmt.Sprintf("; see: %s help %s", prog, t.Name)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		flag, isFlag := strings.CutPrefix(arg, "--")
		if !isFlag || flag == "" {
			if len(positional) == 0 {
				return nil, fmt.Errorf("unexpected argument %q%s", arg, hint)
			}
			key := positional[0]
			positional = positional[1:]
			if props[key] == nil {
				return nil, fmt.Errorf("%s has no parameter %s%s", t.Name, key, hint)
			}
			if err := setValue(out, key, props[key], arg, stdin); err != nil {
				return nil, err
			}
			continue
		}

		name, inline, hasInline := strings.Cut(flag, "=")
		key := strings.ReplaceAll(name, "-", "_")
		// `--set company=BCG` sets one field, like the Personal CRM's update_contact.
		if key == "set" {
			value := inline
			if !hasInline {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("--set needs field=value")
				}
				i++
				value = args[i]
			}
			field, v, ok := strings.Cut(value, "=")
			field = strings.ReplaceAll(strings.TrimSpace(field), "-", "_")
			if !ok || props[field] == nil {
				return nil, fmt.Errorf("--set takes field=value with a parameter of %s%s", t.Name, hint)
			}
			if err := setValue(out, field, props[field], v, stdin); err != nil {
				return nil, err
			}
			continue
		}
		if off, isNo := strings.CutPrefix(key, "no_"); isNo && props[key] == nil && props[off] != nil && schemaType(props[off]) == "boolean" {
			out[off] = false
			continue
		}
		prop := props[key]
		if prop == nil {
			return nil, fmt.Errorf("%s has no option --%s%s", t.Name, name, hint)
		}
		if schemaType(prop) == "boolean" && !hasInline {
			out[key] = true
			continue
		}
		value := inline
		if !hasInline {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--%s needs a value", name)
			}
			i++
			value = args[i]
		}
		if err := setValue(out, key, prop, value, stdin); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func setValue(out map[string]any, key string, prop map[string]any, raw string, stdin *stdinOnce) error {
	switch schemaType(prop) {
	case "array":
		var items []any
		switch {
		case raw == "-":
			text, err := stdin.get()
			if err != nil {
				return err
			}
			items = idsFrom(text)
		case strings.HasPrefix(strings.TrimSpace(raw), "["):
			if err := json.Unmarshal([]byte(raw), &items); err != nil {
				return fmt.Errorf("--%s: %w", key, err)
			}
		default:
			items = []any{raw}
			// Ids may come comma-separated; text never is split, it can contain commas.
			if t := itemType(prop); t == "integer" || t == "number" {
				items = nil
				for _, part := range strings.Split(raw, ",") {
					if part = strings.TrimSpace(part); part != "" {
						items = append(items, part)
					}
				}
			}
		}
		existing, _ := out[key].([]any)
		for _, it := range items {
			v, err := convert(key, itemType(prop), it)
			if err != nil {
				return err
			}
			existing = append(existing, v)
		}
		if existing == nil {
			existing = []any{}
		}
		out[key] = existing
	case "object":
		if strings.HasPrefix(strings.TrimSpace(raw), "{") {
			var obj map[string]any
			if err := json.Unmarshal([]byte(raw), &obj); err != nil {
				return fmt.Errorf("--%s: %w", key, err)
			}
			out[key] = obj
			return nil
		}
		field, v, ok := strings.Cut(raw, "=")
		if !ok {
			return fmt.Errorf("--%s takes field=value or a JSON object", key)
		}
		obj, _ := out[key].(map[string]any)
		if obj == nil {
			obj = map[string]any{}
		}
		obj[strings.TrimSpace(field)] = v
		out[key] = obj
	default:
		value := any(raw)
		if raw == "-" {
			text, err := stdin.get()
			if err != nil {
				return err
			}
			value = strings.TrimSpace(text)
		}
		v, err := convert(key, schemaType(prop), value)
		if err != nil {
			return err
		}
		out[key] = v
	}
	return nil
}

func convert(key, typ string, v any) (any, error) {
	s, isString := v.(string)
	if !isString {
		if n, ok := v.(json.Number); ok {
			s = n.String()
		} else {
			// Already typed by JSON (a number or object from stdin or an inline array).
			if f, ok := v.(float64); ok && typ == "integer" && f == float64(int64(f)) {
				return int64(f), nil
			}
			return v, nil
		}
	}
	switch typ {
	case "integer":
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("--%s needs a whole number, got %q", strings.ReplaceAll(key, "_", "-"), s)
		}
		return n, nil
	case "number":
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, fmt.Errorf("--%s needs a number, got %q", strings.ReplaceAll(key, "_", "-"), s)
		}
		return f, nil
	case "boolean":
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "yes", "1", "on":
			return true, nil
		case "false", "no", "0", "off":
			return false, nil
		}
		return nil, fmt.Errorf("--%s needs true or false, got %q", strings.ReplaceAll(key, "_", "-"), s)
	}
	return s, nil
}

// idsFrom reads ids piped in: a JSON array (of ids or of objects with an id),
// the output of a search ({"ids": [...]} from --ids-only, or {"contacts": [...]}),
// a paginated list ({"items": [...]}), or ids separated by whitespace or commas.
func idsFrom(text string) []any {
	pick := func(v any) ([]any, bool) {
		arr, ok := v.([]any)
		if !ok {
			return nil, false
		}
		out := make([]any, 0, len(arr))
		for _, it := range arr {
			if obj, isObj := it.(map[string]any); isObj {
				if id, has := obj["id"]; has {
					out = append(out, id)
				}
				continue
			}
			out = append(out, it)
		}
		return out, true
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err == nil {
		if ids, ok := pick(v); ok {
			return ids
		}
		if obj, ok := v.(map[string]any); ok {
			for _, key := range []string{
				"ids",
				"contacts",
				"items",
			} {
				if ids, ok := pick(obj[key]); ok {
					return ids
				}
			}
		}
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r'
	})
	out := make([]any, 0, len(fields))
	for _, f := range fields {
		out = append(out, f)
	}
	return out
}
