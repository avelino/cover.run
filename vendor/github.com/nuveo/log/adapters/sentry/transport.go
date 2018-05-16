package sentry

import "github.com/getsentry/raven-go"

type MockTransport struct {
	Count int
}

func (m *MockTransport) Send(url, authHeader string, packet *raven.Packet) error {
	m.Count++
	return nil
}
