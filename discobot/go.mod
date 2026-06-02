module github.com/obot-platform/discobot/discobot

go 1.26

require (
	github.com/go-chi/chi/v5 v5.2.5
	github.com/obot-platform/discobot v0.0.0
	github.com/obot-platform/discobot/agent-go v0.0.0
)

replace github.com/obot-platform/discobot => ..

replace github.com/obot-platform/discobot/agent-go => ../agent-go

require github.com/coder/websocket v1.8.14 // indirect
