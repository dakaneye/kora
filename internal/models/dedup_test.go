package models

import (
	"reflect"
	"testing"
	"time"
)

func TestDeduplicateEvents(t *testing.T) {
	now := time.Now().UTC()

	//nolint:govet // test struct field order prioritizes readability over alignment
	tests := []struct {
		name     string
		events   []Event
		wantLen  int
		wantRels map[string][]string  // URL -> expected relationships
		wantType map[string]EventType // URL -> expected type (from primary)
	}{
		{
			name:    "empty input returns empty output",
			events:  nil,
			wantLen: 0,
		},
		{
			name:    "empty slice returns empty slice",
			events:  []Event{},
			wantLen: 0,
		},
		{
			name: "single event unchanged",
			events: []Event{
				{
					URL:      "https://github.com/org/repo/pull/1",
					Type:     EventTypePRReview,
					Priority: PriorityHigh,
					Metadata: map[string]any{"user_relationships": []string{"reviewer"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/1": {"reviewer"},
			},
		},
		{
			name: "no duplicates preserves all events",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/1",
					Type:      EventTypePRReview,
					Title:     "PR 1",
					Source:    SourceGitHub,
					Author:    Person{Username: "user1"},
					Timestamp: now,
					Priority:  PriorityHigh,
				},
				{
					URL:       "https://github.com/org/repo/pull/2",
					Type:      EventTypePRReview,
					Title:     "PR 2",
					Source:    SourceGitHub,
					Author:    Person{Username: "user2"},
					Timestamp: now,
					Priority:  PriorityMedium,
				},
			},
			wantLen: 2,
		},
		{
			name: "merge two events same URL combines relationships",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/123",
					Type:      EventTypePRReview,
					Title:     "Review PR",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  map[string]any{"user_relationships": []string{"reviewer"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/123",
					Type:      EventTypePRCodeowner,
					Title:     "Codeowner PR",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium,
					Metadata:  map[string]any{"user_relationships": []string{"codeowner"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/123": {"codeowner", "reviewer"}, // sorted alphabetically
			},
			wantType: map[string]EventType{
				"https://github.com/org/repo/pull/123": EventTypePRReview, // higher priority
			},
		},
		{
			name: "three events merge selects highest priority as primary",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/456",
					Type:      EventTypePRMention,
					Title:     "Mentioned",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium, // 3
					Metadata:  map[string]any{"user_relationships": []string{"mentioned"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/456",
					Type:      EventTypePRReview,
					Title:     "Review requested",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh, // 2 - highest
					Metadata:  map[string]any{"user_relationships": []string{"reviewer"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/456",
					Type:      EventTypePRCodeowner,
					Title:     "Codeowner",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium, // 3
					Metadata:  map[string]any{"user_relationships": []string{"codeowner"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/456": {"codeowner", "mentioned", "reviewer"}, // sorted alphabetically
			},
			wantType: map[string]EventType{
				"https://github.com/org/repo/pull/456": EventTypePRReview, // priority 2
			},
		},
		{
			name: "nil metadata handled gracefully",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/789",
					Type:      EventTypePRReview,
					Title:     "Review",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  nil,
				},
				{
					URL:       "https://github.com/org/repo/pull/789",
					Type:      EventTypePRCodeowner,
					Title:     "Codeowner",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium,
					Metadata:  map[string]any{"user_relationships": []string{"codeowner"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/789": {"codeowner"},
			},
		},
		{
			name: "missing user_relationships key handled",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/100",
					Type:      EventTypePRReview,
					Title:     "Review",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  map[string]any{"repo": "org/repo"},
				},
				{
					URL:       "https://github.com/org/repo/pull/100",
					Type:      EventTypePRCodeowner,
					Title:     "Codeowner",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium,
					Metadata:  map[string]any{"user_relationships": []string{"codeowner"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/100": {"codeowner"},
			},
		},
		{
			name: "duplicate relationships deduplicated",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/200",
					Type:      EventTypePRReview,
					Title:     "Review",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  map[string]any{"user_relationships": []string{"reviewer", "mentioned"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/200",
					Type:      EventTypePRMention,
					Title:     "Mentioned",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium,
					Metadata:  map[string]any{"user_relationships": []string{"mentioned"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/200": {"mentioned", "reviewer"}, // sorted alphabetically
			},
		},
		{
			name: "preserves order of first occurrence",
			events: []Event{
				{URL: "https://github.com/org/repo/pull/1", Type: EventTypePRReview, Title: "PR1", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
				{URL: "https://github.com/org/repo/pull/2", Type: EventTypePRReview, Title: "PR2", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
				{URL: "https://github.com/org/repo/pull/1", Type: EventTypePRMention, Title: "PR1b", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityMedium},
				{URL: "https://github.com/org/repo/pull/3", Type: EventTypePRReview, Title: "PR3", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
			},
			wantLen: 3, // PR1, PR2, PR3
		},
		{
			name: "handles []any from JSON unmarshal",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/300",
					Type:      EventTypePRReview,
					Title:     "Review",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  map[string]any{"user_relationships": []any{"reviewer"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/300",
					Type:      EventTypePRCodeowner,
					Title:     "Codeowner",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium,
					Metadata:  map[string]any{"user_relationships": []any{"codeowner"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/300": {"codeowner", "reviewer"}, // sorted alphabetically
			},
		},
		{
			name: "wrong type in user_relationships ignored",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/400",
					Type:      EventTypePRReview,
					Title:     "Review",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  map[string]any{"user_relationships": "not-a-slice"},
				},
				{
					URL:       "https://github.com/org/repo/pull/400",
					Type:      EventTypePRCodeowner,
					Title:     "Codeowner",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityMedium,
					Metadata:  map[string]any{"user_relationships": []string{"codeowner"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/400": {"codeowner"},
			},
		},
		{
			name: "same type same URL still merges relationships",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/500",
					Type:      EventTypePRReview,
					Title:     "Review 1",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh,
					Metadata:  map[string]any{"user_relationships": []string{"reviewer"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/500",
					Type:      EventTypePRReview, // same type
					Title:     "Review 2",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh, // same priority
					Metadata:  map[string]any{"user_relationships": []string{"team-reviewer"}},
				},
			},
			wantLen: 1,
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/500": {"reviewer", "team-reviewer"}, // sorted alphabetically
			},
		},
		{
			name: "critical priority wins over high",
			events: []Event{
				{
					URL:       "https://github.com/org/repo/pull/600",
					Type:      EventTypePRReview,
					Title:     "Review",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityHigh, // 2
					Metadata:  map[string]any{"user_relationships": []string{"reviewer"}},
				},
				{
					URL:       "https://github.com/org/repo/pull/600",
					Type:      EventTypePRAuthor,
					Title:     "Author - CI failing",
					Source:    SourceGitHub,
					Author:    Person{Username: "user"},
					Timestamp: now,
					Priority:  PriorityCritical, // 1 - highest
					Metadata:  map[string]any{"user_relationships": []string{"author"}},
				},
			},
			wantLen: 1,
			wantType: map[string]EventType{
				"https://github.com/org/repo/pull/600": EventTypePRAuthor, // critical priority
			},
			wantRels: map[string][]string{
				"https://github.com/org/repo/pull/600": {"author", "reviewer"}, // sorted alphabetically
			},
		},
		{
			name: "user authored PR in watched repo - author event wins over watched repo event",
			events: []Event{
				{
					URL:       "https://github.com/kubernetes/kubernetes/pull/123",
					Type:      EventTypePRAuthor,
					Title:     "My PR in watched repo",
					Source:    SourceGitHub,
					Author:    Person{Username: "me"},
					Timestamp: now,
					Priority:  PriorityMedium, // 3 - author
					Metadata: map[string]any{
						"user_relationships": []string{"author"},
						"repo":               "kubernetes/kubernetes",
						"number":             123,
					},
				},
				{
					URL:       "https://github.com/kubernetes/kubernetes/pull/123",
					Type:      EventTypePRClosed,
					Title:     "PR merged in watched repo",
					Source:    SourceGitHub,
					Author:    Person{Username: "me"},
					Timestamp: now,
					Priority:  PriorityInfo, // 5 - watched repo
					Metadata: map[string]any{
						"user_relationships": []string{},
						"watched_repo":       true,
						"repo":               "kubernetes/kubernetes",
						"number":             123,
					},
				},
			},
			wantLen: 1,
			wantType: map[string]EventType{
				"https://github.com/kubernetes/kubernetes/pull/123": EventTypePRAuthor, // author wins
			},
			wantRels: map[string][]string{
				"https://github.com/kubernetes/kubernetes/pull/123": {"author"}, // author relationship preserved
			},
		},
		{
			name: "reviewer on PR in watched repo - reviewer event wins over watched repo event",
			events: []Event{
				{
					URL:       "https://github.com/kubernetes/kubernetes/pull/456",
					Type:      EventTypePRReview,
					Title:     "Review requested on watched repo PR",
					Source:    SourceGitHub,
					Author:    Person{Username: "contributor"},
					Timestamp: now,
					Priority:  PriorityHigh, // 2 - reviewer
					Metadata: map[string]any{
						"user_relationships": []string{"direct_reviewer"},
						"repo":               "kubernetes/kubernetes",
						"number":             456,
					},
				},
				{
					URL:       "https://github.com/kubernetes/kubernetes/pull/456",
					Type:      EventTypePRClosed,
					Title:     "PR merged in watched repo",
					Source:    SourceGitHub,
					Author:    Person{Username: "contributor"},
					Timestamp: now,
					Priority:  PriorityInfo, // 5 - watched repo
					Metadata: map[string]any{
						"user_relationships": []string{},
						"watched_repo":       true,
						"repo":               "kubernetes/kubernetes",
						"number":             456,
					},
				},
			},
			wantLen: 1,
			wantType: map[string]EventType{
				"https://github.com/kubernetes/kubernetes/pull/456": EventTypePRReview, // reviewer wins
			},
			wantRels: map[string][]string{
				"https://github.com/kubernetes/kubernetes/pull/456": {"direct_reviewer"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateEvents(tt.events)

			if len(got) != tt.wantLen {
				t.Errorf("DeduplicateEvents() returned %d events, want %d", len(got), tt.wantLen)
			}

			// Check expected relationships
			for url, wantRels := range tt.wantRels {
				var found *Event
				for i := range got {
					if got[i].URL == url {
						found = &got[i]
						break
					}
				}
				if found == nil {
					t.Errorf("event with URL %q not found in result", url)
					continue
				}

				gotRels := extractUserRelationships(found)
				if !stringSliceEqual(gotRels, wantRels) {
					t.Errorf("event %q user_relationships = %v, want %v", url, gotRels, wantRels)
				}
			}

			// Check expected types
			for url, wantType := range tt.wantType {
				var found *Event
				for i := range got {
					if got[i].URL == url {
						found = &got[i]
						break
					}
				}
				if found == nil {
					t.Errorf("event with URL %q not found in result", url)
					continue
				}

				if found.Type != wantType {
					t.Errorf("event %q Type = %v, want %v", url, found.Type, wantType)
				}
			}
		})
	}
}

func TestDeduplicateEvents_WatchedRepoMetadataNotMerged(t *testing.T) {
	now := time.Now().UTC()

	// When user's own PR exists in a watched repo, the author event (priority 3) wins
	// over the watched repo event (priority 5). The watched_repo metadata from the
	// lower-priority event should NOT appear in the merged result.
	events := []Event{
		{
			URL:       "https://github.com/kubernetes/kubernetes/pull/789",
			Type:      EventTypePRAuthor,
			Title:     "My PR",
			Source:    SourceGitHub,
			Author:    Person{Username: "me"},
			Timestamp: now,
			Priority:  PriorityMedium, // 3 - author
			Metadata: map[string]any{
				"user_relationships": []string{"author"},
				"repo":               "kubernetes/kubernetes",
				"number":             789,
				"state":              "merged",
			},
		},
		{
			URL:       "https://github.com/kubernetes/kubernetes/pull/789",
			Type:      EventTypePRClosed,
			Title:     "PR merged in watched repo",
			Source:    SourceGitHub,
			Author:    Person{Username: "me"},
			Timestamp: now,
			Priority:  PriorityInfo, // 5 - watched repo
			Metadata: map[string]any{
				"user_relationships": []string{},
				"watched_repo":       true,
				"repo":               "kubernetes/kubernetes",
				"number":             789,
			},
		},
	}

	got := DeduplicateEvents(events)

	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}

	merged := got[0]

	// Verify author event metadata is preserved
	if merged.Metadata["repo"] != "kubernetes/kubernetes" {
		t.Errorf("repo metadata not preserved")
	}
	if merged.Metadata["state"] != "merged" {
		t.Errorf("state metadata not preserved from author event")
	}

	// Verify watched_repo metadata is NOT present (it was only on the lower-priority event)
	if _, hasWatchedRepo := merged.Metadata["watched_repo"]; hasWatchedRepo {
		t.Errorf("watched_repo metadata should not be present when author event wins")
	}

	// Verify type is from author event
	if merged.Type != EventTypePRAuthor {
		t.Errorf("Type = %v, want %v", merged.Type, EventTypePRAuthor)
	}
}

func TestDeduplicateEvents_PreservesOrder(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{URL: "a", Type: EventTypePRReview, Title: "A", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
		{URL: "b", Type: EventTypePRReview, Title: "B", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
		{URL: "c", Type: EventTypePRReview, Title: "C", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
		{URL: "a", Type: EventTypePRMention, Title: "A2", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityMedium}, // duplicate
		{URL: "d", Type: EventTypePRReview, Title: "D", Source: SourceGitHub, Author: Person{Username: "u"}, Timestamp: now, Priority: PriorityHigh},
	}

	got := DeduplicateEvents(events)

	wantOrder := []string{"a", "b", "c", "d"}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d events, want %d", len(got), len(wantOrder))
	}

	for i, e := range got {
		if e.URL != wantOrder[i] {
			t.Errorf("event[%d].URL = %q, want %q", i, e.URL, wantOrder[i])
		}
	}
}

func TestDeduplicateEvents_PrimaryEventMetadataPreserved(t *testing.T) {
	now := time.Now().UTC()

	// The primary event (higher priority) has additional metadata that should be preserved
	events := []Event{
		{
			URL:       "https://github.com/org/repo/pull/999",
			Type:      EventTypePRReview,
			Title:     "Review",
			Source:    SourceGitHub,
			Author:    Person{Username: "user"},
			Timestamp: now,
			Priority:  PriorityHigh, // This will be primary
			Metadata: map[string]any{
				"repo":               "org/repo",
				"number":             999,
				"ci_rollup":          "success",
				"user_relationships": []string{"reviewer"},
			},
		},
		{
			URL:       "https://github.com/org/repo/pull/999",
			Type:      EventTypePRCodeowner,
			Title:     "Codeowner",
			Source:    SourceGitHub,
			Author:    Person{Username: "user"},
			Timestamp: now,
			Priority:  PriorityMedium,
			Metadata: map[string]any{
				"repo":               "org/repo",
				"number":             999,
				"user_relationships": []string{"codeowner"},
			},
		},
	}

	got := DeduplicateEvents(events)

	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}

	merged := got[0]

	// Check primary metadata is preserved
	if merged.Metadata["ci_rollup"] != "success" {
		t.Errorf("ci_rollup not preserved from primary event")
	}
	if merged.Metadata["repo"] != "org/repo" {
		t.Errorf("repo not preserved from primary event")
	}

	// Check relationships are merged and sorted
	rels := extractUserRelationships(&merged)
	wantRels := []string{"codeowner", "reviewer"} // sorted alphabetically
	if !stringSliceEqual(rels, wantRels) {
		t.Errorf("user_relationships = %v, want %v", rels, wantRels)
	}
}

func TestExtractUserRelationships(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected []string
	}{
		{
			name:     "nil metadata",
			event:    Event{Metadata: nil},
			expected: nil,
		},
		{
			name:     "missing user_relationships key",
			event:    Event{Metadata: map[string]any{"repo": "test"}},
			expected: nil,
		},
		{
			name: "[]string type",
			event: Event{Metadata: map[string]any{
				"user_relationships": []string{"reviewer", "codeowner"},
			}},
			expected: []string{"reviewer", "codeowner"},
		},
		{
			name: "[]any type with strings",
			event: Event{Metadata: map[string]any{
				"user_relationships": []any{"reviewer", "codeowner"},
			}},
			expected: []string{"reviewer", "codeowner"},
		},
		{
			name: "[]any type with mixed content skips non-strings",
			event: Event{Metadata: map[string]any{
				"user_relationships": []any{"reviewer", 123, "codeowner", nil},
			}},
			expected: []string{"reviewer", "codeowner"},
		},
		{
			name: "wrong type returns nil",
			event: Event{Metadata: map[string]any{
				"user_relationships": "not-a-slice",
			}},
			expected: nil,
		},
		{
			name: "int type returns nil",
			event: Event{Metadata: map[string]any{
				"user_relationships": 42,
			}},
			expected: nil,
		},
		{
			name: "empty []string",
			event: Event{Metadata: map[string]any{
				"user_relationships": []string{},
			}},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserRelationships(&tt.event)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractUserRelationships() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// stringSliceEqual compares two string slices for equality.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
