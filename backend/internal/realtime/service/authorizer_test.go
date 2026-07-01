package service

import "testing"

func TestAllowAllAuthorizerAllowsLogsAndAlerts(t *testing.T) {
	authorizer := AllowAllAuthorizer{}
	principal := Principal{UserID: "user-1", Role: "engineer"}

	if !authorizer.AuthorizeLog(principal, sampleProcessedLog()) {
		t.Fatal("AuthorizeLog() = false, want true")
	}
	if !authorizer.AuthorizeAlert(principal, sampleAlert()) {
		t.Fatal("AuthorizeAlert() = false, want true")
	}
}

func TestRBACAuthorizerAllowsAdmin(t *testing.T) {
	authorizer := RBACAuthorizer{}
	principal := Principal{UserID: "user-1", Role: "admin"}

	if !authorizer.AuthorizeLog(principal, sampleProcessedLog()) {
		t.Fatal("AuthorizeLog() = false, want true")
	}
	if !authorizer.AuthorizeAlert(principal, sampleAlert()) {
		t.Fatal("AuthorizeAlert() = false, want true")
	}
}

func TestRBACAuthorizerFiltersEngineerApplications(t *testing.T) {
	authorizer := RBACAuthorizer{}
	allowed := Principal{
		UserID: "user-1",
		Role:   "engineer",
		Apps:   map[string]bool{"payment": true},
	}
	denied := Principal{
		UserID: "user-2",
		Role:   "engineer",
		Apps:   map[string]bool{"billing": true},
	}

	if !authorizer.AuthorizeLog(allowed, sampleProcessedLog()) {
		t.Fatal("allowed engineer log authorization = false, want true")
	}
	if authorizer.AuthorizeLog(denied, sampleProcessedLog()) {
		t.Fatal("denied engineer log authorization = true, want false")
	}
}
