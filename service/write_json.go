package service

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, v interface{}) {
	WriteJSONStatus(w, http.StatusOK, v)
}

// WriteJSONStatus — тот же WriteJSON, но с явным HTTP-кодом.
// Заголовки обязательно выставляются ДО WriteHeader.
func WriteJSONStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}
