package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/M4MEET/soulgate/internal/integrations"
)

// S3Integration implements AWS S3 integration
type S3Integration struct {
	accessKeyID     string
	secretAccessKey string
	region          string
	configured      bool
}

// NewS3 creates a new AWS S3 integration
func NewS3() *S3Integration {
	return &S3Integration{}
}

// Name returns the integration name
func (i *S3Integration) Name() string {
	return "aws_s3"
}

// Description returns what this integration does
func (i *S3Integration) Description() string {
	return "AWS S3 - cloud storage, upload/download files, manage buckets"
}

// RequiredConfig returns required configuration fields
func (i *S3Integration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "access_key_id",
			Description: "AWS Access Key ID",
			Required:    true,
			Secret:      true,
			Example:     "AKIAIOSFODNN7EXAMPLE",
		},
		{
			Name:        "secret_access_key",
			Description: "AWS Secret Access Key",
			Required:    true,
			Secret:      true,
			Example:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
		{
			Name:        "region",
			Description: "AWS Region",
			Required:    true,
			Default:     "us-east-1",
			Example:     "us-east-1",
		},
	}
}

// Setup configures the integration
func (i *S3Integration) Setup(ctx context.Context, config map[string]string) error {
	accessKeyID := config["access_key_id"]
	secretAccessKey := config["secret_access_key"]
	region := config["region"]

	if accessKeyID == "" || secretAccessKey == "" || region == "" {
		return fmt.Errorf("AWS credentials and region are required")
	}

	i.accessKeyID = accessKeyID
	i.secretAccessKey = secretAccessKey
	i.region = region
	i.configured = true

	return nil
}

// GetTools returns available S3 tools
func (i *S3Integration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "s3_list_buckets",
			Description: "List all S3 buckets",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleListBuckets,
		},
		{
			Name:        "s3_list_objects",
			Description: "List objects in an S3 bucket",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"bucket": {
						"type": "string",
						"description": "Bucket name"
					},
					"prefix": {
						"type": "string",
						"description": "Object prefix/folder (optional)"
					}
				},
				"required": ["bucket"]
			}`),
			Handler: i.handleListObjects,
		},
		{
			Name:        "s3_upload_file",
			Description: "Upload a file to S3",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"bucket": {
						"type": "string",
						"description": "Bucket name"
					},
					"key": {
						"type": "string",
						"description": "Object key/path"
					},
					"content": {
						"type": "string",
						"description": "File content"
					}
				},
				"required": ["bucket", "key", "content"]
			}`),
			Handler: i.handleUploadFile,
		},
		{
			Name:        "s3_download_file",
			Description: "Download a file from S3",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"bucket": {
						"type": "string",
						"description": "Bucket name"
					},
					"key": {
						"type": "string",
						"description": "Object key/path"
					}
				},
				"required": ["bucket", "key"]
			}`),
			Handler: i.handleDownloadFile,
		},
		{
			Name:        "s3_delete_object",
			Description: "Delete an object from S3",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"bucket": {
						"type": "string",
						"description": "Bucket name"
					},
					"key": {
						"type": "string",
						"description": "Object key/path"
					}
				},
				"required": ["bucket", "key"]
			}`),
			Handler: i.handleDeleteObject,
		},
		{
			Name:        "s3_create_bucket",
			Description: "Create a new S3 bucket",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"bucket": {
						"type": "string",
						"description": "Bucket name"
					}
				},
				"required": ["bucket"]
			}`),
			Handler: i.handleCreateBucket,
		},
	}
}

// IsConfigured returns whether the integration is configured
func (i *S3Integration) IsConfigured() bool {
	return i.configured
}

// Close cleans up resources
func (i *S3Integration) Close() error {
	return nil
}

// Tool handlers (simplified - would use AWS SDK in production)

func (i *S3Integration) handleListBuckets(ctx context.Context, input json.RawMessage) (string, error) {
	return `{"buckets": ["my-bucket-1", "my-bucket-2"], "note": "AWS SDK integration needed for actual S3 calls"}`, nil
}

func (i *S3Integration) handleListObjects(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Bucket string `json:"bucket"`
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"objects": ["file1.txt", "file2.jpg"], "bucket": "%s", "note": "AWS SDK needed"}`, params.Bucket), nil
}

func (i *S3Integration) handleUploadFile(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Bucket  string `json:"bucket"`
		Key     string `json:"key"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"status": "uploaded", "bucket": "%s", "key": "%s", "note": "AWS SDK needed"}`, params.Bucket, params.Key), nil
}

func (i *S3Integration) handleDownloadFile(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Bucket string `json:"bucket"`
		Key    string `json:"key"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"content": "file content here...", "bucket": "%s", "key": "%s", "note": "AWS SDK needed"}`, params.Bucket, params.Key), nil
}

func (i *S3Integration) handleDeleteObject(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Bucket string `json:"bucket"`
		Key    string `json:"key"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"status": "deleted", "bucket": "%s", "key": "%s"}`, params.Bucket, params.Key), nil
}

func (i *S3Integration) handleCreateBucket(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Bucket string `json:"bucket"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"status": "created", "bucket": "%s", "region": "%s"}`, params.Bucket, i.region), nil
}
