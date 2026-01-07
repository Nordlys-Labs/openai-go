package responses_test

import (
	"testing"

	"github.com/Nordlys-Labs/openai-go/responses"
	"github.com/Nordlys-Labs/openai-go/shared/constant"
)

func TestResponseAccumulator_BasicLifecycle(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Test response.created event
	createdEvent := responses.ResponseStreamEventUnion{
		Type: "response.created",
		Response: responses.Response{
			Status: "in_progress",
		},
	}
	if !acc.AddEvent(createdEvent) {
		t.Fatal("Failed to add response.created event")
	}

	// Test response.in_progress event
	inProgressEvent := responses.ResponseStreamEventUnion{
		Type: "response.in_progress",
		Response: responses.Response{
			Status: "in_progress",
		},
	}
	if !acc.AddEvent(inProgressEvent) {
		t.Fatal("Failed to add response.in_progress event")
	}

	// Test response.completed event
	completedEvent := responses.ResponseStreamEventUnion{
		Type: "response.completed",
		Response: responses.Response{
			Status: responses.ResponseStatusCompleted,
		},
	}
	if !acc.AddEvent(completedEvent) {
		t.Fatal("Failed to add response.completed event")
	}

	if !acc.IsComplete() {
		t.Error("Response should be marked as complete")
	}
}

func TestResponseAccumulator_TextAccumulation(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add output item
	addItemEvent := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 0,
	}
	if !acc.AddEvent(addItemEvent) {
		t.Fatal("Failed to add output item")
	}

	// Add text delta events
	delta1 := responses.ResponseStreamEventUnion{
		Type:        "response.output_text.delta",
		OutputIndex: 0,
		Delta:       "Hello",
	}
	if !acc.AddEvent(delta1) {
		t.Fatal("Failed to add first text delta")
	}

	delta2 := responses.ResponseStreamEventUnion{
		Type:        "response.output_text.delta",
		OutputIndex: 0,
		Delta:       " world",
	}
	if !acc.AddEvent(delta2) {
		t.Fatal("Failed to add second text delta")
	}

	// Check accumulated text
	if len(acc.Response.Output) == 0 {
		t.Fatal("Output array should not be empty")
	}
	if len(acc.Response.Output[0].Content) == 0 {
		t.Fatal("Content array should not be empty")
	}
	if acc.Response.Output[0].Content[0].Text != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", acc.Response.Output[0].Content[0].Text)
	}

	// Add text done event
	doneEvent := responses.ResponseStreamEventUnion{
		Type:        "response.output_text.done",
		OutputIndex: 0,
		Text:        "Hello world",
	}
	if !acc.AddEvent(doneEvent) {
		t.Fatal("Failed to add text done event")
	}

	// Check JustFinishedText
	if finished, ok := acc.JustFinishedText(); !ok {
		t.Error("Expected text to be just finished")
	} else if finished.Text != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", finished.Text)
	}

	// After another event, JustFinishedText should be false
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type: "response.completed",
		Response: responses.Response{
			Status: responses.ResponseStatusCompleted,
		},
	})
	if _, ok := acc.JustFinishedText(); ok {
		t.Error("JustFinishedText should be false after another event")
	}
}

func TestResponseAccumulator_RefusalAccumulation(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add refusal delta events
	delta1 := responses.ResponseStreamEventUnion{
		Type:  "response.refusal.delta",
		Delta: "I cannot",
	}
	acc.AddEvent(delta1)

	delta2 := responses.ResponseStreamEventUnion{
		Type:  "response.refusal.delta",
		Delta: " help with that",
	}
	acc.AddEvent(delta2)

	// Add refusal done event
	doneEvent := responses.ResponseStreamEventUnion{
		Type:    "response.refusal.done",
		Refusal: "I cannot help with that",
	}
	if !acc.AddEvent(doneEvent) {
		t.Fatal("Failed to add refusal done event")
	}

	// Check JustFinishedRefusal
	if finished, ok := acc.JustFinishedRefusal(); !ok {
		t.Error("Expected refusal to be just finished")
	} else if finished.Refusal != "I cannot help with that" {
		t.Errorf("Expected 'I cannot help with that', got '%s'", finished.Refusal)
	}
}

func TestResponseAccumulator_ToolCallTracking(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add function call arguments delta
	deltaEvent := responses.ResponseStreamEventUnion{
		Type:   "response.function_call_arguments.delta",
		ItemID: "call_123",
		Delta:  `{"location"`,
	}
	acc.AddEvent(deltaEvent)

	// Add function call arguments done
	doneEvent := responses.ResponseStreamEventUnion{
		Type:      "response.function_call_arguments.done",
		ItemID:    "call_123",
		Name:      "get_weather",
		Arguments: `{"location":"Paris"}`,
	}
	if !acc.AddEvent(doneEvent) {
		t.Fatal("Failed to add function call done event")
	}

	// Check JustFinishedToolCall
	if finished, ok := acc.JustFinishedToolCall(); !ok {
		t.Error("Expected tool call to be just finished")
	} else {
		if finished.CallID != "call_123" {
			t.Errorf("Expected call_123, got '%s'", finished.CallID)
		}
		if finished.Index != 0 {
			t.Errorf("Expected index 0, got %d", finished.Index)
		}
	}

	// Check GetToolCallIndex
	if idx := acc.GetToolCallIndex("call_123"); idx != 0 {
		t.Errorf("Expected index 0 for call_123, got %d", idx)
	}

	// Add another tool call
	doneEvent2 := responses.ResponseStreamEventUnion{
		Type:      "response.function_call_arguments.done",
		ItemID:    "call_456",
		Name:      "get_time",
		Arguments: `{"timezone":"UTC"}`,
	}
	acc.AddEvent(doneEvent2)

	// Check second tool call gets index 1
	if idx := acc.GetToolCallIndex("call_456"); idx != 1 {
		t.Errorf("Expected index 1 for call_456, got %d", idx)
	}

	// Unknown call should return -1
	if idx := acc.GetToolCallIndex("unknown"); idx != -1 {
		t.Errorf("Expected -1 for unknown call, got %d", idx)
	}
}

func TestResponseAccumulator_ReasoningAccumulation(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add reasoning text done event
	doneEvent := responses.ResponseStreamEventUnion{
		Type: "response.reasoning_text.done",
		Text: "Step 1: Analyze the problem...",
	}
	if !acc.AddEvent(doneEvent) {
		t.Fatal("Failed to add reasoning done event")
	}

	// Check JustFinishedReasoning
	if finished, ok := acc.JustFinishedReasoning(); !ok {
		t.Error("Expected reasoning to be just finished")
	} else if finished.Text != "Step 1: Analyze the problem..." {
		t.Errorf("Expected reasoning text, got '%s'", finished.Text)
	}
}

func TestResponseAccumulator_AudioTranscriptAccumulation(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add audio transcript done event
	doneEvent := responses.ResponseStreamEventUnion{
		Type: "response.audio.transcript.done",
	}
	if !acc.AddEvent(doneEvent) {
		t.Fatal("Failed to add audio transcript done event")
	}

	// Check JustFinishedAudioTranscript
	if _, ok := acc.JustFinishedAudioTranscript(); !ok {
		t.Error("Expected audio transcript to be just finished")
	}
}

func TestResponseAccumulator_BuiltInToolsCompletion(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Test file search completion
	fileSearchEvent := responses.ResponseStreamEventUnion{
		Type:   "response.file_search_call.completed",
		ItemID: "fs_123",
	}
	if !acc.AddEvent(fileSearchEvent) {
		t.Fatal("Failed to add file search completed event")
	}
	if finished, ok := acc.JustFinishedFileSearch(); !ok {
		t.Error("Expected file search to be just finished")
	} else if finished.CallID != "fs_123" {
		t.Errorf("Expected fs_123, got '%s'", finished.CallID)
	}

	// Test web search completion
	webSearchEvent := responses.ResponseStreamEventUnion{
		Type:   "response.web_search_call.completed",
		ItemID: "ws_123",
	}
	if !acc.AddEvent(webSearchEvent) {
		t.Fatal("Failed to add web search completed event")
	}
	if finished, ok := acc.JustFinishedWebSearch(); !ok {
		t.Error("Expected web search to be just finished")
	} else if finished.CallID != "ws_123" {
		t.Errorf("Expected ws_123, got '%s'", finished.CallID)
	}

	// Test code interpreter completion
	codeInterpreterEvent := responses.ResponseStreamEventUnion{
		Type:   "response.code_interpreter_call.completed",
		ItemID: "ci_123",
	}
	if !acc.AddEvent(codeInterpreterEvent) {
		t.Fatal("Failed to add code interpreter completed event")
	}
	if finished, ok := acc.JustFinishedCodeInterpreter(); !ok {
		t.Error("Expected code interpreter to be just finished")
	} else if finished.ItemID != "ci_123" {
		t.Errorf("Expected ci_123, got '%s'", finished.ItemID)
	}
}

func TestResponseAccumulator_ErrorHandling(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Test error event
	errorEvent := responses.ResponseStreamEventUnion{
		Type:    "error",
		Message: "Something went wrong",
	}
	if !acc.AddEvent(errorEvent) {
		t.Fatal("Failed to add error event")
	}

	// Check JustFinishedError
	if finished, ok := acc.JustFinishedError(); !ok {
		t.Error("Expected error to be just finished")
	} else if finished.Err != "Something went wrong" {
		t.Errorf("Expected error message, got '%s'", finished.Err)
	}

	// Test failed response event
	acc2 := responses.NewResponseAccumulator()
	failedEvent := responses.ResponseStreamEventUnion{
		Type: "response.failed",
		Response: responses.Response{
			Error: responses.ResponseError{
				Message: "Failed to complete",
			},
		},
	}
	acc2.AddEvent(failedEvent)

	// JustFinishedError should work immediately after failed event
	if finished, ok := acc2.JustFinishedError(); !ok {
		t.Error("Expected error to be just finished after failed event")
	} else if finished.Err != "Failed to complete" {
		t.Errorf("Expected 'Failed to complete', got '%s'", finished.Err)
	}
}

func TestResponseAccumulator_MultipleOutputIndices(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add text to output index 0
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type:        "response.output_text.delta",
		OutputIndex: 0,
		Delta:       "First",
	})

	// Add text to output index 2 (skipping 1)
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type:        "response.output_text.delta",
		OutputIndex: 2,
		Delta:       "Third",
	})

	// Check that output array expanded correctly
	if len(acc.Response.Output) < 3 {
		t.Fatalf("Expected at least 3 output items, got %d", len(acc.Response.Output))
	}

	if acc.Response.Output[0].Content[0].Text != "First" {
		t.Errorf("Expected 'First' at index 0, got '%s'", acc.Response.Output[0].Content[0].Text)
	}

	if acc.Response.Output[2].Content[0].Text != "Third" {
		t.Errorf("Expected 'Third' at index 2, got '%s'", acc.Response.Output[2].Content[0].Text)
	}
}

func TestResponseAccumulator_UnknownEventTypes(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Unknown event types should be accepted (no-op)
	unknownEvent := responses.ResponseStreamEventUnion{
		Type: "response.unknown.event",
	}
	if !acc.AddEvent(unknownEvent) {
		t.Error("Unknown event types should return true (no-op)")
	}
}

func TestResponseAccumulator_JustFinishedStateClearsOnNextEvent(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add text done event
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type:        "response.output_text.done",
		OutputIndex: 0,
		Text:        "Done",
	})

	// Verify text is just finished
	if _, ok := acc.JustFinishedText(); !ok {
		t.Fatal("Expected text to be just finished")
	}

	// Add another event
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type: "response.completed",
	})

	// Verify all JustFinished methods return false
	if _, ok := acc.JustFinishedText(); ok {
		t.Error("JustFinishedText should be false after another event")
	}
	if _, ok := acc.JustFinishedToolCall(); ok {
		t.Error("JustFinishedToolCall should be false")
	}
	if _, ok := acc.JustFinishedRefusal(); ok {
		t.Error("JustFinishedRefusal should be false")
	}
	if _, ok := acc.JustFinishedReasoning(); ok {
		t.Error("JustFinishedReasoning should be false")
	}
	if _, ok := acc.JustFinishedAudioTranscript(); ok {
		t.Error("JustFinishedAudioTranscript should be false")
	}
	if _, ok := acc.JustFinishedFileSearch(); ok {
		t.Error("JustFinishedFileSearch should be false")
	}
	if _, ok := acc.JustFinishedWebSearch(); ok {
		t.Error("JustFinishedWebSearch should be false")
	}
	if _, ok := acc.JustFinishedCodeInterpreter(); ok {
		t.Error("JustFinishedCodeInterpreter should be false")
	}
}

func TestResponseAccumulator_StatusTracking(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Initially not complete
	if acc.IsComplete() {
		t.Error("Response should not be complete initially")
	}

	// Set status to completed
	acc.Response.Status = responses.ResponseStatusCompleted

	// Now should be complete
	if !acc.IsComplete() {
		t.Error("Response should be complete after setting status")
	}
}

func TestResponseAccumulator_OutputItemDone(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add output item done event
	doneEvent := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.done",
		OutputIndex: 0,
	}
	if !acc.AddEvent(doneEvent) {
		t.Fatal("Failed to add output item done event")
	}

	// Verify event was processed (just stored as type)
	// This event type doesn't have a specific JustFinished method
	// but should be handled without error
}

func TestResponseAccumulator_ResponseIncomplete(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add incomplete event
	incompleteEvent := responses.ResponseStreamEventUnion{
		Type: "response.incomplete",
	}
	if !acc.AddEvent(incompleteEvent) {
		t.Fatal("Failed to add incomplete event")
	}

	// Response object should be updated
	// (In real scenario, the event would contain a Response with status set)
}

func TestResponseAccumulator_EmptyInitialization(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Verify initialization
	if acc.Response.Output == nil {
		// This is OK - slice will be nil initially
	}

	if len(acc.Response.Output) != 0 {
		t.Errorf("Expected empty output array, got %d items", len(acc.Response.Output))
	}

	if acc.GetToolCallIndex("nonexistent") != -1 {
		t.Error("Expected -1 for nonexistent tool call")
	}

	if acc.IsComplete() {
		t.Error("Response should not be complete initially")
	}
}

func TestResponseAccumulator_ConstantTypes(t *testing.T) {
	// Verify constant types are available
	_ = constant.ValueOf[constant.ResponseCompleted]()
	_ = constant.ValueOf[constant.ResponseOutputTextDelta]()
	_ = constant.ValueOf[constant.ResponseOutputTextDone]()
	_ = constant.ValueOf[constant.ResponseFunctionCallArgumentsDone]()
}

func TestResponseAccumulator_FunctionCallAdded(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add output_item.added event with function_call type
	addItemEvent := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: responses.ResponseOutputItemUnion{
			ID:     "item_123",
			Type:   "function_call",
			CallID: "call_123",
			Name:   "get_weather",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:     "item_123",
				Type:   "function_call",
				CallID: "call_123",
				Name:   "get_weather",
			}},
		},
	}
	if !acc.AddEvent(addItemEvent) {
		t.Fatal("Failed to add function call item")
	}

	// Check JustAddedFunctionCall
	if added, ok := acc.JustAddedFunctionCall(); !ok {
		t.Error("Expected function call to be just added")
	} else {
		if added.CallID != "call_123" {
			t.Errorf("Expected call_123, got '%s'", added.CallID)
		}
		if added.Index != 0 {
			t.Errorf("Expected index 0, got %d", added.Index)
		}
		if added.Name != "get_weather" {
			t.Errorf("Expected get_weather, got '%s'", added.Name)
		}
		if added.OutputIndex != 0 {
			t.Errorf("Expected OutputIndex 0, got %d", added.OutputIndex)
		}
	}

	// After another event, JustAddedFunctionCall should be false
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type: "response.completed",
	})
	if _, ok := acc.JustAddedFunctionCall(); ok {
		t.Error("JustAddedFunctionCall should be false after another event")
	}
}

func TestResponseAccumulator_FunctionCallDelta(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add function_call item first
	addItemEvent := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: responses.ResponseOutputItemUnion{
			ID:     "item_123",
			Type:   "function_call",
			CallID: "call_123",
			Name:   "get_weather",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:     "item_123",
				Type:   "function_call",
				CallID: "call_123",
				Name:   "get_weather",
			}},
		},
	}
	acc.AddEvent(addItemEvent)

	// Add function_call_arguments.delta
	deltaEvent := responses.ResponseStreamEventUnion{
		Type:   "response.function_call_arguments.delta",
		ItemID: "item_123",
		Delta:  `{"location":"Paris"}`,
	}
	if !acc.AddEvent(deltaEvent) {
		t.Fatal("Failed to add function call delta")
	}

	// Check JustDeltaFunctionCall
	if delta, ok := acc.JustDeltaFunctionCall(); !ok {
		t.Error("Expected function call delta to be just received")
	} else {
		if delta.Index != 0 {
			t.Errorf("Expected index 0, got %d", delta.Index)
		}
		if delta.Delta != `{"location":"Paris"}` {
			t.Errorf("Expected delta with JSON, got '%s'", delta.Delta)
		}
		if delta.OutputIndex != 0 {
			t.Errorf("Expected OutputIndex 0, got %d", delta.OutputIndex)
		}
	}

	// After another event, JustDeltaFunctionCall should be false
	acc.AddEvent(responses.ResponseStreamEventUnion{
		Type: "response.completed",
	})
	if _, ok := acc.JustDeltaFunctionCall(); ok {
		t.Error("JustDeltaFunctionCall should be false after another event")
	}
}

func TestResponseAccumulator_GetFunctionCallMeta(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add first function_call item (gets index 0)
	addItemEvent1 := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: responses.ResponseOutputItemUnion{
			ID:     "item_123",
			Type:   "function_call",
			CallID: "call_123",
			Name:   "get_weather",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:     "item_123",
				Type:   "function_call",
				CallID: "call_123",
				Name:   "get_weather",
			}},
		},
	}
	acc.AddEvent(addItemEvent1)

	// Add second function_call item (gets index 1)
	addItemEvent2 := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: responses.ResponseOutputItemUnion{
			ID:     "item_456",
			Type:   "function_call",
			CallID: "call_456",
			Name:   "get_time",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:     "item_456",
				Type:   "function_call",
				CallID: "call_456",
				Name:   "get_time",
			}},
		},
	}
	acc.AddEvent(addItemEvent2)

	// Check GetFunctionCallMeta for first call (index 0)
	if meta, ok := acc.GetFunctionCallMeta("item_123"); !ok {
		t.Error("Expected function call meta to be found for item_123")
	} else {
		if meta.CallID != "call_123" {
			t.Errorf("Expected call_123, got '%s'", meta.CallID)
		}
		if meta.Name != "get_weather" {
			t.Errorf("Expected get_weather, got '%s'", meta.Name)
		}
		if meta.Index != 0 {
			t.Errorf("Expected index 0 for first call, got %d", meta.Index)
		}
		if meta.OutputIndex != 0 {
			t.Errorf("Expected OutputIndex 0, got %d", meta.OutputIndex)
		}
	}

	// Check GetFunctionCallMeta for second call (index 1)
	if meta, ok := acc.GetFunctionCallMeta("item_456"); !ok {
		t.Error("Expected function call meta to be found for item_456")
	} else {
		if meta.CallID != "call_456" {
			t.Errorf("Expected call_456, got '%s'", meta.CallID)
		}
		if meta.Name != "get_time" {
			t.Errorf("Expected get_time, got '%s'", meta.Name)
		}
		if meta.Index != 1 {
			t.Errorf("Expected index 1 for second call, got %d", meta.Index)
		}
		if meta.OutputIndex != 1 {
			t.Errorf("Expected OutputIndex 1, got %d", meta.OutputIndex)
		}
	}

	// Unknown item ID should return not found
	if _, ok := acc.GetFunctionCallMeta("unknown"); ok {
		t.Error("GetFunctionCallMeta should return false for unknown item ID")
	}
}

func TestResponseAccumulator_MultipleFunctionCallsWithDeltas(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add first function call item
	addItemEvent1 := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: responses.ResponseOutputItemUnion{
			ID:     "item_1",
			Type:   "function_call",
			CallID: "call_1",
			Name:   "get_weather",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:     "item_1",
				Type:   "function_call",
				CallID: "call_1",
				Name:   "get_weather",
			}},
		},
	}
	acc.AddEvent(addItemEvent1)

	// Add second function call item
	addItemEvent2 := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: responses.ResponseOutputItemUnion{
			ID:     "item_2",
			Type:   "function_call",
			CallID: "call_2",
			Name:   "get_time",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:     "item_2",
				Type:   "function_call",
				CallID: "call_2",
				Name:   "get_time",
			}},
		},
	}
	acc.AddEvent(addItemEvent2)

	// Add delta for second function call
	deltaEvent2 := responses.ResponseStreamEventUnion{
		Type:   "response.function_call_arguments.delta",
		ItemID: "item_2",
		Delta:  `{"timezone":"UTC"}`,
	}
	acc.AddEvent(deltaEvent2)

	// Check delta for second call uses correct index
	if delta, ok := acc.JustDeltaFunctionCall(); !ok {
		t.Error("Expected function call delta to be just received")
	} else {
		if delta.Index != 1 {
			t.Errorf("Expected index 1 for second call, got %d", delta.Index)
		}
	}

	// Add delta for first function call
	deltaEvent1 := responses.ResponseStreamEventUnion{
		Type:   "response.function_call_arguments.delta",
		ItemID: "item_1",
		Delta:  `{"location":"Paris"}`,
	}
	acc.AddEvent(deltaEvent1)

	// Check delta for first call uses correct index
	if delta, ok := acc.JustDeltaFunctionCall(); !ok {
		t.Error("Expected function call delta to be just received")
	} else {
		if delta.Index != 0 {
			t.Errorf("Expected index 0 for first call, got %d", delta.Index)
		}
	}
}

func TestResponseAccumulator_FunctionCallDeltaBeforeAdded(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add delta before function_call item (should be handled gracefully)
	deltaEvent := responses.ResponseStreamEventUnion{
		Type:   "response.function_call_arguments.delta",
		ItemID: "item_unknown",
		Delta:  `{"test":"value"}`,
	}
	if !acc.AddEvent(deltaEvent) {
		t.Fatal("Failed to add delta event")
	}

	// JustDeltaFunctionCall should return false (no metadata found)
	if _, ok := acc.JustDeltaFunctionCall(); ok {
		t.Error("JustDeltaFunctionCall should return false when metadata not found")
	}
}

func TestResponseAccumulator_NonFunctionCallItemAdded(t *testing.T) {
	acc := responses.NewResponseAccumulator()

	// Add output_item.added event with message type (not function_call)
	addItemEvent := responses.ResponseStreamEventUnion{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: responses.ResponseOutputItemUnion{
			ID:   "item_123",
			Type: "message",
		},
		Response: responses.Response{
			Output: []responses.ResponseOutputItemUnion{{
				ID:   "item_123",
				Type: "message",
			}},
		},
	}
	if !acc.AddEvent(addItemEvent) {
		t.Fatal("Failed to add message item")
	}

	// JustAddedFunctionCall should return false for non-function_call items
	if _, ok := acc.JustAddedFunctionCall(); ok {
		t.Error("JustAddedFunctionCall should return false for non-function_call items")
	}
}
