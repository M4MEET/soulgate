package sdk

// Protocol defines the plugin-host communication protocol

// ToolExecuteRequest is sent to the plugin to execute a tool
type ToolExecuteRequest struct {
	ToolName string `json:"tool_name"`
	Input    string `json:"input"` // JSON-encoded input
}

// ToolExecuteResponse is returned by the plugin after execution
type ToolExecuteResponse struct {
	Output string `json:"output"` // JSON-encoded output
	Error  string `json:"error,omitempty"`
}

// BrokerRequest is sent from plugin to host to call a broker
type BrokerRequest struct {
	Broker    string `json:"broker"`    // e.g., "files"
	Operation string `json:"operation"` // e.g., "read"
	Params    string `json:"params"`    // JSON-encoded parameters
}

// BrokerResponse is returned from host to plugin
type BrokerResponse struct {
	Result string `json:"result"` // JSON-encoded result
	Error  string `json:"error,omitempty"`
}

// Common broker operation parameters

// FilesReadParams for files.read broker operation
type FilesReadParams struct {
	Path string `json:"path"`
}

// FilesReadResult for files.read broker operation
type FilesReadResult struct {
	Content string `json:"content"` // Base64-encoded content
}

// FilesListParams for files.list broker operation
type FilesListParams struct {
	Path string `json:"path"`
}

// FilesListResult for files.list broker operation
type FilesListResult struct {
	Files []FileInfo `json:"files"`
}

// FilesStatParams for files.stat broker operation
type FilesStatParams struct {
	Path string `json:"path"`
}

// FilesStatResult for files.stat broker operation
type FilesStatResult struct {
	File FileInfo `json:"file"`
}

// FileInfo represents file metadata
type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}
