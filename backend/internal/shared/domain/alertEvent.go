package domain

type AlertEvent struct {
}

func ToAlertEvent(event RawLogEvent) AlertEvent {
	return AlertEvent{}
}
