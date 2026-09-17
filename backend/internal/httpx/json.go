package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

func WriteJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)

	w.Header().Set("Cache-control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error al escribir respuesta JSON: %v", err)
	}
}

func WriteError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	WriteJSON(
		w,
		status,
		errorResponse{
			Error: message,
		})
}
