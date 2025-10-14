package model

import "time"

type Singer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Genre     string    `json:"genre"`
	DebutYear int       `json:"debut_year"`
	CreatedAt time.Time `json:"created_at"`
}
