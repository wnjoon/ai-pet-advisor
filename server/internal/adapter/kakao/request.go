package kakao

// KakaoRequest represents the top-level request from KakaoTalk OpenBuilder skill server.
type KakaoRequest struct {
	Intent      KakaoIntent      `json:"intent"`
	UserRequest KakaoUserRequest `json:"userRequest"`
	Bot         KakaoBot         `json:"bot"`
	Action      KakaoAction      `json:"action"`
}

// KakaoIntent represents the intent block in the request.
type KakaoIntent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// KakaoUserRequest represents the user request data.
type KakaoUserRequest struct {
	Timezone    string      `json:"timezone"`
	Block       KakaoBlock  `json:"block"`
	Utterance   string      `json:"utterance"`
	Lang        string      `json:"lang"`
	User        KakaoUser   `json:"user"`
	CallbackURL string      `json:"callbackUrl"`
}

// KakaoBlock represents the block information.
type KakaoBlock struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// KakaoUser represents the user information.
type KakaoUser struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
}

// KakaoBot represents the bot information.
type KakaoBot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// KakaoAction represents the action block in the request.
type KakaoAction struct {
	Name         string            `json:"name"`
	ClientExtra  map[string]string `json:"clientExtra"`
	Params       map[string]string `json:"params"`
	ID           string            `json:"id"`
	DetailParams map[string]any    `json:"detailParams"`
}
