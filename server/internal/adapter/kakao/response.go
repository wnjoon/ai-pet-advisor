package kakao

// KakaoResponse is the top-level response structure for KakaoTalk OpenBuilder
type KakaoResponse struct {
	Version     string        `json:"version"`
	UseCallback bool          `json:"useCallback,omitempty"`
	Data        *CallbackData `json:"data,omitempty"`
	Template    *Template     `json:"template,omitempty"`
}

// CallbackData is used in immediate callback acknowledgment
type CallbackData struct {
	Text string `json:"text"`
}

// Template represents the response template
type Template struct {
	Outputs      []Output      `json:"outputs"`
	QuickReplies []QuickReply  `json:"quickReplies,omitempty"`
}

// Output represents one of the output types
type Output struct {
	SimpleText *SimpleText `json:"simpleText,omitempty"`
	ListCard   *ListCard   `json:"listCard,omitempty"`
}

// SimpleText represents a simple text output
type SimpleText struct {
	Text string `json:"text"`
}

// ListCard represents a list card output
type ListCard struct {
	Header ListCardHeader `json:"header"`
	Items  []ListCardItem `json:"items"`
}

// ListCardHeader represents the header of a list card
type ListCardHeader struct {
	Title string `json:"title"`
}

// ListCardItem represents an item in a list card
type ListCardItem struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Action      string `json:"action,omitempty"`      // "message" or "webLink"
	MessageText string `json:"messageText,omitempty"` // for action="message"
	WebLinkURL  string `json:"webLinkUrl,omitempty"`  // for action="webLink"
}

// QuickReply represents a quick reply button
type QuickReply struct {
	Label       string `json:"label"`
	Action      string `json:"action"` // "message" or "webLink"
	MessageText string `json:"messageText,omitempty"`
	WebLinkURL  string `json:"webLinkUrl,omitempty"`
}

// NewCallbackAck returns the immediate "please wait" response with useCallback=true
func NewCallbackAck(text string) *KakaoResponse {
	return &KakaoResponse{
		Version:     "2.0",
		UseCallback: true,
		Data: &CallbackData{
			Text: text,
		},
	}
}

// NewSimpleTextResponse returns a simple text response
func NewSimpleTextResponse(text string) *KakaoResponse {
	return &KakaoResponse{
		Version: "2.0",
		Template: &Template{
			Outputs: []Output{
				{
					SimpleText: &SimpleText{
						Text: text,
					},
				},
			},
		},
	}
}

// NewSimpleTextWithQuickReplies returns a simple text response with quick reply buttons
func NewSimpleTextWithQuickReplies(text string, replies []QuickReply) *KakaoResponse {
	return &KakaoResponse{
		Version: "2.0",
		Template: &Template{
			Outputs: []Output{
				{
					SimpleText: &SimpleText{
						Text: text,
					},
				},
			},
			QuickReplies: replies,
		},
	}
}

// NewListCardResponse returns a list card response
func NewListCardResponse(title string, items []ListCardItem) *KakaoResponse {
	return &KakaoResponse{
		Version: "2.0",
		Template: &Template{
			Outputs: []Output{
				{
					ListCard: &ListCard{
						Header: ListCardHeader{
							Title: title,
						},
						Items: items,
					},
				},
			},
		},
	}
}

// NewErrorResponse returns a user-friendly error message
func NewErrorResponse(msg string) *KakaoResponse {
	return NewSimpleTextResponse(msg)
}
