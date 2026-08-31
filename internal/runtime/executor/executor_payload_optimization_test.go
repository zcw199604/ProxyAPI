package executor

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestEnsureColonSpacedJSONLeavesInvalidPayloadUnchanged(t *testing.T) {
	input := []byte(`{"text":"unterminated}`)
	output := ensureColonSpacedJSON(input)
	if &output[0] != &input[0] || string(output) != string(input) {
		t.Fatal("invalid JSON payload changed")
	}
}

func TestNormalizeKimiToolMessageLinksReusesCanonicalPayload(t *testing.T) {
	input := []byte(`{"messages":[{"role":"assistant","reasoning_content":"checking","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}]}`)
	output, errNormalize := normalizeKimiToolMessageLinks(input)
	if errNormalize != nil {
		t.Fatalf("normalizeKimiToolMessageLinks returned error: %v", errNormalize)
	}
	if &output[0] != &input[0] {
		t.Fatal("canonical Kimi tool history was copied")
	}
}

func TestNormalizeKimiToolMessageLinksPreservesLargeArguments(t *testing.T) {
	input := []byte(`{"messages":[{"role":"assistant","content":"lookup","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":{"id":9007199254740993}}}]},{"role":"tool","call_id":"call_1","content":"ok"}]}`)
	output, errNormalize := normalizeKimiToolMessageLinks(input)
	if errNormalize != nil {
		t.Fatalf("normalizeKimiToolMessageLinks returned error: %v", errNormalize)
	}
	if got := gjson.GetBytes(output, "messages.0.tool_calls.0.function.arguments.id").Raw; got != "9007199254740993" {
		t.Fatalf("argument id = %s, want exact large integer", got)
	}
	if got := gjson.GetBytes(output, "messages.1.tool_call_id").String(); got != "call_1" {
		t.Fatalf("tool_call_id = %q, want call_1", got)
	}
	if got := gjson.GetBytes(output, "messages.0.reasoning_content").String(); got != "lookup" {
		t.Fatalf("reasoning_content = %q, want lookup", got)
	}
}

var benchmarkExecutorPayloadOutput []byte

func BenchmarkNormalizeKimiToolMessageLinksLargeSinglePatch(b *testing.B) {
	content := strings.Repeat("x", 8<<20)
	input := []byte(`{"messages":[{"role":"assistant","content":"` + content + `","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}]}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for b.Loop() {
		benchmarkExecutorPayloadOutput, _ = normalizeKimiToolMessageLinks(input)
	}
}

func BenchmarkNormalizeKimiToolMessageLinksLargeMultiplePatches(b *testing.B) {
	content := strings.Repeat("x", (8<<20)/32)
	var builder strings.Builder
	builder.Grow(8 << 20)
	builder.WriteString(`{"messages":[`)
	for index := 0; index < 32; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{"role":"assistant","content":"`)
		builder.WriteString(content)
		builder.WriteString(`","tool_calls":[{"id":"call_`)
		builder.WriteString(strings.Repeat("x", index%3))
		builder.WriteString(`","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","call_id":"call_`)
		builder.WriteString(strings.Repeat("x", index%3))
		builder.WriteString(`","content":"ok"}`)
	}
	builder.WriteString(`]}`)
	input := []byte(builder.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for b.Loop() {
		benchmarkExecutorPayloadOutput, _ = normalizeKimiToolMessageLinks(input)
	}
}

func BenchmarkNormalizeKimiToolMessageLinksLargeCanonicalPayload(b *testing.B) {
	content := strings.Repeat("x", (8<<20)/64)
	var builder strings.Builder
	builder.Grow(8 << 20)
	builder.WriteString(`{"messages":[`)
	for index := 0; index < 64; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{"role":"user","content":"`)
		builder.WriteString(content)
		builder.WriteString(`"}`)
	}
	builder.WriteString(`]}`)
	input := []byte(builder.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for b.Loop() {
		benchmarkExecutorPayloadOutput, _ = normalizeKimiToolMessageLinks(input)
	}
}
