package models

import "time"

type Document struct {
	ID         int       `json:"id"`
	Vendor     string    `json:"vendor"`
	CLIType    string    `json:"cli_type"`
	DocType    string    `json:"doc_type"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Embedding  []byte    `json:"-"`
	SourceFile string    `json:"source_file"`
	ChunkIndex int       `json:"chunk_index"`
	CreatedAt  time.Time `json:"created_at"`
}
