package usage

import (
	"context"
	"testing"
)

func TestGenerateEnabledDefaultsNilToTrue(t *testing.T) {
	if !GenerateEnabled(nil) {
		t.Fatalf("GenerateEnabled(nil) = false, want true")
	}
}

func TestGenerateEnabledHonorsExplicitFalse(t *testing.T) {
	if GenerateEnabled(GenerateFlag(false)) {
		t.Fatalf("GenerateEnabled(false) = true, want false")
	}
}

func TestGenerateEnabledHonorsExplicitTrue(t *testing.T) {
	if !GenerateEnabled(GenerateFlag(true)) {
		t.Fatalf("GenerateEnabled(true) = false, want true")
	}
}

func TestGenerateFromContextDefaultsMissingToTrue(t *testing.T) {
	if !GenerateFromContext(context.Background()) {
		t.Fatalf("GenerateFromContext(background) = false, want true")
	}
}

func TestGenerateFromContextHonorsExplicitFalse(t *testing.T) {
	ctx := WithGenerate(context.Background(), false)
	if GenerateFromContext(ctx) {
		t.Fatalf("GenerateFromContext(false) = true, want false")
	}
}

func TestRecordOmittedGenerateIsEnabled(t *testing.T) {
	// Existing callers construct Record without setting Generate.
	// Omission must remain distinguishable from explicit false and default to true.
	record := Record{
		Provider: "openai",
		Model:    "gpt-5.4",
	}
	if record.Generate != nil {
		t.Fatalf("Record.Generate = %v, want nil for omitted field", record.Generate)
	}
	if !GenerateEnabled(record.Generate) {
		t.Fatalf("GenerateEnabled(omitted) = false, want true")
	}
}

func TestManagerPublishAssignsEventID(t *testing.T) {
	m := NewManager(1)
	plugin := pluginFunc(func(_ context.Context, record Record) {
		if record.EventID == "" {
			t.Errorf("generated EventID is empty")
		}
	})
	m.Register(plugin)
	m.Start(context.Background())
	m.Publish(context.Background(), Record{Provider: "test"})
	m.Stop()
}

func TestManagerPublishPreservesEventID(t *testing.T) {
	m := NewManager(1)
	want := "event-fixed"
	plugin := pluginFunc(func(_ context.Context, record Record) {
		if record.EventID != want {
			t.Errorf("EventID = %q, want %q", record.EventID, want)
		}
	})
	m.Register(plugin)
	m.Start(context.Background())
	m.Publish(context.Background(), Record{EventID: want})
	m.Stop()
}

type pluginFunc func(context.Context, Record)

func (f pluginFunc) HandleUsage(ctx context.Context, record Record) { f(ctx, record) }
