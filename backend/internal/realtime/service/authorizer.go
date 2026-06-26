package service

import (
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type Authorizer interface {
	AuthorizeLog(principal Principal, log sharedDomain.ProcessedLogEvent) bool
	AuthorizeAlert(principal Principal, alert sharedDomain.AlertEvent) bool
}

type AllowAllAuthorizer struct{}

func (AllowAllAuthorizer) AuthorizeLog(
	principal Principal,
	log sharedDomain.ProcessedLogEvent,
) bool {
	return true
}

func (AllowAllAuthorizer) AuthorizeAlert(
	principal Principal,
	alert sharedDomain.AlertEvent,
) bool {
	return true
}
