package slack

import (
	"context"

	"github.com/dakaneye/kora/internal/auth"
	slackauth "github.com/dakaneye/kora/internal/auth/slack"
)

// mockAuthProvider implements auth.AuthProvider for testing.
type mockAuthProvider struct {
	credential      auth.Credential
	authenticateErr error
	getCredErr      error
	service         auth.Service
	authenticated   bool
}

func newMockAuthProvider() *mockAuthProvider {
	// Create a mock credential with valid format
	cred, _ := slackauth.NewSlackToken("xoxp-test-token-12345")
	return &mockAuthProvider{
		service:       auth.ServiceSlack,
		authenticated: true,
		credential:    cred,
	}
}

func (m *mockAuthProvider) Service() auth.Service {
	return m.service
}

func (m *mockAuthProvider) Authenticate(ctx context.Context) error {
	if m.authenticateErr != nil {
		return m.authenticateErr
	}
	if !m.authenticated {
		return auth.ErrNotAuthenticated
	}
	return nil
}

func (m *mockAuthProvider) GetCredential(ctx context.Context) (auth.Credential, error) {
	if m.getCredErr != nil {
		return nil, m.getCredErr
	}
	if !m.authenticated {
		return nil, auth.ErrNotAuthenticated
	}
	return m.credential, nil
}

func (m *mockAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return m.authenticated
}
