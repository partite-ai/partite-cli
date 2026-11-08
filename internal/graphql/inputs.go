package graphql

import "encoding/json"

type NewAgent struct {
	Name                      string                                        `json:"name"`
	Description               Optional[string]                              `json:"description,omitzero"`
	Tags                      []*Tag                                        `json:"tags"`
	Identity                  Optional[string]                              `json:"identity,omitzero"`
	TaskInstructions          string                                        `json:"taskInstructions"`
	ModelComplexity           ModelComplexity                               `json:"modelComplexity"`
	InputMessageType          string                                        `json:"inputMessageType"`
	SendThinkingToUser        Optional[bool]                                `json:"sendThinkingToUser,omitzero"`
	SendMessagesToUser        Optional[bool]                                `json:"sendMessagesToUser,omitzero"`
	McpConnections            []NewMcpConnectionAgentAssociation            `json:"mcpConnections"`
	Particles                 []NewParticleAgentAssociation                 `json:"particles"`
	MemoryInteractionProfiles []NewMemoryInteractionProfileAgentAssociation `json:"memoryInteractionProfiles"`
	ClientTasks               []NewClientTaskAgentAssociation               `json:"clientTasks"`
	Skills                    []NewSkillAgentAssociation                    `json:"skills"`
}

type UpdateAgent struct {
	Name                      Optional[string]                                        `json:"name,omitzero"`
	Description               Optional[string]                                        `json:"description,omitzero"`
	Tags                      Optional[[]*Tag]                                        `json:"tags,omitzero"`
	Identity                  Optional[string]                                        `json:"identity,omitzero"`
	TaskInstructions          Optional[string]                                        `json:"taskInstructions,omitzero"`
	ModelComplexity           Optional[ModelComplexity]                               `json:"modelComplexity,omitzero"`
	InputMessageType          Optional[string]                                        `json:"inputMessageType,omitzero"`
	SendThinkingToUser        Optional[bool]                                          `json:"sendThinkingToUser,omitzero"`
	SendMessagesToUser        Optional[bool]                                          `json:"sendMessagesToUser,omitzero"`
	McpConnections            Optional[[]NewMcpConnectionAgentAssociation]            `json:"mcpConnections,omitzero"`
	Particles                 Optional[[]NewParticleAgentAssociation]                 `json:"particles,omitzero"`
	MemoryInteractionProfiles Optional[[]NewMemoryInteractionProfileAgentAssociation] `json:"memoryInteractionProfiles,omitzero"`
	ClientTasks               Optional[[]NewClientTaskAgentAssociation]               `json:"clientTasks,omitzero"`
	Skills                    Optional[[]NewSkillAgentAssociation]                    `json:"skills,omitzero"`
}

type NewMcpConnectionAgentAssociation struct {
	// The ID of the MCP connection to associate with the agent.
	McpConnectionID string `json:"mcpConnectionID"`
	// The filter type to apply to the MCP connection.
	//
	// NONE: All tools from the MCP connection are available to the agent.
	// LIST: Only the tools specified in toolNames are available to the agent.
	// PATTERN: Only tools matching the toolNamePattern regex are available to the agent.
	FilterType ToolFilterType `json:"filterType"`
	// The list of tool names to allow the agent to use. Only applicable if filterType is LIST. Required to have at least one entry if filterType is LIST.
	ToolNames []string `json:"toolNames"`
	// A regex pattern to match tool names against. Only applicable if filterType is PATTERN. Required to be a valid regex if filterType is PATTERN.
	ToolNamePattern *string `json:"toolNamePattern,omitempty"`
}

type NewClientTaskAgentAssociation struct {
	// The ID of the task definition to expose to this agent version as a tool.
	ClientTaskDefinitionID string `json:"clientTaskDefinitionID"`
}

// NewSkillAgentAssociation gives one agent version a skill to load.
type NewSkillAgentAssociation struct {
	SkillID string `json:"skillID"`
}

// NewMemoryInteractionProfileAgentAssociation gives one agent version a memory
// interaction profile.
type NewMemoryInteractionProfileAgentAssociation struct {
	MemoryInteractionProfileID string `json:"memoryInteractionProfileID"`
}

type NewModelProvider struct {
	Name           string                   `json:"name"`
	Description    Optional[string]         `json:"description,omitzero"`
	ProviderType   ModelProviderType        `json:"providerType"`
	ProviderConfig ModelProviderConfigInput `json:"providerConfig"`
	Tags           []*Tag                   `json:"tags"`
	// Left out, the server doesn't probe the credentials.
	Probe *bool `json:"probe,omitempty"`
}

type UpdateModelProvider struct {
	Name        Optional[string] `json:"name,omitzero"`
	Description Optional[string] `json:"description,omitzero"`
	// Left out, the server keeps the stored config and its secret.
	ProviderConfig *ModelProviderConfigInput `json:"providerConfig,omitempty"`
	Tags           Optional[[]*Tag]          `json:"tags,omitzero"`
	// Left out, the server doesn't probe new credentials.
	Probe *bool `json:"probe,omitempty"`
}

type ModelProviderConfigInput struct {
	Anthropic        *ModelProviderConfigInputAnthropic        `json:"anthropic,omitempty"`
	OpenAI           *ModelProviderConfigInputOpenAI           `json:"openAI,omitempty"`
	GoogleGenAI      *ModelProviderConfigInputGoogleGenAI      `json:"googleGenAI,omitempty"`
	OpenAICompatible *ModelProviderConfigInputOpenAICompatible `json:"openAICompatible,omitempty"`
	Fireworks        *ModelProviderConfigInputFireworks        `json:"fireworks,omitempty"`
	Cerebras         *ModelProviderConfigInputCerebras         `json:"cerebras,omitempty"`
}

type ModelProviderConfigInputAnthropic struct {
	ApiKey string `json:"apiKey"`
}

type ModelProviderConfigInputOpenAI struct {
	ApiKey string `json:"apiKey"`
}

type ModelProviderConfigInputGoogleGenAI struct {
	Gemini *ModelProviderConfigInputGoogleGenAIGeminiApi `json:"gemini,omitempty"`
}

type ModelProviderConfigInputGoogleGenAIGeminiApi struct {
	ApiKey string `json:"apiKey"`
}

type ModelProviderConfigInputOpenAICompatible struct {
	Url      string           `json:"url"`
	ApiToken Optional[string] `json:"apiToken,omitzero"`
}

type ModelProviderConfigInputFireworks struct {
	ApiKey string `json:"apiKey"`
	// When set, the model list includes this Fireworks account's models.
	AccountID Optional[string] `json:"accountID,omitzero"`
}

type ModelProviderConfigInputCerebras struct {
	ApiKey string `json:"apiKey"`
}

type NewModelProfile struct {
	Name          string                  `json:"name"`
	Description   Optional[string]        `json:"description,omitzero"`
	Configuration ModelProfileConfigInput `json:"configuration"`
	Tags          []*Tag                  `json:"tags"`
	// Probe asks the server to send one request to the provider before the
	// write and refuse a profile the provider rejects. Omitted, the server
	// doesn't probe.
	Probe *bool `json:"probe,omitempty"`
}

type UpdateModelProfile struct {
	Name          Optional[string]         `json:"name,omitzero"`
	Description   Optional[string]         `json:"description,omitzero"`
	Configuration *ModelProfileConfigInput `json:"configuration,omitempty"`
	Tags          Optional[[]*Tag]         `json:"tags,omitzero"`
	// Probe asks the server to send one request to the provider before the
	// write and refuse a profile the provider rejects. Omitted, the server
	// doesn't probe.
	Probe *bool `json:"probe,omitempty"`
}

// ModelProfileConfigInput is one set of settings for whatever model the profile
// names.
//
// There is no longer a shape per vendor. Which of these a model actually accepts
// is reported by the provider, as ModelInfo.supportedParameters, rather than being
// fixed by the vendor's type.
type ModelProfileConfigInput struct {
	Model                string            `json:"model"`
	MaxTokens            Optional[int64]   `json:"maxTokens,omitzero"`
	Temperature          Optional[float64] `json:"temperature,omitzero"`
	TopP                 Optional[float64] `json:"topP,omitzero"`
	TopK                 Optional[int64]   `json:"topK,omitzero"`
	ThinkingBudgetTokens Optional[int64]   `json:"thinkingBudgetTokens,omitzero"`
	ReasoningEffort      Optional[string]  `json:"reasoningEffort,omitzero"`
	ThinkingType         Optional[string]  `json:"thinkingType,omitzero"`
	ThinkingDisplay      Optional[string]  `json:"thinkingDisplay,omitzero"`
}

type ModelSetRuleInput struct {
	ModelProfileID string           `json:"modelProfileID"`
	Complexity     Optional[string] `json:"complexity,omitzero"`
	MatchTags      Optional[[]*Tag] `json:"matchTags,omitzero"`
}

type NewModelSet struct {
	Name                  string              `json:"name"`
	Description           Optional[string]    `json:"description,omitzero"`
	Tags                  []*Tag              `json:"tags"`
	DefaultModelProfileID string              `json:"defaultModelProfileID"`
	Rules                 []ModelSetRuleInput `json:"rules"`
}

type UpdateModelSet struct {
	Name                  Optional[string]              `json:"name,omitzero"`
	Description           Optional[string]              `json:"description,omitzero"`
	Tags                  Optional[[]*Tag]              `json:"tags,omitzero"`
	DefaultModelProfileID Optional[string]              `json:"defaultModelProfileID,omitzero"`
	Rules                 Optional[[]ModelSetRuleInput] `json:"rules,omitzero"`
}

type NewMesh struct {
	Name          string                       `json:"name"`
	Description   Optional[string]             `json:"description,omitzero"`
	Tags          []*Tag                       `json:"tags"`
	ModelSetID    string                       `json:"modelSetID"`
	MemoryStoreID Optional[string]             `json:"memoryStoreID,omitzero"`
	Boundaries    []NewBoundaryMeshAssociation `json:"boundaries,omitempty"`
}

type UpdateMesh struct {
	Name        Optional[string] `json:"name,omitzero"`
	Description Optional[string] `json:"description,omitzero"`
	Tags        Optional[[]*Tag] `json:"tags,omitzero"`
	ModelSetID  Optional[string] `json:"modelSetID,omitzero"`
	// A null detaches the mesh's memory store. Left out, the mesh keeps it.
	MemoryStoreID Optional[string]                       `json:"memoryStoreID,omitzero"`
	Boundaries    Optional[[]NewBoundaryMeshAssociation] `json:"boundaries,omitzero"`
}

type NewLink struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	FromAgentID        string            `json:"fromAgentID"`
	FromAgentVersionID Optional[string]  `json:"fromAgentVersionID,omitzero"`
	ToAgentID          string            `json:"toAgentID"`
	ToAgentVersionID   Optional[string]  `json:"toAgentVersionID,omitzero"`
	TransfersControl   bool              `json:"transfersControl"`
	Responses          []NewLinkResponse `json:"responses"`
}

type UpdateLink struct {
	Name               Optional[string]            `json:"name,omitzero"`
	Description        Optional[string]            `json:"description,omitzero"`
	TransfersControl   Optional[bool]              `json:"transfersControl,omitzero"`
	FromAgentVersionID Optional[string]            `json:"fromAgentVersionID,omitzero"`
	ToAgentVersionID   Optional[string]            `json:"toAgentVersionID,omitzero"`
	Responses          Optional[[]NewLinkResponse] `json:"responses,omitzero"`
}

type NewLinkResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MessageType string `json:"messageType"`
}

type NewInput struct {
	Name      string             `json:"name"`
	AgentID   string             `json:"agentID"`
	Responses []NewInputResponse `json:"responses"`
}

type UpdateInput struct {
	Name           Optional[string]             `json:"name,omitzero"`
	AgentVersionID Optional[string]             `json:"agentVersionID,omitzero"`
	Responses      Optional[[]NewInputResponse] `json:"responses,omitzero"`
}

type NewInputResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MessageType string `json:"messageType"`
}

type NewWebhookDestination struct {
	Name             string           `json:"name"`
	Description      Optional[string] `json:"description,omitzero"`
	MeshID           string           `json:"meshID"`
	MeshVersionLabel Optional[string] `json:"meshVersionLabel,omitzero"`
	InputName        string           `json:"inputName"`
}

type UpdateWebhookDestination struct {
	Name             Optional[string] `json:"name,omitzero"`
	Description      Optional[string] `json:"description,omitzero"`
	MeshID           Optional[string] `json:"meshID,omitzero"`
	MeshVersionLabel Optional[string] `json:"meshVersionLabel,omitzero"`
	InputName        Optional[string] `json:"inputName,omitzero"`
}

type NewClientTaskDefinition struct {
	// Tool name exposed to the LLM. Must be unique across task definitions.
	Name        string `json:"name"`
	Description string `json:"description"`
	// A JSON schema the agent must satisfy when invoking the task.
	InputMessageType string `json:"inputMessageType"`
	// A JSON schema the user must satisfy when completing the task.
	OutputMessageType string `json:"outputMessageType"`
	// UIs a client can show to complete the task. At most one of each type.
	UIForms []ClientTaskUIFormInput `json:"uiForms,omitempty"`
	// Overwrite a definition that already has the name instead of failing.
	OverwriteExisting *bool `json:"overwriteExisting,omitempty"`
}

type UpdateClientTaskDefinition struct {
	Name              Optional[string] `json:"name,omitzero"`
	Description       Optional[string] `json:"description,omitzero"`
	InputMessageType  Optional[string] `json:"inputMessageType,omitzero"`
	OutputMessageType Optional[string] `json:"outputMessageType,omitzero"`
	// Replaces all of the task's UI forms when set.
	UIForms Optional[[]ClientTaskUIFormInput] `json:"uiForms,omitzero"`
}

// ClientTaskUIFormInput is one UI a client can show to complete a task.
type ClientTaskUIFormInput struct {
	Type       ClientTaskUIFormType `json:"type"`
	Definition string               `json:"definition"`
}

// NewSkill creates a skill from the files of a skill folder.
type NewSkill struct {
	// Name is the skill's own name, unique across skills.
	Name   string      `json:"name"`
	Source SkillSource `json:"source"`
}

// UpdateSkill renames a skill, replaces its files, or both.
type UpdateSkill struct {
	Name   Optional[string]      `json:"name,omitzero"`
	Source Optional[SkillSource] `json:"source,omitzero"`
}

// SkillSource is a skill folder given as its files. The provider never sends
// a zip, so the archive form of the API is left out.
type SkillSource struct {
	Files []SkillSourceFile `json:"files"`
}

// SkillSourceFile is one file of a skill folder, given as text.
type SkillSourceFile struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type NewParticleAgentAssociation struct {
	InstalledParticleID string         `json:"installedParticleID"`
	FilterType          ToolFilterType `json:"filterType"`
	ToolNames           []string       `json:"toolNames"`
	ToolNamePattern     *string        `json:"toolNamePattern,omitempty"`
}

type CreateInstalledParticleInput struct {
	Name        string                    `json:"name"`
	ArtifactID  string                    `json:"artifactID"`
	Credentials []ParticleCredentialInput `json:"credentials"`
	// KvWorkspaceID is required when the artifact's manifest grants the KV
	// capability, and rejected when it does not.
	KvWorkspaceID Optional[string] `json:"kvWorkspaceID,omitzero"`
	// Mounts is a required non-null list in the API. An install of an artifact
	// that declares no filesystem mounts sends it empty; sending nothing at all
	// is a validation error.
	Mounts []NewInstalledParticleMount `json:"mounts"`
}

// NewInstalledParticleMount binds one manifest mount name to a workspace.
type NewInstalledParticleMount struct {
	MountName   string `json:"mountName"`
	WorkspaceID string `json:"workspaceID"`
}

type UpdateInstalledParticleInput struct {
	InstalledParticleID string                              `json:"installedParticleID"`
	Name                Optional[string]                    `json:"name,omitzero"`
	ArtifactID          Optional[string]                    `json:"artifactID,omitzero"`
	Credentials         Optional[[]ParticleCredentialInput] `json:"credentials,omitzero"`
	// KvWorkspaceID repoints the install's KV state. Omitting it leaves the
	// existing binding alone.
	KvWorkspaceID Optional[string] `json:"kvWorkspaceID,omitzero"`
	// Mounts replaces the bindings wholesale rather than merging. Omitting it
	// leaves them alone; an empty list clears them.
	Mounts Optional[[]NewInstalledParticleMount] `json:"mounts,omitzero"`
}

type ParticleCredentialInput struct {
	CredentialName          string                                          `json:"credentialName"`
	Method                  string                                          `json:"method"`
	Basic                   *BasicParticleCredentialInput                   `json:"basic,omitempty"`
	Apikey                  *ApiKeyParticleCredentialInput                  `json:"apikey,omitempty"`
	SigningKey              *SigningKeyParticleCredentialInput              `json:"signingKey,omitempty"`
	Raw                     *RawParticleCredentialInput                     `json:"raw,omitempty"`
	OAuth2ClientCredentials *OAuth2ClientCredentialsParticleCredentialInput `json:"oauth2ClientCredentials,omitempty"`
}

type BasicParticleCredentialInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ApiKeyParticleCredentialInput struct {
	Key string `json:"key"`
}

type SigningKeyParticleCredentialInput struct {
	Algorithm string `json:"algorithm"`
	Key       string `json:"key"`
}

type RawParticleCredentialInput struct {
	Value string `json:"value"`
}

type OAuth2ClientCredentialsParticleCredentialInput struct {
	ClientID     string  `json:"clientID"`
	ClientSecret string  `json:"clientSecret"`
	TokenUrl     *string `json:"tokenUrl,omitempty"`
}

// EmbeddingProfileConfigInput is one set of settings for whatever model the
// profile names. As with model profiles, there is no longer a shape per vendor.
type EmbeddingProfileConfigInput struct {
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	// SendDimensions asks the service for that size with each request. Set it for
	// a model that can be asked for fewer dimensions than it produces.
	SendDimensions Optional[bool] `json:"sendDimensions,omitzero"`
	// TaskType is what the embedding is for, such as RETRIEVAL_DOCUMENT. Google
	// only.
	TaskType Optional[string] `json:"taskType,omitzero"`
}

type NewEmbeddingProfile struct {
	Name          string                      `json:"name"`
	Description   Optional[string]            `json:"description,omitzero"`
	Configuration EmbeddingProfileConfigInput `json:"configuration"`
	// Probe asks the server to send one request to the provider before the
	// write and refuse a profile the provider rejects. Omitted, the server
	// doesn't probe.
	Probe *bool `json:"probe,omitempty"`
}

type UpdateEmbeddingProfile struct {
	Name          Optional[string]             `json:"name,omitzero"`
	Description   Optional[string]             `json:"description,omitzero"`
	Configuration *EmbeddingProfileConfigInput `json:"configuration,omitempty"`
	// Probe asks the server to send one request to the provider before the
	// write and refuse a profile the provider rejects. Omitted, the server
	// doesn't probe.
	Probe *bool `json:"probe,omitempty"`
}

// NewMemoryStoreScope is one scope written with its store, or added to one.
type NewMemoryStoreScope struct {
	Description                 Optional[string] `json:"description,omitzero"`
	Kind                        string           `json:"kind"`
	Key                         string           `json:"key"`
	Value                       Optional[string] `json:"value,omitzero"`
	Match                       Optional[string] `json:"match,omitzero"`
	ExtractionEnabled           bool             `json:"extractionEnabled"`
	ExtractionPrompt            Optional[string] `json:"extractionPrompt,omitzero"`
	ExtractionModelProfileID    Optional[string] `json:"extractionModelProfileID,omitzero"`
	ConsolidationEnabled        bool             `json:"consolidationEnabled"`
	ConsolidationPrompt         Optional[string] `json:"consolidationPrompt,omitzero"`
	ConsolidationModelProfileID Optional[string] `json:"consolidationModelProfileID,omitzero"`
}

// UpdateMemoryStoreScope changes one scope. The key, the kind and the value of
// a scope never change, because the records it already produced carry them.
type UpdateMemoryStoreScope struct {
	Description                 Optional[string] `json:"description,omitzero"`
	Match                       Optional[string] `json:"match,omitzero"`
	ExtractionEnabled           Optional[bool]   `json:"extractionEnabled,omitzero"`
	ExtractionPrompt            Optional[string] `json:"extractionPrompt,omitzero"`
	ExtractionModelProfileID    Optional[string] `json:"extractionModelProfileID,omitzero"`
	ConsolidationEnabled        Optional[bool]   `json:"consolidationEnabled,omitzero"`
	ConsolidationPrompt         Optional[string] `json:"consolidationPrompt,omitzero"`
	ConsolidationModelProfileID Optional[string] `json:"consolidationModelProfileID,omitzero"`
}

// MemoryStoreScopeEdit names one scope of a store and what to change about it.
type MemoryStoreScopeEdit struct {
	ID      string                  `json:"id"`
	Changes *UpdateMemoryStoreScope `json:"changes"`
}

type NewMemoryStore struct {
	Name                       string                 `json:"name"`
	Description                Optional[string]       `json:"description,omitzero"`
	IngestConversations        bool                   `json:"ingestConversations"`
	IndexConversationMessages  bool                   `json:"indexConversationMessages"`
	EmbeddingProfileID         Optional[string]       `json:"embeddingProfileID,omitzero"`
	UserNameTagKey             Optional[string]       `json:"userNameTagKey,omitzero"`
	ExtractIntervalSeconds     int                    `json:"extractIntervalSeconds"`
	ConsolidateIntervalSeconds int                    `json:"consolidateIntervalSeconds"`
	Scopes                     []*NewMemoryStoreScope `json:"scopes,omitempty"`
}

type UpdateMemoryStore struct {
	Name                       Optional[string]        `json:"name,omitzero"`
	Description                Optional[string]        `json:"description,omitzero"`
	IngestConversations        Optional[bool]          `json:"ingestConversations,omitzero"`
	IndexConversationMessages  Optional[bool]          `json:"indexConversationMessages,omitzero"`
	EmbeddingProfileID         Optional[string]        `json:"embeddingProfileID,omitzero"`
	UserNameTagKey             Optional[string]        `json:"userNameTagKey,omitzero"`
	ExtractIntervalSeconds     Optional[int]           `json:"extractIntervalSeconds,omitzero"`
	ConsolidateIntervalSeconds Optional[int]           `json:"consolidateIntervalSeconds,omitzero"`
	DeleteScopeIDs             []string                `json:"deleteScopeIDs,omitempty"`
	UpdateScopes               []*MemoryStoreScopeEdit `json:"updateScopes,omitempty"`
	CreateScopes               []*NewMemoryStoreScope  `json:"createScopes,omitempty"`
}

// MemoryQuerySourceInput is the text one injection or history searches for.
type MemoryQuerySourceInput struct {
	Kind string           `json:"kind"`
	Text Optional[string] `json:"text,omitzero"`
}

// MemorySelectionTagInput maps one tag of a request onto one tag of a memory.
type MemorySelectionTagInput struct {
	Source        string           `json:"source"`
	TagKey        string           `json:"tagKey"`
	TagValue      Optional[string] `json:"tagValue,omitzero"`
	RequestTagKey Optional[string] `json:"requestTagKey,omitzero"`
}

// MemorySelectionInput narrows the memory that one injection or tool reads.
type MemorySelectionInput struct {
	Kind string                     `json:"kind"`
	Tags []*MemorySelectionTagInput `json:"tags,omitempty"`
}

type MemoryConversationHistoryInput struct {
	TokenBudget int                     `json:"tokenBudget"`
	Selection   string                  `json:"selection"`
	Query       *MemoryQuerySourceInput `json:"query,omitempty"`
}

type MemoryContextInjectionInput struct {
	Name        string                  `json:"name"`
	Description Optional[string]        `json:"description,omitzero"`
	TokenBudget int                     `json:"tokenBudget"`
	RecordTypes []string                `json:"recordTypes"`
	Selection   *MemorySelectionInput   `json:"selection"`
	Query       *MemoryQuerySourceInput `json:"query"`
}

type MemoryEffortLevelInput struct {
	Level       string `json:"level"`
	TokenBudget int    `json:"tokenBudget"`
}

type MemoryQueryToolInput struct {
	ToolName     string                    `json:"toolName"`
	Description  string                    `json:"description"`
	RecordTypes  []string                  `json:"recordTypes"`
	Selection    *MemorySelectionInput     `json:"selection"`
	TokenBudget  Optional[int]             `json:"tokenBudget,omitzero"`
	EffortLevels []*MemoryEffortLevelInput `json:"effortLevels,omitempty"`
}

type MemoryTagMappingInput struct {
	Kind     string           `json:"kind"`
	Keys     []string         `json:"keys,omitempty"`
	TagKey   Optional[string] `json:"tagKey,omitzero"`
	TagValue Optional[string] `json:"tagValue,omitzero"`
}

type MemoryWriteToolInput struct {
	ToolName    string                   `json:"toolName"`
	Description string                   `json:"description"`
	TagMappings []*MemoryTagMappingInput `json:"tagMappings,omitempty"`
}

type NewMemoryInteractionProfile struct {
	Name                string                          `json:"name"`
	Description         Optional[string]                `json:"description,omitzero"`
	ConversationHistory *MemoryConversationHistoryInput `json:"conversationHistory,omitempty"`
	ContextInjections   []*MemoryContextInjectionInput  `json:"contextInjections,omitempty"`
	QueryTools          []*MemoryQueryToolInput         `json:"queryTools,omitempty"`
	WriteTools          []*MemoryWriteToolInput         `json:"writeTools,omitempty"`
	AllowExpansion      bool                            `json:"allowExpansion"`
}

// UpdateMemoryInteractionProfile changes one profile. Each list is optional so
// that a list left out keeps what the profile held, and an empty list given
// empties it.
type UpdateMemoryInteractionProfile struct {
	Name                Optional[string]                          `json:"name,omitzero"`
	Description         Optional[string]                          `json:"description,omitzero"`
	ConversationHistory Optional[*MemoryConversationHistoryInput] `json:"conversationHistory,omitzero"`
	ContextInjections   Optional[[]*MemoryContextInjectionInput]  `json:"contextInjections,omitzero"`
	QueryTools          Optional[[]*MemoryQueryToolInput]         `json:"queryTools,omitzero"`
	WriteTools          Optional[[]*MemoryWriteToolInput]         `json:"writeTools,omitzero"`
	AllowExpansion      Optional[bool]                            `json:"allowExpansion,omitzero"`
}

// The evaluation update inputs are written by hand rather than generated so a
// null reaches the API instead of being left out. The API leaves alone what it
// is not told about, which is right for a patch but means a generated input,
// where a nil pointer is omitted, could never clear a field. Terraform needs
// to: a description removed from the configuration has to be removed from the
// resource.

type UpdateJudgeMetric struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	Category    *MetricCategory      `json:"category"`
	Scope       *JudgeScope          `json:"scope"`
	Prompt      *string              `json:"prompt"`
	Complexity  *ModelComplexity     `json:"complexity"`
	Examples    []NewScoreExample    `json:"examples"`
	InputFields []NewJudgeInputField `json:"inputFields"`
}

// UpdateRubricMetric keeps omitempty, unlike the inputs above: the API rejects
// the scoring fields outright on a metric a model judges, so a field that does
// not apply has to be left out rather than sent as null.
type UpdateRubricMetric struct {
	ID            string           `json:"ID"`
	TrialCount    *int             `json:"trialCount,omitempty"`
	Complexity    *ModelComplexity `json:"complexity,omitempty"`
	InputValues   *json.RawMessage `json:"inputValues,omitempty"`
	ScoreFloor    *float64         `json:"scoreFloor,omitempty"`
	ScoreCeiling  *float64         `json:"scoreCeiling,omitempty"`
	LowerIsBetter *bool            `json:"lowerIsBetter,omitempty"`
	Weight        *float64         `json:"weight,omitempty"`
}

type UpdateRubric struct {
	Name             *string              `json:"name"`
	Description      *string              `json:"description"`
	DeletedMetricIDs []string             `json:"deletedMetricIDs"`
	UpdatedMetrics   []UpdateRubricMetric `json:"updatedMetrics"`
	NewMetrics       []NewRubricMetric    `json:"newMetrics"`
}

// UpdateRoute and McpConnectionSubscriptionChanges are written by hand so a
// list can be left out. The API leaves a list alone when it is left out and
// clears it when it is null; a generated input can only send null.

type UpdateRoute struct {
	Name        *string                     `json:"name,omitempty"`
	Description *string                     `json:"description,omitempty"`
	Segments    Optional[[]NewRouteSegment] `json:"segments,omitzero"`
}

type McpConnectionSubscriptionChanges struct {
	Name            *string                                             `json:"name,omitempty"`
	Criteria        Optional[[]McpConnectionSubscriptionCriterionInput] `json:"criteria,omitzero"`
	AddTunnelKeyIDs []string                                            `json:"addTunnelKeyIDs,omitempty"`
}

type NewBoundary struct {
	Name          string                      `json:"name"`
	Description   *string                     `json:"description,omitempty"`
	ReportOnly    bool                        `json:"reportOnly"`
	Rules         []BoundaryRuleInput         `json:"rules"`
	Modifications []BoundaryModificationInput `json:"modifications"`
}

type UpdateBoundary struct {
	Name          Optional[string]                      `json:"name,omitzero"`
	Description   Optional[string]                      `json:"description,omitzero"`
	ReportOnly    Optional[bool]                        `json:"reportOnly,omitzero"`
	Rules         Optional[[]BoundaryRuleInput]         `json:"rules,omitzero"`
	Modifications Optional[[]BoundaryModificationInput] `json:"modifications,omitzero"`
}

type NewBoundaryMeshAssociation struct {
	BoundaryID string `json:"boundaryID"`
}

type NewBoundaryAccessPointAssociation struct {
	BoundaryID string `json:"boundaryID"`
}

type AccessPointIssuerInput struct {
	Issuer string `json:"issuer"`
	// Left out, it is the access point's URL. A null turns off the audience check.
	Audience Optional[string] `json:"audience,omitzero"`
	JwksURL  *string          `json:"jwksUrl,omitempty"`
	JWKS     json.RawMessage  `json:"jwks,omitempty"`
}

type NewAccessPoint struct {
	Name                       string                              `json:"name"`
	Description                *string                             `json:"description,omitempty"`
	Tags                       []*Tag                              `json:"tags"`
	AuthorizationServerURL     *string                             `json:"authorizationServerUrl,omitempty"`
	ClientTokenLifetimeSeconds *int                                `json:"clientTokenLifetimeSeconds,omitempty"`
	Issuers                    []AccessPointIssuerInput            `json:"issuers"`
	Boundaries                 []NewBoundaryAccessPointAssociation `json:"boundaries"`
}

type UpdateAccessPoint struct {
	Name                       Optional[string]                              `json:"name,omitzero"`
	Description                Optional[string]                              `json:"description,omitzero"`
	Tags                       Optional[[]*Tag]                              `json:"tags,omitzero"`
	AuthorizationServerURL     Optional[string]                              `json:"authorizationServerUrl,omitzero"`
	ClientTokenLifetimeSeconds Optional[int]                                 `json:"clientTokenLifetimeSeconds,omitzero"`
	Issuers                    Optional[[]AccessPointIssuerInput]            `json:"issuers,omitzero"`
	Boundaries                 Optional[[]NewBoundaryAccessPointAssociation] `json:"boundaries,omitzero"`
}

type UpdateAccessPointClient struct {
	Name   Optional[string]          `json:"name,omitzero"`
	Claims Optional[json.RawMessage] `json:"claims,omitzero"`
	// A null removes them all.
	RedirectUris Optional[[]string] `json:"redirectUris,omitzero"`
}

type BoundaryRuleInput struct {
	Name       string         `json:"name"`
	Action     BoundaryAction `json:"action"`
	When       *string        `json:"when,omitempty"`
	Effect     BoundaryEffect `json:"effect"`
	Message    *string        `json:"message,omitempty"`
	ReportOnly bool           `json:"reportOnly"`
}

type BoundaryModificationInput struct {
	Name         string                         `json:"name"`
	Action       BoundaryAction                 `json:"action"`
	When         *string                        `json:"when,omitempty"`
	SetTags      []BoundaryNamedExpressionInput `json:"setTags"`
	SetTracing   *string                        `json:"setTracing,omitempty"`
	SetArguments []BoundaryNamedExpressionInput `json:"setArguments"`
}

type BoundaryNamedExpressionInput struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}
