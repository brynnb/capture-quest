package world

import (
	"capturequest/internal/db"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
)

// ValidateNameContext preserves format/filter policy with an owned query context.
func ValidateNameContext(ctx context.Context, database db.ContextDBTX, name string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if len(name) < 4 || len(name) > 15 {
		return false, "Invalid name format. Names must be 4-15 characters long.", nil
	}
	if !unicode.IsUpper(rune(name[0])) {
		return false, "Invalid name format. Names must start with an uppercase letter.", nil
	}
	for index, char := range name {
		if index > 0 && (!unicode.IsLetter(char) || unicode.IsUpper(char)) {
			return false, "Invalid name format. Names must contain only letters, and only the first letter can be uppercase.", nil
		}
	}
	rows, err := database.QueryContext(ctx, `SELECT word FROM disallowed_words WHERE word<>''`)
	if err != nil {
		return false, "", fmt.Errorf("name filter query: %w", err)
	}
	defer rows.Close()
	lowerName := strings.ToLower(name)
	matched := ""
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return false, "", fmt.Errorf("name filter scan: %w", err)
		}
		word = strings.ToLower(strings.TrimSpace(word))
		if word != "" && strings.Contains(lowerName, word) {
			matched = word
		}
	}
	if err := rows.Err(); err != nil {
		return false, "", fmt.Errorf("name filter iteration: %w", err)
	}
	if matched != "" {
		return false, "The name contains a disallowed phrase.", nil
	}
	return true, "", nil
}

// ItemToMap converts an item model to a map with lowercased keys for JSON compatibility.
func ItemToMap(item any) map[string]interface{} {
	if m, ok := StructToMap(item).(map[string]interface{}); ok {
		return m
	}
	return nil
}

// StructToMap converts any struct (or slice of structs) to a map with camelCase keys.
// This is used to ensure Go models generate JSON that matches TypeScript expectations.
func StructToMap(obj interface{}) interface{} {
	if obj == nil {
		return nil
	}

	v := reflect.ValueOf(obj)
	// Handle pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	// Handle slices
	if v.Kind() == reflect.Slice {
		res := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			res[i] = StructToMap(v.Index(i).Interface())
		}
		return res
	}

	// Handle maps (recurse into values)
	if v.Kind() == reflect.Map {
		res := make(map[string]interface{})
		for _, key := range v.MapKeys() {
			// Convert key to string if it isn't already
			strKey := fmt.Sprintf("%v", key.Interface())
			res[strKey] = StructToMap(v.MapIndex(key).Interface())
		}
		return res
	}

	// Special case for time.Time - return RFC3339 string
	if t, ok := obj.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	if t, ok := obj.(*time.Time); ok && t != nil {
		return t.Format(time.RFC3339)
	}

	// If not a struct, return as is (base types)
	if v.Kind() != reflect.Struct {
		return obj
	}

	res := make(map[string]interface{})
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		name := field.Name
		// Special case for ID or fields ending in ID
		if name == "ID" {
			name = "id"
		} else if strings.HasSuffix(name, "ID") {
			// e.g. AllocationID -> allocationId
			name = strings.ToLower(name[:1]) + name[1:len(name)-2] + "Id"
		} else {
			// standard camelCase: lower the first letter
			name = strings.ToLower(name[:1]) + name[1:]
		}

		val := v.Field(i).Interface()

		// Recurse into slices or structs
		res[name] = StructToMap(val)
	}

	return res
}
