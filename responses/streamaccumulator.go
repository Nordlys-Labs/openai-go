package responses

// ResponseAccumulator accumulates Response streaming events into a single Response
// and exposes "just finished" events for consumers.
type ResponseAccumulator struct {
	Response

	toolCallIndices map[string]int64
	nextToolIndex   int64

	// transient state set when the last AddEvent produced a "just finished" item
	justFinishedType            string
	justFinishedText            string
	justFinishedRefusal         string
	justFinishedTool            FinishedResponseToolCall
	justFinishedReasoning       string
	justFinishedAudioTranscript string
	justFinishedFileSearch      FinishedResponseFileSearch
	justFinishedWebSearch       FinishedResponseWebSearch
	justFinishedCodeInterpreter FinishedResponseCodeInterpreter
	justFinishedError           string
}

type FinishedResponseText struct{ Text string }

type FinishedResponseToolCall struct {
	CallID string
	Index  int64
}

type FinishedResponseRefusal struct{ Refusal string }

type FinishedResponseReasoning struct{ Text string }

type FinishedResponseAudioTranscript struct{ Transcript string }

type FinishedResponseFileSearch struct{ CallID string }

type FinishedResponseWebSearch struct{ CallID string }

type FinishedResponseCodeInterpreter struct{ ItemID string }

type FinishedResponseError struct{ Err string }

func NewResponseAccumulator() *ResponseAccumulator {
	return &ResponseAccumulator{toolCallIndices: map[string]int64{}}
}

// AddEvent incorporates a streaming event. Returns false only on mismatch errors.
func (acc *ResponseAccumulator) AddEvent(event ResponseStreamEventUnion) bool {
	// clear transient justFinished state
	acc.justFinishedType = ""
	acc.justFinishedText = ""
	acc.justFinishedRefusal = ""
	acc.justFinishedTool = FinishedResponseToolCall{}
	acc.justFinishedReasoning = ""
	acc.justFinishedAudioTranscript = ""
	acc.justFinishedFileSearch = FinishedResponseFileSearch{}
	acc.justFinishedWebSearch = FinishedResponseWebSearch{}
	acc.justFinishedCodeInterpreter = FinishedResponseCodeInterpreter{}
	acc.justFinishedError = ""

	switch event.Type {
	case "response.created":
		acc.Response = event.Response
		return true
	case "response.in_progress":
		acc.Response = event.Response
		return true
	case "response.completed":
		acc.Response = event.Response
		return true
	case "response.failed":
		acc.Response = event.Response
		acc.justFinishedType = "error"
		acc.justFinishedError = event.Response.Error.Message
		return true
	case "response.incomplete":
		acc.Response = event.Response
		return true
	case "response.output_item.added":
		// append item (item comes from Response.Output in real events)
		// In streaming, the item should already be on event.Response.Output
		if len(event.Response.Output) > 0 {
			acc.Output = append(acc.Output, event.Response.Output...)
		}
		return true
	case "response.output_item.done":
		// nothing special to accumulate; mark last output item done
		acc.justFinishedType = "output_item_done"
		return true
	case "response.output_text.delta":
		// try to locate output index
		oi := int(event.OutputIndex)
		acc.Output = expandToFitResponseOutput(acc.Output, oi)
		// ensure Content slice exists
		if len(acc.Output[oi].Content) == 0 {
			acc.Output[oi].Content = []ResponseOutputMessageContentUnion{{}}
		}
		// append text to first content of type output_text
		for i := range acc.Output[oi].Content {
			if acc.Output[oi].Content[i].Type == "output_text" || acc.Output[oi].Content[i].Type == "" {
				acc.Output[oi].Content[i].Text += event.Delta
				break
			}
		}
		return true
	case "response.output_text.done":
		acc.justFinishedType = "text_done"
		acc.justFinishedText = event.Text
		// also set text on output index if possible
		oi := int(event.OutputIndex)
		acc.Output = expandToFitResponseOutput(acc.Output, oi)
		if len(acc.Output[oi].Content) == 0 {
			acc.Output[oi].Content = []ResponseOutputMessageContentUnion{{}}
		}
		acc.Output[oi].Content[0].Type = "output_text"
		acc.Output[oi].Content[0].Text = event.Text
		return true
	case "response.refusal.delta":
		// append to existing refusal on response's output or transient
		// store into transient refusal fragment
		acc.justFinishedRefusal += event.Delta
		return true
	case "response.refusal.done":
		acc.justFinishedType = "refusal_done"
		acc.justFinishedRefusal = event.Refusal
		return true
	case "response.function_call_arguments.delta":
		// no-op for now
		return true
	case "response.function_call_arguments.done":
		// assign an index for this item id
		idx := acc.nextToolIndex
		acc.nextToolIndex++
		acc.toolCallIndices[event.ItemID] = idx
		acc.justFinishedType = "function_call_args_done"
		acc.justFinishedTool = FinishedResponseToolCall{CallID: event.ItemID, Index: idx}
		return true
	case "response.reasoning_text.done":
		acc.justFinishedType = "reasoning_done"
		acc.justFinishedReasoning = event.Text
		return true
	case "response.audio.transcript.done":
		acc.justFinishedType = "audio_transcript_done"
		// transcript sequence number exists but not text; event may not include text
		return true
	case "response.file_search_call.completed":
		acc.justFinishedType = "file_search_completed"
		acc.justFinishedFileSearch = FinishedResponseFileSearch{CallID: event.ItemID}
		return true
	case "response.web_search_call.completed":
		acc.justFinishedType = "web_search_completed"
		acc.justFinishedWebSearch = FinishedResponseWebSearch{CallID: event.ItemID}
		return true
	case "response.code_interpreter_call.completed":
		acc.justFinishedType = "code_interpreter_completed"
		acc.justFinishedCodeInterpreter = FinishedResponseCodeInterpreter{ItemID: event.ItemID}
		return true
	case "error":
		acc.justFinishedType = "error"
		acc.justFinishedError = event.Message
		return true
	default:
		// unknown/unsupported event: no-op but accept
		return true
	}
}

func (acc *ResponseAccumulator) JustFinishedText() (FinishedResponseText, bool) {
	if acc.justFinishedType == "text_done" {
		return FinishedResponseText{Text: acc.justFinishedText}, true
	}
	return FinishedResponseText{}, false
}

func (acc *ResponseAccumulator) JustFinishedToolCall() (FinishedResponseToolCall, bool) {
	if acc.justFinishedType == "function_call_args_done" {
		return acc.justFinishedTool, true
	}
	return FinishedResponseToolCall{}, false
}

func (acc *ResponseAccumulator) JustFinishedRefusal() (FinishedResponseRefusal, bool) {
	if acc.justFinishedType == "refusal_done" {
		return FinishedResponseRefusal{Refusal: acc.justFinishedRefusal}, true
	}
	return FinishedResponseRefusal{}, false
}

func (acc *ResponseAccumulator) JustFinishedReasoning() (FinishedResponseReasoning, bool) {
	if acc.justFinishedType == "reasoning_done" {
		return FinishedResponseReasoning{Text: acc.justFinishedReasoning}, true
	}
	return FinishedResponseReasoning{}, false
}

func (acc *ResponseAccumulator) JustFinishedAudioTranscript() (FinishedResponseAudioTranscript, bool) {
	if acc.justFinishedType == "audio_transcript_done" {
		return FinishedResponseAudioTranscript{Transcript: acc.justFinishedAudioTranscript}, true
	}
	return FinishedResponseAudioTranscript{}, false
}

func (acc *ResponseAccumulator) JustFinishedFileSearch() (FinishedResponseFileSearch, bool) {
	if acc.justFinishedType == "file_search_completed" {
		return acc.justFinishedFileSearch, true
	}
	return FinishedResponseFileSearch{}, false
}

func (acc *ResponseAccumulator) JustFinishedWebSearch() (FinishedResponseWebSearch, bool) {
	if acc.justFinishedType == "web_search_completed" {
		return acc.justFinishedWebSearch, true
	}
	return FinishedResponseWebSearch{}, false
}

func (acc *ResponseAccumulator) JustFinishedCodeInterpreter() (FinishedResponseCodeInterpreter, bool) {
	if acc.justFinishedType == "code_interpreter_completed" {
		return acc.justFinishedCodeInterpreter, true
	}
	return FinishedResponseCodeInterpreter{}, false
}

func (acc *ResponseAccumulator) JustFinishedError() (FinishedResponseError, bool) {
	if acc.justFinishedType == "error" && acc.justFinishedError != "" {
		return FinishedResponseError{Err: acc.justFinishedError}, true
	}
	// also fall back to Response.Error
	if acc.Error.Message != "" {
		return FinishedResponseError{Err: acc.Error.Message}, true
	}
	return FinishedResponseError{}, false
}

func (acc *ResponseAccumulator) GetToolCallIndex(callID string) int64 {
	if idx, ok := acc.toolCallIndices[callID]; ok {
		return idx
	}
	return -1
}

func (acc *ResponseAccumulator) IsComplete() bool {
	return acc.Status == ResponseStatusCompleted
}

// helpers
func expandToFitResponseOutput(slice []ResponseOutputItemUnion, index int) []ResponseOutputItemUnion {
	if index < len(slice) {
		return slice
	}
	if index < cap(slice) {
		return slice[:index+1]
	}
	newSlice := make([]ResponseOutputItemUnion, index+1)
	copy(newSlice, slice)
	return newSlice
}
