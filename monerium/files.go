package monerium

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

// UploadFile accepts request with filename and content of the file to be uploaded via generic file upload endpoint.
// UploadFile can be used e.g. for uploading supporting documents for large redeem orders.
func (c *Client) UploadFile(ctx context.Context, req *UploadFileRequest) (*File, error) {
	path := "/files"

	bs, err := c.upload(ctx, path, req.Filename, req.Content)
	if err != nil {
		return nil, err
	}
	var o File
	if err = json.Unmarshal(bs, &o); err != nil {
		return nil, err
	}

	return &o, nil
}

// UploadFileRequest contains filename and content of the file to be uploaded.
type UploadFileRequest struct {
	Filename string
	Content  io.Reader
}

// File represents a file that was successfully uploaded.
type File struct {
	ID   string    `json:"id,omitempty"`
	Name string    `json:"name,omitempty"`
	Type string    `json:"type,omitempty"`
	Size int       `json:"size,omitempty"`
	Hash string    `json:"hash,omitempty"`
	Meta *FileMeta `json:"meta,omitempty"`
}

// FileMeta represents a metadata of a file that was successfully uploaded.
type FileMeta struct {
	UploadedBy string    `json:"uploadedBy,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}
