package executor

import (
	"encoding/json"
	"reflect"

	"github.com/tidwall/gjson"
)

type piCodexWebsocketContinuation struct {
	lastRequestBody map[string]any
	lastResponseID  string
	lastResponse    []any
}

func (s *codexWebsocketSession) buildPiContinuationBody(body []byte) []byte {
	if s == nil {
		return body
	}
	var current map[string]any
	if json.Unmarshal(body, &current) != nil {
		return body
	}

	s.piStateMu.Lock()
	defer s.piStateMu.Unlock()
	state := s.piContinuation
	if state == nil || state.lastResponseID == "" {
		return body
	}
	if !reflect.DeepEqual(piRequestWithoutInput(current), piRequestWithoutInput(state.lastRequestBody)) {
		s.piContinuation = nil
		return body
	}
	currentInput, okCurrent := current["input"].([]any)
	lastInput, okLast := state.lastRequestBody["input"].([]any)
	if !okCurrent || !okLast {
		s.piContinuation = nil
		return body
	}
	baseline := append(append([]any(nil), lastInput...), state.lastResponse...)
	if len(currentInput) < len(baseline) || !reflect.DeepEqual(currentInput[:len(baseline)], baseline) {
		s.piContinuation = nil
		return body
	}
	current["previous_response_id"] = state.lastResponseID
	current["input"] = currentInput[len(baseline):]
	encoded, errMarshal := json.Marshal(current)
	if errMarshal != nil {
		return body
	}
	return encoded
}

func (s *codexWebsocketSession) storePiContinuation(requestBody, completedPayload []byte) {
	if s == nil {
		return
	}
	var request map[string]any
	if json.Unmarshal(requestBody, &request) != nil {
		return
	}
	responseID := gjson.GetBytes(completedPayload, "response.id").String()
	if responseID == "" {
		return
	}
	var responseItems []any
	if raw := gjson.GetBytes(completedPayload, "response.output"); raw.Exists() {
		_ = json.Unmarshal([]byte(raw.Raw), &responseItems)
	}
	filtered := responseItems[:0]
	for _, item := range responseItems {
		itemMap, _ := item.(map[string]any)
		itemType, _ := itemMap["type"].(string)
		if itemType == "function_call_output" || itemType == "custom_tool_call_output" {
			continue
		}
		filtered = append(filtered, item)
	}
	s.piStateMu.Lock()
	s.piContinuation = &piCodexWebsocketContinuation{
		lastRequestBody: request,
		lastResponseID:  responseID,
		lastResponse:    append([]any(nil), filtered...),
	}
	s.piStateMu.Unlock()
}

func piRequestWithoutInput(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		if key == "input" || key == "previous_response_id" || key == "type" {
			continue
		}
		result[key] = value
	}
	return result
}
