<claude-mem-context>
# Memory Context

# [spider.ai] recent context, 2026-05-15 9:17pm GMT+8

Legend: 🎯session 🔴bugfix 🟣feature 🔄refactor ✅change 🔵discovery ⚖️decision
Format: ID TIME TYPE TITLE
Fetch details: get_observations([IDs]) | Search: mem-search skill

### May 15, 2026
4923 8:01p 🔵 spider.ai — OpenAIClient.buildMessages uses flat string content field
4924 8:02p 🔵 spider.ai — LLM package tests pass
4925 8:03p 🔵 spider.ai — llm and agent package tests pass clean
4926 " 🔵 spider.ai server port 8002 already in use on startup
S2725 spider.ai — 用户请求分析配置文件整体逻辑 (May 15 at 8:10 PM)
4927 8:11p 🔵 spider.ai — agent.go tool input parsing code path confirmed
4929 " 🔵 spider.ai — agent.go streaming event loop full structure confirmed
4931 " 🔵 spider.ai — compactor.go buildAssistantMessage and parseToolResultBlock confirmed
4933 " 🟣 spider.ai — LLM retry logic with exponential backoff added to agent.go
4935 8:26p 🔵 spider.ai — project config files and Claude permissions confirmed
4936 8:27p 🔵 spider.ai — API config layer structure confirmed
4937 " 🔵 spider.ai — 用户请求分析配置文件整体逻辑
4938 8:28p 🔵 spider.ai — 用户请求分析配置文件整体逻辑
S2729 Fix ContentBlock JSON serialization — Claude API wire format correctness for tool_use/tool_result/text blocks (May 15 at 8:28 PM)
4939 8:29p 🟣 ContentBlock.MarshalJSON added to llm/client.go
4940 " 🔵 spider.ai server already running on :8002
S2730 spider.ai — codex review triggered against main branch (May 15 at 8:29 PM)
S2727 Fix ContentBlock JSON serialization for Claude API wire format correctness (May 15 at 8:29 PM)
4942 8:35p 🔵 spider.ai — codex review triggered against main branch
S2731 isTransientLLMError prefix matching refactored to range loop — easier to extend (May 15 at 8:35 PM)
4943 8:37p 🔵 Codex review identifies two functional regressions in tool-result rendering and OpenAI retry logic
4944 " 🔵 ChatView.vue toolIndex fallback creates orphaned map — tool_result lookup always fails
4945 8:38p 🔴 ChatView.vue toolIndex map now persisted on message object — live tool results render correctly
4946 " 🔴 isTransientLLMError now recognizes OpenAI error format — transient OpenAI errors retried
4947 8:39p ✅ Frontend build passes after toolIndex and retry classifier fixes
4948 " 🔄 isTransientLLMError prefix matching refactored to range loop — easier to extend
S2733 spider.ai 全修 session — full diff summary of 17 changed files (May 15 at 8:39 PM)
4949 8:43p 🔵 spider.ai app.go — no ConfigPath field in App struct
4950 8:44p 🔵 spider.ai MCP package structure confirmed
4951 8:46p 🔵 spider.ai MCP App struct — full dependency inventory confirmed
4952 " ✅ config.DefaultConfigPath exported; App struct gains ConfigPath field
4953 8:47p 🔄 settings.go saveConfig uses App.ConfigPath instead of DataDir-derived path
4954 " 🔴 BaseURL derivation fixed for wildcard listen addresses
4955 " 🔴 saveConfig writes to App.ConfigPath instead of DataDir/config.yaml
4956 " 🔴 App.ConfigPath wired at startup in main.go serve()
4957 8:48p 🟣 Settings API exposes MaxTurns and Compaction config fields
4958 " 🟣 Settings API now reads and writes MaxTurns and Compaction config at runtime
4959 8:49p 🔴 updateProvider clears cached models when provider credentials change
4960 8:50p ✅ spider.ai 全修 session — full diff summary of 17 changed files
S2734 Code review of spider.ai config save fix branch (May 15 at 8:50 PM)
S2736 Complete host selection injection architecture: frontend to LLM system prompt (May 15 at 8:56 PM)
4961 8:58p 🟣 spider.ai — LLM retry with exponential backoff and frontend retry banner
4962 " 🔄 spider.ai — config path propagation and settings API expansion
4963 " 🔄 spider.ai — ContentBlock.MarshalJSON and OpenAI message builder for tool calls
4964 9:04p 🔵 Selected hosts injection flow: frontend to system prompt
4965 " 🔵 Complete host selection injection flow: frontend to LLM system prompt
4966 " 🔵 Host selection differs between ChatView and ExecView components
4967 " 🔵 Complete host selection injection architecture: frontend to LLM system prompt
S2738 spider.ai — 用户询问选中主机如何注入会话 (May 15 at 9:04 PM)
4969 9:08p 🔵 spider.ai — 用户询问选中主机如何注入会话
S2739 spider.ai — 选中主机注入方案对比：system prompt vs 工具层强制注入 (May 15 at 9:12 PM)
4970 9:13p 🔵 Selected host injection flow — how selectedHostIds travels from frontend to backend system prompt
4971 9:14p 🔵 Host injection flow — system prompt construction and tool execution path
4972 9:15p ✅ Agent struct extended with selectedHostIDs field
4974 " ✅ Agent.Run() calls injectSelectedHosts() before tool execution
4976 " 🟣 injectSelectedHosts() implemented — auto-fills host fields in tool input at execution time
4977 9:16p 🔄 BuildSystemPrompt() host injection removed — moved to tool layer via injectSelectedHosts()
4978 " ✅ chat.go call site updated — selectedHostIDs now passed to NewAgent() instead of BuildSystemPrompt()
4979 9:17p 🔵 spider.ai — go build ./... output requested
4980 " 🔴 spider.ai — go build fails: go-build cache permission denied
</claude-mem-context>