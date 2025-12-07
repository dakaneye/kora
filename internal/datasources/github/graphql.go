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
	// The -f flag passes the input as a field in the request body
	out, err := c.cred.ExecuteAPI(ctx, "graphql",
		"-f", fmt.Sprintf("query=%s", query),
		"-f", fmt.Sprintf("variables=%s", mustMarshalVariables(variables)),
	)
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

// mustMarshalVariables marshals variables to JSON, returning "{}" on error.
func mustMarshalVariables(vars map[string]any) string {
	if len(vars) == 0 {
		return "{}"
	}
	b, err := json.Marshal(vars)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// PRQuery is the GraphQL query for fetching rich PR context.
// This query fetches all metadata fields defined in EFA 0001.
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
        }
      }

      reviewThreads(first: 50) {
        nodes {
          isResolved
        }
      }

      comments(first: 1) {
        totalCount
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
const SearchPRsQuery = `
query SearchPRs($query: String!, $first: Int!) {
  search(query: $query, type: ISSUE, first: $first) {
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
const SearchIssuesQuery = `
query SearchIssues($query: String!, $first: Int!) {
  search(query: $query, type: ISSUE, first: $first) {
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
