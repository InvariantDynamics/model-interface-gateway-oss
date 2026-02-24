package mig

import (
	"sort"
	"strings"
	"time"
)

type ConnectionFilters struct {
	TenantID string
	Kind     string
	Protocol string
}

func (s *Service) RegisterConnection(conn ConnectionSnapshot) (string, func()) {
	if conn.ID == "" {
		conn.ID = "conn-" + newMessageID()[:12]
	}
	if conn.StartedAt == "" {
		conn.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if conn.Meta == nil {
		conn.Meta = map[string]interface{}{}
	}
	s.mu.Lock()
	if s.connections == nil {
		s.connections = map[string]ConnectionSnapshot{}
	}
	s.connections[conn.ID] = conn
	s.mu.Unlock()

	return conn.ID, func() {
		s.mu.Lock()
		delete(s.connections, conn.ID)
		s.mu.Unlock()
	}
}

func (s *Service) Connections(filters ConnectionFilters) ConnectionsResponse {
	tenant := strings.TrimSpace(filters.TenantID)
	kind := strings.TrimSpace(filters.Kind)
	protocol := strings.TrimSpace(filters.Protocol)

	s.mu.RLock()
	defer s.mu.RUnlock()

	connections := make([]ConnectionSnapshot, 0, len(s.connections))
	summary := ConnectionSummary{
		ByProtocol:        map[string]int{},
		ByKind:            map[string]int{},
		ByTenant:          map[string]int{},
		NATSBindingActive: s.natsBinding != nil,
	}

	for _, conn := range s.connections {
		if tenant != "" && conn.TenantID != tenant {
			continue
		}
		if kind != "" && conn.Kind != kind {
			continue
		}
		if protocol != "" && conn.Protocol != protocol {
			continue
		}
		connections = append(connections, conn)
		summary.Total++
		if conn.Protocol != "" {
			summary.ByProtocol[conn.Protocol]++
		}
		if conn.Kind != "" {
			summary.ByKind[conn.Kind]++
		}
		if conn.TenantID != "" {
			summary.ByTenant[conn.TenantID]++
		}
	}

	sort.Slice(connections, func(i, j int) bool {
		return connections[i].StartedAt > connections[j].StartedAt
	})

	return ConnectionsResponse{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Summary:      summary,
		Connections:  connections,
		FilterTenant: tenant,
		FilterKind:   kind,
		FilterProto:  protocol,
	}
}
