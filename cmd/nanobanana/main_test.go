package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"maragu.dev/clir"

	"maragu.dev/nanobanana/internal/nanobanana"
)

func TestGenerate(t *testing.T) {
	// Create a mock API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/generate" {
			t.Errorf("expected /generate, got %s", r.URL.Path)
		}

		// Return a mock image (1x1 pixel PNG) as base64
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Simple mock response - in reality this would be base64 encoded
		w.Write([]byte(`{"images":["iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="]}`))
	}))
	defer mockServer.Close()

	// Create a temporary directory for output
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.png")

	// Create a client pointing to the mock server
	client := nanobanana.NewClient("test-api-key").WithBaseURL(mockServer.URL)

	// Create router
	router := clir.NewRouter()
	router.RouteFunc("generate", generateHandler(client))

	// Run the command
	ctx := clir.Context{
		Args: []string{"generate", outputPath, "test prompt"},
		Ctx:  context.Background(),
		Out:  os.Stdout,
		Err:  os.Stderr,
	}

	err := router.Run(ctx)
	if err != nil {
		t.Fatalf("generate command failed: %v", err)
	}

	// Verify the output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("output file was not created: %s", outputPath)
	}
}
