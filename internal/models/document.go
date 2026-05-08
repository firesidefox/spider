package models

import "time"

type DocumentGroup struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Document struct {
	GroupID    *int      `json:"group_id"`
	ID         int       `json:"id"`
	Vendor     string    `json:"vendor"`
	Tags       []string  `json:"tags"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Embedding  []byte    `json:"-"`
	SourceFile string    `json:"source_file"`
	ChunkIndex int       `json:"chunk_index"`
	CreatedAt  time.Time `json:"created_at"`
}
