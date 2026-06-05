package adapters

type Capabilities struct {
	PromptCapture      bool `json:"prompt_capture"`
	ModelCallCapture   bool `json:"model_call_capture"`
	ModelResultCapture bool `json:"model_result_capture"`
	ToolCallCapture    bool `json:"tool_call_capture"`
	ToolResultCapture  bool `json:"tool_result_capture"`
	ShellCapture       bool `json:"shell_capture"`
	FileDiffCapture    bool `json:"file_diff_capture"`
	PermissionCapture  bool `json:"permission_capture"`
	SubagentCapture    bool `json:"subagent_capture"`
	MCPToolCapture     bool `json:"mcp_tool_capture"`
	CanInstallHooks    bool `json:"can_install_hooks"`
	CanRunAsWrapper    bool `json:"can_run_as_wrapper"`
	CanImportTrace     bool `json:"can_import_trace"`
	CanIngestHTTP      bool `json:"can_ingest_http"`
}
