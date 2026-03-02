package openapi

import "testing"

func TestEdgeCases_OpenAPI(t *testing.T) {
	t.Run("Edge01_status_202_should_be_accepted", func(t *testing.T) {
		if got := statusDescription(202); got != "Accepted" {
			t.Fatalf("expected 202 to map to 'Accepted', got %q", got)
		}
	})

	t.Run("Edge02_status_205_should_be_reset_content", func(t *testing.T) {
		if got := statusDescription(205); got != "Reset Content" {
			t.Fatalf("expected 205 to map to 'Reset Content', got %q", got)
		}
	})

	t.Run("Edge03_status_206_should_be_partial_content", func(t *testing.T) {
		if got := statusDescription(206); got != "Partial Content" {
			t.Fatalf("expected 206 to map to 'Partial Content', got %q", got)
		}
	})

	t.Run("Edge04_status_409_should_be_conflict", func(t *testing.T) {
		if got := statusDescription(409); got != "Conflict" {
			t.Fatalf("expected 409 to map to 'Conflict', got %q", got)
		}
	})

	t.Run("Edge05_status_410_should_be_gone", func(t *testing.T) {
		if got := statusDescription(410); got != "Gone" {
			t.Fatalf("expected 410 to map to 'Gone', got %q", got)
		}
	})

	t.Run("Edge06_status_413_should_be_payload_too_large", func(t *testing.T) {
		if got := statusDescription(413); got != "Payload Too Large" {
			t.Fatalf("expected 413 to map to 'Payload Too Large', got %q", got)
		}
	})

	t.Run("Edge07_status_415_should_be_unsupported_media_type", func(t *testing.T) {
		if got := statusDescription(415); got != "Unsupported Media Type" {
			t.Fatalf("expected 415 to map to 'Unsupported Media Type', got %q", got)
		}
	})

	t.Run("Edge08_status_429_should_be_too_many_requests", func(t *testing.T) {
		if got := statusDescription(429); got != "Too Many Requests" {
			t.Fatalf("expected 429 to map to 'Too Many Requests', got %q", got)
		}
	})

	t.Run("Edge09_convert_path_without_leading_slash", func(t *testing.T) {
		if got := convertPath("users/:id"); got != "/users/{id}" {
			t.Fatalf("expected convertPath to normalize leading slash, got %q", got)
		}
	})

	t.Run("Edge10_convert_path_with_duplicate_slashes", func(t *testing.T) {
		if got := convertPath("//users//:id//"); got != "/users/{id}" {
			t.Fatalf("expected convertPath to collapse duplicate/trailing slashes, got %q", got)
		}
	})
}
