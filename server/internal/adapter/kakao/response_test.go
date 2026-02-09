package kakao

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCallbackAck(t *testing.T) {
	resp := NewCallbackAck("잠시만 기다려주세요")

	assert.Equal(t, "2.0", resp.Version)
	assert.True(t, resp.UseCallback)
	require.NotNil(t, resp.Data)
	assert.Equal(t, "잠시만 기다려주세요", resp.Data.Text)

	// Verify JSON structure
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed["version"])
	assert.Equal(t, true, parsed["useCallback"])
	dataObj, ok := parsed["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "잠시만 기다려주세요", dataObj["text"])
}

func TestNewSimpleTextResponse(t *testing.T) {
	resp := NewSimpleTextResponse("테스트 메시지")

	assert.Equal(t, "2.0", resp.Version)
	assert.False(t, resp.UseCallback)
	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].SimpleText)
	assert.Equal(t, "테스트 메시지", resp.Template.Outputs[0].SimpleText.Text)

	// Verify JSON structure
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed["version"])
	template, ok := parsed["template"].(map[string]interface{})
	require.True(t, ok)
	outputs, ok := template["outputs"].([]interface{})
	require.True(t, ok)
	require.Len(t, outputs, 1)
	output0, ok := outputs[0].(map[string]interface{})
	require.True(t, ok)
	simpleText, ok := output0["simpleText"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "테스트 메시지", simpleText["text"])
}

func TestNewSimpleTextWithQuickReplies(t *testing.T) {
	text := "병원 방문을 권장합니다"
	replies := []QuickReply{
		{
			Label:      "동물병원 찾기",
			Action:     "webLink",
			WebLinkURL: "https://m.search.naver.com/search.naver?query=동물병원+근처",
		},
	}

	resp := NewSimpleTextWithQuickReplies(text, replies)

	assert.Equal(t, "2.0", resp.Version)
	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].SimpleText)
	assert.Equal(t, text, resp.Template.Outputs[0].SimpleText.Text)
	require.Len(t, resp.Template.QuickReplies, 1)
	assert.Equal(t, "동물병원 찾기", resp.Template.QuickReplies[0].Label)
	assert.Equal(t, "webLink", resp.Template.QuickReplies[0].Action)
	assert.Equal(t, "https://m.search.naver.com/search.naver?query=동물병원+근처", resp.Template.QuickReplies[0].WebLinkURL)

	// Verify JSON structure
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	template, ok := parsed["template"].(map[string]interface{})
	require.True(t, ok)
	quickReplies, ok := template["quickReplies"].([]interface{})
	require.True(t, ok)
	require.Len(t, quickReplies, 1)
	qr0, ok := quickReplies[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "동물병원 찾기", qr0["label"])
	assert.Equal(t, "webLink", qr0["action"])
}

func TestNewListCardResponse(t *testing.T) {
	title := "메뉴"
	items := []ListCardItem{
		{
			Title:       "반려견 등록",
			Description: "새 반려견을 등록합니다",
			Action:      "message",
			MessageText: "반려견 등록",
		},
		{
			Title:       "반려견 전환",
			Description: "다른 반려견으로 전환합니다",
			Action:      "message",
			MessageText: "반려견 전환",
		},
	}

	resp := NewListCardResponse(title, items)

	assert.Equal(t, "2.0", resp.Version)
	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].ListCard)
	assert.Equal(t, title, resp.Template.Outputs[0].ListCard.Header.Title)
	require.Len(t, resp.Template.Outputs[0].ListCard.Items, 2)
	assert.Equal(t, "반려견 등록", resp.Template.Outputs[0].ListCard.Items[0].Title)
	assert.Equal(t, "새 반려견을 등록합니다", resp.Template.Outputs[0].ListCard.Items[0].Description)

	// Verify JSON structure
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	template, ok := parsed["template"].(map[string]interface{})
	require.True(t, ok)
	outputs, ok := template["outputs"].([]interface{})
	require.True(t, ok)
	require.Len(t, outputs, 1)
	output0, ok := outputs[0].(map[string]interface{})
	require.True(t, ok)
	listCard, ok := output0["listCard"].(map[string]interface{})
	require.True(t, ok)
	header, ok := listCard["header"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, title, header["title"])
	itemsArr, ok := listCard["items"].([]interface{})
	require.True(t, ok)
	require.Len(t, itemsArr, 2)
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("오류가 발생했습니다")

	assert.Equal(t, "2.0", resp.Version)
	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].SimpleText)
	assert.Equal(t, "오류가 발생했습니다", resp.Template.Outputs[0].SimpleText.Text)

	// Verify it uses the same structure as NewSimpleTextResponse
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed["version"])
	template, ok := parsed["template"].(map[string]interface{})
	require.True(t, ok)
	outputs, ok := template["outputs"].([]interface{})
	require.True(t, ok)
	require.Len(t, outputs, 1)
}
