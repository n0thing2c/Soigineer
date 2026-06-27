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
