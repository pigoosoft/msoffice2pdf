package repo

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"gorm.io/gorm"
)

// QueryOperator is a SQL comparison / filter operator.
type QueryOperator string

const (
	Equal        QueryOperator = "="
	NotEqual     QueryOperator = "!="
	Greater      QueryOperator = ">"
	GreaterEqual QueryOperator = ">="
	Less         QueryOperator = "<"
	LessEqual    QueryOperator = "<="
	Like         QueryOperator = "LIKE"
	NotLike      QueryOperator = "NOT LIKE"
	In           QueryOperator = "IN"
	NotIn        QueryOperator = "NOT IN"
	Between      QueryOperator = "BETWEEN"
	IsNull       QueryOperator = "IS NULL"
	IsNotNull    QueryOperator = "IS NOT NULL"
)

// Condition is a single query filter.
type Condition struct {
	Field    string
	Value    interface{}
	Operator QueryOperator
}

// QueryBuilder builds GORM queries from field maps / typed conditions.
type QueryBuilder struct {
	db *gorm.DB
	// cache: per model type, Go field name -> DB column name
	cacheMu  sync.RWMutex
	colCache map[reflect.Type]map[string]string
}

// NewQueryBuilder creates a QueryBuilder.
func NewQueryBuilder(db *gorm.DB) *QueryBuilder {
	return &QueryBuilder{db: db, colCache: make(map[reflect.Type]map[string]string)}
}

// BuildSmartQuery builds a query from a conditions map.
// model: GORM model used for reflection / column mapping
// conditions: field -> value map; supports several value formats
// alias: primary-table alias for joins; empty for a single table
func (qb *QueryBuilder) BuildSmartQuery(model interface{}, conditions map[string]interface{}, alias string) (*gorm.DB, error, map[string]interface{}) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	query := qb.db.Model(model)
	notInModel := map[string]interface{}{}
	for field, value := range conditions {
		if !qb.fieldExists(modelType, field) {
			notInModel[field] = value
			continue // skip unknown fields
		}

		query = qb.applyConditionWithModel(query, modelType, field, value, alias)
	}

	return query, nil, notInModel
}

// fieldExists reports whether fieldName exists on the model (supports nesting / __ops).
func (qb *QueryBuilder) fieldExists(modelType reflect.Type, fieldName string) bool {
	// strip operator suffix
	if strings.Contains(fieldName, "__") {
		fieldName = strings.Split(fieldName, "__")[0]
	}
	// nested fields (e.g. "User.Profile.Name")
	if strings.Contains(fieldName, ".") {
		parts := strings.Split(fieldName, ".")
		currentType := modelType

		for _, part := range parts {
			field, exists := currentType.FieldByName(part)
			if !exists {
				return false
			}
			currentType = field.Type
			if currentType.Kind() == reflect.Ptr {
				currentType = currentType.Elem()
			}
		}
		return true
	}

	_, exists := modelType.FieldByName(fieldName)
	return exists
}

// applyConditionWithModel applies one condition using the model type for column mapping.
func (qb *QueryBuilder) applyConditionWithModel(query *gorm.DB, modelType reflect.Type, field string, value interface{}, alias string) *gorm.DB {
	fieldType := qb.getFieldType(modelType, field)
	if fieldType == nil {
		return query
	}

	// operator suffix syntax (e.g. "field__gt", "field__like")
	if strings.Contains(field, "__") {
		parts := strings.Split(field, "__")
		if len(parts) == 2 {
			fieldName := parts[0]
			operator := parts[1]
			col := qb.columnNameFor(modelType, fieldName)
			return qb.applyOperatorCondition(query, col, operator, value, fieldType, alias)
		}
	}

	col := qb.columnNameFor(modelType, field)
	return qb.applySmartCondition(query, col, value, fieldType, alias)
}

// getFieldType returns the Go type of a field path on the model.
func (qb *QueryBuilder) getFieldType(modelType reflect.Type, fieldPath string) reflect.Type {
	// strip operator suffix
	if strings.Contains(fieldPath, "__") {
		fieldPath = strings.Split(fieldPath, "__")[0]
	}
	if !strings.Contains(fieldPath, ".") {
		field, exists := modelType.FieldByName(fieldPath)
		if !exists {
			return nil
		}
		return field.Type
	}

	parts := strings.Split(fieldPath, ".")
	currentType := modelType

	for _, part := range parts {
		field, exists := currentType.FieldByName(part)
		if !exists {
			return nil
		}
		currentType = field.Type
		if currentType.Kind() == reflect.Ptr {
			currentType = currentType.Elem()
		}
	}

	return currentType
}

// applyOperatorCondition applies a condition with an explicit operator suffix.
func (qb *QueryBuilder) applyOperatorCondition(query *gorm.DB, field, operator string, value interface{}, fieldType reflect.Type, alias string) *gorm.DB {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	switch operator {
	case "gt":
		return query.Where(fmt.Sprintf("%s > ?", aliasdot+field), value)
	case "gte":
		return query.Where(fmt.Sprintf("%s >= ?", aliasdot+field), value)
	case "lt":
		return query.Where(fmt.Sprintf("%s < ?", aliasdot+field), value)
	case "lte":
		return query.Where(fmt.Sprintf("%s <= ?", aliasdot+field), value)
	case "like":
		return query.Where(fmt.Sprintf("%s LIKE ?", aliasdot+field), value)
	case "not_like":
		return query.Where(fmt.Sprintf("%s NOT LIKE ?", aliasdot+field), value)
	case "in":
		return query.Where(fmt.Sprintf("%s IN (?)", aliasdot+field), value)
	case "not_in":
		return query.Where(fmt.Sprintf("%s NOT IN (?)", aliasdot+field), value)
	case "null":
		if value == true {
			return query.Where(fmt.Sprintf("%s IS NULL", aliasdot+field))
		}
		return query.Where(fmt.Sprintf("%s IS NOT NULL", aliasdot+field))
	case "between":
		if values, ok := value.([]interface{}); ok && len(values) == 2 {
			minv := Min(values[0], values[1])
			maxv := Max(values[0], values[1])
			return query.Where(fmt.Sprintf(" ( %s >= ? AND %s <= ? ) ", aliasdot+field, aliasdot+field), minv, maxv)
		}
	}

	// default: equality
	return query.Where(fmt.Sprintf("%s = ?", aliasdot+field), value)
}

// applySmartCondition picks a filter style from the field type and value shape.
func (qb *QueryBuilder) applySmartCondition(query *gorm.DB, field string, value interface{}, fieldType reflect.Type, alias string) *gorm.DB {
	a := ""
	if alias != "" {
		a = alias + "."
	}
	if value == nil {
		return query.Where(fmt.Sprintf("%s IS NULL", a+field))
	}

	switch fieldType.Kind() {
	case reflect.String:
		return qb.handleStringField(query, field, value, alias)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return qb.handleNumericField(query, field, value, alias)
	case reflect.Bool:
		return query.Where(fmt.Sprintf("%s = ?", a+field), value)
	case reflect.Struct:
		if fieldType == reflect.TypeOf(time.Time{}) {
			return qb.handleTimeField(query, field, value, alias)
		}
	case reflect.Slice, reflect.Array:
		return qb.handleSliceField(query, field, value, alias)
	case reflect.Ptr:
		elemType := fieldType.Elem()
		return qb.applySmartCondition(query, field, value, elemType, alias)
	}

	// default: equality
	return query.Where(fmt.Sprintf("%s = ?", a+field), value)
}

// handleStringField filters a string column (LIKE / NOT LIKE / equality).
func (qb *QueryBuilder) handleStringField(query *gorm.DB, field string, value interface{}, alias string) *gorm.DB {
	strValue := fmt.Sprintf("%v", value)
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	// values with % use LIKE
	if strings.Contains(strValue, "%") {
		return query.Where(fmt.Sprintf("%s LIKE ?", aliasdot+field), strValue)
	}

	// leading "!" uses NOT LIKE (prefix match)
	if strings.HasPrefix(strValue, "!") {
		return query.Where(fmt.Sprintf("%s NOT LIKE ?", aliasdot+field), strings.TrimPrefix(strValue, "!")+"%")
	}

	// default: exact match
	return query.Where(fmt.Sprintf("%s = ?", aliasdot+field), strValue)
}

// handleNumericField filters a numeric column (ranges, comparisons, negatives).
func (qb *QueryBuilder) handleNumericField(query *gorm.DB, field string, value interface{}, alias string) *gorm.DB {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	// try range forms (e.g. "18-30", "-10--5")
	if strValue, ok := value.(string); ok {
		strValue = strings.TrimSpace(strValue)
		// comparison first (e.g. ">18", "<-30")
		if handled, result := qb.handleComparisonOperators(query, field, strValue, alias); handled {
			return result
		}

		if handled, result := qb.handleRangeQuery(query, field, strValue, alias); handled {
			return result
		}
	}

	// default: exact match
	return query.Where(fmt.Sprintf("%s = ?", aliasdot+field), value)
}

// handleComparisonOperators handles string forms like ">=18", "<30".
func (qb *QueryBuilder) handleComparisonOperators(query *gorm.DB, field, strValue string, alias string) (bool, *gorm.DB) {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	if strings.HasPrefix(strValue, ">=") {
		numStr := strings.TrimPrefix(strValue, ">=")
		numStr = strings.TrimSpace(numStr)
		if num, err := qb.parseNumber(numStr); err == nil {
			return true, query.Where(fmt.Sprintf("%s >= ?", aliasdot+field), num)
		}
	}

	if strings.HasPrefix(strValue, "<=") {
		numStr := strings.TrimPrefix(strValue, "<=")
		numStr = strings.TrimSpace(numStr)
		if num, err := qb.parseNumber(numStr); err == nil {
			return true, query.Where(fmt.Sprintf("%s <= ?", aliasdot+field), num)
		}
	}

	if strings.HasPrefix(strValue, ">") {
		numStr := strings.TrimPrefix(strValue, ">")
		numStr = strings.TrimSpace(numStr)
		if num, err := qb.parseNumber(numStr); err == nil {
			return true, query.Where(fmt.Sprintf("%s > ?", aliasdot+field), num)
		}
	}

	if strings.HasPrefix(strValue, "<") {
		numStr := strings.TrimPrefix(strValue, "<")
		numStr = strings.TrimSpace(numStr)
		if num, err := qb.parseNumber(numStr); err == nil {
			return true, query.Where(fmt.Sprintf("%s < ?", aliasdot+field), num)
		}
	}

	return false, query
}

// handleRangeQuery handles "min-max", "min-", and "-max" numeric ranges.
func (qb *QueryBuilder) handleRangeQuery(query *gorm.DB, field, strValue string, alias string) (bool, *gorm.DB) {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	// match range forms (e.g. "18-30", "-10--5", "10--5")
	pattern := `^(-?\d*\.?\d*)\s*-\s*(-?\d*\.?\d*)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(strValue)

	if len(matches) == 3 {
		minStr, maxStr := matches[1], matches[2]
		minStr = strings.TrimSpace(minStr)
		maxStr = strings.TrimSpace(maxStr)
		var minVal interface{}
		if minStr != "" {
			if min, err := qb.parseNumber(minStr); err == nil {
				minVal = min
			} else {
				return false, query // parse failed
			}
		}

		var maxVal interface{}
		if maxStr != "" {
			if max, err := qb.parseNumber(maxStr); err == nil {
				maxVal = max
			} else {
				return false, query // parse failed
			}
		}

		if minVal != nil && maxVal != nil {
			// full range: min-max
			minv := Min(minVal, maxVal)
			maxv := Max(minVal, maxVal)
			return true, query.Where(fmt.Sprintf(" ( %s >= ? AND %s <= ? ) ", aliasdot+field, aliasdot+field), minv, maxv)
		} else if minVal != nil {
			// lower bound only: min- → >= min
			return true, query.Where(fmt.Sprintf("%s >= ?", aliasdot+field), minVal)
		} else if maxVal != nil {
			// upper bound only: -max → <= max
			return true, query.Where(fmt.Sprintf("%s <= ?", aliasdot+field), maxVal)
		}
	}

	return false, query
}

// parseNumber parses an integer or float string.
func (qb *QueryBuilder) parseNumber(numStr string) (interface{}, error) {
	numStr = strings.TrimSpace(numStr)

	if strings.Contains(numStr, ".") {
		if val, err := strconv.ParseFloat(numStr, 64); err == nil {
			return val, nil
		}
	} else {
		if val, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return val, nil
		}
	}

	return nil, fmt.Errorf("invalid number format: %s", numStr)
}

// handleTimeField filters a time.Time column (range / comparison / equality).
func (qb *QueryBuilder) handleTimeField(query *gorm.DB, field string, value interface{}, alias string) *gorm.DB {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	// date range (e.g. "2023-01-01,2023-12-31")
	if strValue, ok := value.(string); ok {
		if strings.Contains(strValue, ",") {
			parts := strings.Split(strValue, ",")
			if len(parts) == 2 {
				minv := Min(parts[0], parts[1])
				maxv := Max(parts[0], parts[1])
				return query.Where(fmt.Sprintf(" ( %s >= ? AND %s <= ? ) ", aliasdot+field, aliasdot+field), minv, maxv)
			}
		}

		if strings.HasPrefix(strValue, ">=") {
			return query.Where(fmt.Sprintf("%s >= ?", aliasdot+field), strings.TrimPrefix(strValue, ">="))
		} else if strings.HasPrefix(strValue, "<=") {
			return query.Where(fmt.Sprintf("%s <= ?", aliasdot+field), strings.TrimPrefix(strValue, "<="))
		} else if strings.HasPrefix(strValue, ">") {
			return query.Where(fmt.Sprintf("%s > ?", aliasdot+field), strings.TrimPrefix(strValue, ">"))
		} else if strings.HasPrefix(strValue, "<") {
			return query.Where(fmt.Sprintf("%s < ?", aliasdot+field), strings.TrimPrefix(strValue, "<"))
		}
	}

	// default: exact match
	return query.Where(fmt.Sprintf("%s = ?", aliasdot+field), value)
}

// handleSliceField filters with IN when the value is a slice or comma-separated list.
func (qb *QueryBuilder) handleSliceField(query *gorm.DB, field string, value interface{}, alias string) *gorm.DB {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	if reflect.TypeOf(value).Kind() == reflect.Slice {
		return query.Where(fmt.Sprintf("%s IN (?)", aliasdot+field), value)
	}

	if strValue, ok := value.(string); ok {
		if strings.Contains(strValue, ",") {
			values := strings.Split(strValue, ",")
			return query.Where(fmt.Sprintf("%s IN (?)", aliasdot+field), values)
		}
	}

	return query.Where(fmt.Sprintf("%s = ?", aliasdot+field), value)
}

// BuildQueryWithConditions builds a query from typed Condition values.
func (qb *QueryBuilder) BuildQueryWithConditions(model interface{}, conditions []Condition, alias string) (*gorm.DB, error) {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	query := qb.db.Model(model)
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	for _, cond := range conditions {
		col := qb.columnNameFor(modelType, cond.Field)
		query = query.Where(fmt.Sprintf("%s %s ?", aliasdot+col, cond.Operator), cond.Value)
	}

	return query, nil
}

// AddPagination applies OFFSET/LIMIT pagination.
func (qb *QueryBuilder) AddPagination(query *gorm.DB, page, pageSize int) *gorm.DB {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	return query.Offset(offset).Limit(pageSize)
}

// AddSorting applies ORDER BY on a model field.
func (qb *QueryBuilder) AddSorting(query *gorm.DB, model interface{}, field string, descending bool, alias string) *gorm.DB {
	aliasdot := ""
	if alias != "" {
		aliasdot = alias + "."
	}
	if field == "" {
		return query
	}
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	order := aliasdot + qb.columnNameFor(modelType, field)
	if descending {
		order += " DESC"
	} else {
		order += " ASC"
	}

	return query.Order(order)
}

// toColumnName converts a struct field name to snake_case (generic fallback).
func (qb *QueryBuilder) toColumnName(field string) string {
	// strip operator suffix
	if strings.Contains(field, "__") {
		field = strings.Split(field, "__")[0]
	}
	// keep only the last segment (nested a.b.c -> c)
	if strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		field = parts[len(parts)-1]
	}
	var b strings.Builder
	runes := []rune(field)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					b.WriteByte('_')
				} else if i+1 < len(runes) {
					next := runes[i+1]
					if unicode.IsLower(next) {
						b.WriteByte('_')
					}
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// columnNameFor maps a Go field name to a DB column via cache / GORM schema.
func (qb *QueryBuilder) columnNameFor(modelType reflect.Type, field string) string {
	// strip operator suffix and nesting
	if strings.Contains(field, "__") {
		field = strings.Split(field, "__")[0]
	}
	if strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		field = parts[len(parts)-1]
	}

	qb.cacheMu.RLock()
	m, ok := qb.colCache[modelType]
	if ok {
		if col, ok2 := m[field]; ok2 {
			qb.cacheMu.RUnlock()
			return col
		}
	}
	qb.cacheMu.RUnlock()

	qb.cacheMu.Lock()
	defer qb.cacheMu.Unlock()
	stmt := &gorm.Statement{DB: qb.db}
	_ = stmt.Parse(reflect.New(modelType).Interface())
	fieldMap := make(map[string]string)
	if stmt.Schema != nil {
		for _, f := range stmt.Schema.Fields {
			fieldMap[f.Name] = f.DBName
		}
	}
	if qb.colCache == nil {
		qb.colCache = make(map[reflect.Type]map[string]string)
	}
	qb.colCache[modelType] = fieldMap
	if col, ok2 := fieldMap[field]; ok2 {
		return col
	}
	// fall back to snake_case heuristic
	return qb.toColumnName(field)
}

// helpers: generic-like comparison for interface{} values
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func less(a, b interface{}) bool {
	if fa, ok := toFloat64(a); ok {
		if fb, ok2 := toFloat64(b); ok2 {
			return fa < fb
		}
	}
	// fallback string compare
	sa := fmt.Sprintf("%v", a)
	sb := fmt.Sprintf("%v", b)
	return sa < sb
}

func Min(a, b interface{}) interface{} {
	if less(a, b) {
		return a
	}
	return b
}

func Max(a, b interface{}) interface{} {
	if less(a, b) {
		return b
	}
	return a
}
