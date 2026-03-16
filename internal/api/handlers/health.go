package handlers

import (
	"time"

	"github.com/go-fuego/fuego"
)

type healthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func Health(c fuego.ContextNoBody) (healthResponse, error) {
	return healthResponse{Status: "ok", Timestamp: time.Now().UTC()}, nil
}
