package secrets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvFileStore_GetPut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	store := NewEnvFileStore(path)
	ctx := context.Background()

	if err := store.Put(ctx, "MY_KEY", "my_value"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get(ctx, "MY_KEY")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "my_value" {
		t.Errorf("Get = %q, want %q", got, "my_value")
	}
}

func TestEnvFileStore_QuotesComposeSensitiveValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	store := NewEnvFileStore(path)
	ctx := context.Background()

	value := "pa$$ word#with'quote"
	if err := store.Put(ctx, "SECRET", value); err != nil {
		t.Fatalf("Put: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(data); !strings.Contains(got, "SECRET='pa$$ word#with\\'quote'") {
		t.Fatalf("stored value was not quoted for Compose interpolation: %q", got)
	}

	got, err := store.Get(ctx, "SECRET")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != value {
		t.Errorf("Get = %q, want %q", got, value)
	}
}

func TestEnvFileStore_Upsert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	store := NewEnvFileStore(path)
	ctx := context.Background()

	store.Put(ctx, "KEY", "first")
	store.Put(ctx, "KEY", "second")

	data, _ := os.ReadFile(path)
	count := strings.Count(string(data), "KEY=")
	if count != 1 {
		t.Errorf("expected 1 occurrence of KEY=, got %d\n%s", count, data)
	}

	got, _ := store.Get(ctx, "KEY")
	if got != "second" {
		t.Errorf("Get after upsert = %q, want %q", got, "second")
	}
}

func TestEnvFileStore_GetMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	store := NewEnvFileStore(path)
	ctx := context.Background()

	got, err := store.Get(ctx, "NONEXISTENT")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "" {
		t.Errorf("Get = %q, want empty", got)
	}
}

func TestEnvFileStore_PreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("EXISTING=original\n"), 0600)

	store := NewEnvFileStore(path)
	ctx := context.Background()

	store.Put(ctx, "NEW_KEY", "new_value")

	existing, _ := store.Get(ctx, "EXISTING")
	if existing != "original" {
		t.Errorf("EXISTING = %q, want %q", existing, "original")
	}
	newVal, _ := store.Get(ctx, "NEW_KEY")
	if newVal != "new_value" {
		t.Errorf("NEW_KEY = %q, want %q", newVal, "new_value")
	}
}

func TestEnvFileStore_Generate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	store := NewEnvFileStore(path)
	ctx := context.Background()

	val, err := store.Generate(ctx, "TEST_SECRET", 32, GenerateOpts{Recipe: "letters,digits,32"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(val) != 32 {
		t.Errorf("len = %d, want 32", len(val))
	}

	stored, _ := store.Get(ctx, "TEST_SECRET")
	if stored != val {
		t.Errorf("stored value doesn't match generated")
	}
}

func TestEnvFileStore_GenerateHex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	store := NewEnvFileStore(path)
	ctx := context.Background()

	val, err := store.Generate(ctx, "HEX_TOKEN", 64, GenerateOpts{Recipe: "hex"})
	if err != nil {
		t.Fatalf("Generate hex: %v", err)
	}
	if len(val) != 64 {
		t.Errorf("len = %d, want 64", len(val))
	}
}
