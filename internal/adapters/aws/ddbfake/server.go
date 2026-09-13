// Package ddbfake is an in-process DynamoDB JSON protocol subset for tests.
package ddbfake

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// AV is a DynamoDB JSON attribute map.
type AV map[string]map[string]any

// Server stores items in memory and speaks enough DynamoDB JSON for adapter tests.
type Server struct {
	mu       sync.Mutex
	tables   map[string]map[string]AV
	Requests [][]byte
	Targets  []string
}

func New() *Server {
	return &Server{tables: map[string]map[string]AV{}}
}

func (s *Server) Seed(table, pk string, item AV) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tables[table] == nil {
		s.tables[table] = map[string]AV{}
	}
	s.tables[table][pk] = clone(item)
}

func (s *Server) Item(table, pk string) (AV, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tables[table][pk]
	return clone(item), ok
}

func (s *Server) LastRequest() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Requests) == 0 {
		return ""
	}
	return string(s.Requests[len(s.Requests)-1])
}

func (s *Server) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Requests)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	target := r.Header.Get("X-Amz-Target")
	s.mu.Lock()
	s.Requests = append(s.Requests, append([]byte(nil), body...))
	s.Targets = append(s.Targets, target)
	s.mu.Unlock()

	op := target
	if i := strings.LastIndex(target, "."); i >= 0 {
		op = target[i+1:]
	}
	switch op {
	case "GetItem":
		s.getItem(w, body)
	case "PutItem":
		s.putItem(w, body)
	case "UpdateItem":
		s.updateItem(w, body)
	case "TransactWriteItems":
		s.transact(w, body)
	default:
		http.Error(w, `{"__type":"UnknownOperationException","message":"`+op+`"}`, http.StatusBadRequest)
	}
}

func (s *Server) getItem(w http.ResponseWriter, body []byte) {
	var req struct {
		TableName string
		Key       AV
	}
	_ = json.Unmarshal(body, &req)
	pk := pkValue(req.Key)
	s.mu.Lock()
	item, ok := s.tables[req.TableName][pk]
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"Item": item})
}

func (s *Server) putItem(w http.ResponseWriter, body []byte) {
	var req struct {
		TableName           string
		Item                AV
		ConditionExpression string
	}
	_ = json.Unmarshal(body, &req)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.putLocked(req.TableName, req.Item, req.ConditionExpression) {
		writeConditionalFailed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *Server) transact(w http.ResponseWriter, body []byte) {
	var req struct {
		TransactItems []struct {
			Put *struct {
				TableName           string
				Item                AV
				ConditionExpression string
			}
		}
	}
	_ = json.Unmarshal(body, &req)
	s.mu.Lock()
	defer s.mu.Unlock()

	reasons := make([]map[string]string, len(req.TransactItems))
	failed := false
	for i, item := range req.TransactItems {
		reasons[i] = map[string]string{"Code": "None"}
		if item.Put == nil {
			continue
		}
		pk := pkValue(item.Put.Item)
		_, exists := s.tables[item.Put.TableName][pk]
		if strings.Contains(item.Put.ConditionExpression, "attribute_not_exists") && exists {
			reasons[i] = map[string]string{"Code": "ConditionalCheckFailed", "Message": "The conditional request failed"}
			failed = true
		}
	}
	if failed {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"__type":              "com.amazonaws.dynamodb.v20120810#TransactionCanceledException",
			"message":             "Transaction cancelled",
			"CancellationReasons": reasons,
		})
		return
	}
	for _, item := range req.TransactItems {
		if item.Put == nil {
			continue
		}
		s.putLocked(item.Put.TableName, item.Put.Item, "")
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *Server) putLocked(table string, item AV, cond string) bool {
	if s.tables[table] == nil {
		s.tables[table] = map[string]AV{}
	}
	pk := pkValue(item)
	_, exists := s.tables[table][pk]
	if strings.Contains(cond, "attribute_not_exists") && exists {
		return false
	}
	s.tables[table][pk] = clone(item)
	return true
}

func (s *Server) updateItem(w http.ResponseWriter, body []byte) {
	var req struct {
		TableName                 string
		Key                       AV
		UpdateExpression          string
		ConditionExpression       string
		ExpressionAttributeValues AV
		ReturnValues              string
	}
	_ = json.Unmarshal(body, &req)
	pk := pkValue(req.Key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tables[req.TableName] == nil {
		s.tables[req.TableName] = map[string]AV{}
	}
	item, ok := s.tables[req.TableName][pk]
	if !ok {
		item = clone(req.Key)
	}
	if req.ConditionExpression != "" && !matchCondition(item, req.ConditionExpression, req.ExpressionAttributeValues) {
		writeConditionalFailed(w)
		return
	}
	applyUpdate(item, req.UpdateExpression, req.ExpressionAttributeValues)
	s.tables[req.TableName][pk] = item
	resp := map[string]any{}
	if strings.EqualFold(req.ReturnValues, "ALL_NEW") {
		resp["Attributes"] = item
	}
	writeJSON(w, http.StatusOK, resp)
}

func matchCondition(item AV, expr string, values AV) bool {
	expr = strings.TrimSpace(expr)
	if strings.Contains(expr, "attribute_not_exists") {
		name := strings.TrimSuffix(strings.TrimPrefix(expr[strings.Index(expr, "(")+1:], ""), ")")
		name = strings.Trim(name, " )")
		_, ok := item[name]
		return !ok
	}
	parts := strings.Split(expr, " AND ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			lhs, rhs, _ := strings.Cut(part, "=")
			lhs, rhs = strings.TrimSpace(lhs), strings.TrimSpace(rhs)
			want := values[rhs]
			got := item[lhs]
			if stringify(got) != stringify(want) {
				return false
			}
		}
	}
	return true
}

func applyUpdate(item AV, expr string, values AV) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "ADD ") {
		rest := strings.TrimPrefix(expr, "ADD ")
		setPart := ""
		if i := strings.Index(rest, " SET "); i >= 0 {
			setPart = rest[i+5:]
			rest = rest[:i]
		}
		field, valName, _ := strings.Cut(strings.TrimSpace(rest), " ")
		field, valName = strings.TrimSpace(field), strings.TrimSpace(valName)
		delta := intValue(values[valName])
		cur := intValue(item[field])
		item[field] = map[string]any{"N": strconv.FormatInt(cur+delta, 10)}
		if setPart != "" {
			applySet(item, setPart, values)
		}
		return
	}
	if strings.HasPrefix(expr, "SET ") {
		applySet(item, strings.TrimPrefix(expr, "SET "), values)
	}
}

func applySet(item AV, expr string, values AV) {
	for _, assign := range strings.Split(expr, ",") {
		lhs, rhs, ok := strings.Cut(assign, "=")
		if !ok {
			continue
		}
		lhs, rhs = strings.TrimSpace(lhs), strings.TrimSpace(rhs)
		if av, ok := values[rhs]; ok {
			item[lhs] = av
		}
	}
}

func pkValue(item AV) string {
	for _, name := range []string{"owner_key", "token_hash", "region", "short_code"} {
		if v, ok := item[name]; ok {
			if s, _ := v["S"].(string); s != "" {
				return s
			}
		}
	}
	for _, v := range item {
		if s, _ := v["S"].(string); s != "" {
			return s
		}
	}
	return ""
}

func intValue(v map[string]any) int64 {
	if v == nil {
		return 0
	}
	switch n := v["N"].(type) {
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	case float64:
		return int64(n)
	}
	return 0
}

func stringify(v map[string]any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func clone(item AV) AV {
	if item == nil {
		return nil
	}
	out := AV{}
	for k, v := range item {
		out[k] = v
	}
	return out
}

func writeConditionalFailed(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"__type":  "com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException",
		"message": "The conditional request failed",
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
