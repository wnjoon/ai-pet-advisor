package adapter

// IncomingMessage is a platform-agnostic representation of a user message.
type IncomingMessage struct {
	UserID      string
	Platform    string // "kakao", "telegram", "web"
	Text        string
	CallbackURL string                 // KakaoTalk callback URL (optional)
	Extra       map[string]any // Platform-specific data
}

// AgentResponse is the AI agent's response to be formatted per platform.
type AgentResponse struct {
	Text        string   // Main response text
	UrgencyLevel int     // 1-4
	WebLinks    []string // Optional web links (e.g. hospital search)
}

// PlatformAdapter defines the interface for platform-specific message handling.
type PlatformAdapter interface {
	ParseRequest(data []byte) (*IncomingMessage, error)
	FormatResponse(resp AgentResponse) (any, error)
	SendCallback(callbackURL string, response any) error
}
