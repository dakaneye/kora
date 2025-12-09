// Package github provides GraphQL client infrastructure for rich PR/Issue context.
// Ground truth defined in specs/efas/0001-event-model.md (metadata fields)
// and specs/efas/0002-auth-provider.md (credential delegation).
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GraphQLClient executes GitHub GraphQL queries via gh CLI delegation.
//
// SECURITY: All API calls are delegated to the credential's ExecuteAPI method.
// This client NEVER sees or handles GitHub tokens directly.
// See EFA 0002 for credential security requirements.
type GraphQLClient struct {
	cred githubCredential
}

// NewGraphQLClient creates a GraphQL client using the provided credential.
func NewGraphQLClient(cred githubCredential) *GraphQLClient {
	return &GraphQLClient{cred: cred}
}

// GraphQLError represents an error returned by the GitHub GraphQL API.
type GraphQLError struct {
	Message   string   `json:"message"`
	Type      string   `json:"type"`
	Path      []string `json:"path"`
	Locations []struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"locations"`
}

// GraphQLResponse is the standard wrapper for GraphQL responses.
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors"`
}

// Execute runs a GraphQL query and returns the raw data portion.
// Variables are passed as a JSON-encoded map.
//
// Example:
//
//	data, err := client.Execute(ctx, `
//	  query($owner: String!, $repo: String!, $number: Int!) {
//	    repository(owner: $owner, name: $repo) {
//	      pullRequest(number: $number) {
//	        title
//	        state
//	      }
//	    }
//	  }
//	`, map[string]any{"owner": "org", "repo": "repo", "number": 123})
func (c *GraphQLClient) Execute(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	// Execute via gh api graphql
	// Per gh documentation: all fields other than "query" and "operationName"
	// are interpreted as GraphQL variables.
	// Use -f for string values, -F for typed values (int, bool, etc.)
	args := []string{"-f", fmt.Sprintf("query=%s", query)}

	// Add each variable as a separate flag
	for k, v := range variables {
		switch val := v.(type) {
		case string:
			// -f for string values (no type conversion)
			args = append(args, "-f", fmt.Sprintf("%s=%s", k, val))
		case int, int64, float64, bool:
			// -F for typed values (gh does magic type conversion)
			args = append(args, "-F", fmt.Sprintf("%s=%v", k, val))
		default:
			// For complex types, serialize to JSON and use -f
			jsonVal, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("marshal variable %s: %w", k, err)
			}
			args = append(args, "-f", fmt.Sprintf("%s=%s", k, string(jsonVal)))
		}
	}

	out, err := c.cred.ExecuteAPI(ctx, "graphql", args...)
	if err != nil {
		// Check for common error patterns
		// SECURITY: Don't include request body in errors to prevent information disclosure
		errStr := err.Error()
		if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "API rate limit") {
			return nil, fmt.Errorf("rate limited: %w", err)
		}
		return nil, fmt.Errorf("execute graphql: %w", err)
	}

	// Parse the response
	var resp GraphQLResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse graphql response: %w", err)
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		return nil, &GraphQLErrorList{Errors: resp.Errors}
	}

	return resp.Data, nil
}

// GraphQLErrorList wraps multiple GraphQL errors.
type GraphQLErrorList struct {
	Errors []GraphQLError
}

func (e *GraphQLErrorList) Error() string {
	if len(e.Errors) == 0 {
		return "unknown graphql error"
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("graphql error: %s", e.Errors[0].Message)
	}
	msgs := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		msgs = append(msgs, err.Message)
	}
	return fmt.Sprintf("graphql errors: %s", strings.Join(msgs, "; "))
}

// IsNotFound checks if the error indicates a resource was not found.
func (e *GraphQLErrorList) IsNotFound() bool {
	for _, err := range e.Errors {
		if err.Type == "NOT_FOUND" {
			return true
		}
	}
	return false
}

// PRQuery is the GraphQL query for fetching rich PR context.
// This query fetches all metadata fields defined in EFA 0001.
//
// NOTE: This query fetches comment and review bodies (up to 50 each) to support
// @mention detection for pr_comment_mention events. This increases payload size
// but is necessary for detecting user mentions in PR discussions.
const PRQuery = `
query PRContext($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      number
      title
      state
      isDraft
      mergeable
      url
      createdAt
      updatedAt

      author {
        login
      }

      assignees(first: 10) {
        nodes {
          login
        }
      }

      labels(first: 20) {
        nodes {
          name
        }
      }

      milestone {
        title
      }

      body

      headRefName
      baseRefName

      additions
      deletions
      changedFiles

      files(first: 100) {
        nodes {
          path
          additions
          deletions
        }
      }

      reviewRequests(first: 20) {
        nodes {
          requestedReviewer {
            ... on User {
              __typename
              login
            }
            ... on Team {
              __typename
              slug
              organization {
                login
              }
            }
          }
        }
      }

      reviews(first: 50) {
        nodes {
          author {
            login
          }
          state
          body
          createdAt
        }
      }

      reviewThreads(first: 50) {
        nodes {
          isResolved
        }
      }

      comments(first: 50) {
        totalCount
        nodes {
          author {
            login
          }
          body
          createdAt
          updatedAt
        }
      }

      commits(last: 1) {
        nodes {
          commit {
            statusCheckRollup {
              state
              contexts(first: 50) {
                nodes {
                  ... on CheckRun {
                    __typename
                    name
                    status
                    conclusion
                  }
                  ... on StatusContext {
                    __typename
                    context
                    state
                  }
                }
              }
            }
          }
        }
      }

      closingIssuesReferences(first: 10) {
        nodes {
          url
        }
      }
    }
  }
}
`

// IssueQuery is the GraphQL query for fetching rich Issue context.
// This query fetches all metadata fields defined in EFA 0001.
const IssueQuery = `
query IssueContext($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      number
      title
      state
      url
      createdAt
      updatedAt

      author {
        login
      }

      assignees(first: 10) {
        nodes {
          login
        }
      }

      labels(first: 20) {
        nodes {
          name
        }
      }

      milestone {
        title
      }

      body

      comments(first: 10) {
        totalCount
        nodes {
          author {
            login
          }
          body
          createdAt
        }
      }

      reactions {
        totalCount
      }

      reactionGroups {
        content
        users {
          totalCount
        }
      }

      timelineItems(first: 20, itemTypes: [ASSIGNED_EVENT, LABELED_EVENT, MENTIONED_EVENT, CROSS_REFERENCED_EVENT]) {
        nodes {
          ... on AssignedEvent {
            __typename
            createdAt
            actor {
              login
            }
          }
          ... on LabeledEvent {
            __typename
            createdAt
            actor {
              login
            }
            label {
              name
            }
          }
          ... on MentionedEvent {
            __typename
            createdAt
          }
          ... on CrossReferencedEvent {
            __typename
            createdAt
            source {
              ... on PullRequest {
                url
              }
            }
          }
        }
      }
    }
  }
}
`

// SearchPRsQuery searches for PRs matching criteria and returns basic info.
// Use PRQuery to fetch full context for each PR.
// Note: Variable is named $searchQuery to avoid conflict with gh's reserved "query" field.
// Supports pagination via $after cursor.
const SearchPRsQuery = `
query SearchPRs($searchQuery: String!, $first: Int!, $after: String) {
  search(query: $searchQuery, type: ISSUE, first: $first, after: $after) {
    issueCount
    nodes {
      ... on PullRequest {
        number
        title
        url
        updatedAt
        repository {
          nameWithOwner
        }
        author {
          login
        }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
`

// SearchIssuesQuery searches for issues matching criteria and returns basic info.
// Use IssueQuery to fetch full context for each issue.
// Note: Variable is named $searchQuery to avoid conflict with gh's reserved "query" field.
// Supports pagination via $after cursor.
const SearchIssuesQuery = `
query SearchIssues($searchQuery: String!, $first: Int!, $after: String) {
  search(query: $searchQuery, type: ISSUE, first: $first, after: $after) {
    issueCount
    nodes {
      ... on Issue {
        number
        title
        url
        updatedAt
        repository {
          nameWithOwner
        }
        author {
          login
        }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
`
