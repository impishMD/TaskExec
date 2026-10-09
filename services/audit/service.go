package audit

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
)

type Service struct {
	recorder Recorder
	exporter Exporter
	trusted  []netip.Prefix
	enabled  bool

	stopRetention context.CancelFunc
	retentionDone chan struct{}
}

func StartService(
	store db.AuditEventManager,
	conf *util.AuditConfig,
	nodeID string,
	exporter Exporter,
) (*Service, error) {
	if !conf.IsEnabled() {
		return &Service{recorder: Nop{}}, nil
	}

	if exporter == nil {
		exporter = localOnlyExporter{}
	}
	trusted, err := conf.TrustedProxies()
	if err != nil {
		return nil, err
	}

	// Started first, so it exports the start event.
	if err = exporter.Start(); err != nil {
		return nil, fmt.Errorf("audit export: %w", err)
	}

	s := &Service{
		recorder: NewRecorder(store, Options{InstanceID: conf.InstanceID, NodeID: nodeID}),
		exporter: exporter,
		trusted:  trusted,
		enabled:  true,
	}

	destinations := exporter.DestinationIDs()
	if destinations == nil {
		destinations = []string{}
	}
	s.recorder.Record(WithActor(context.Background(), SystemActor(ComponentServer)), Event{
		Kind:     AuditLifecycleStart,
		Metadata: LifecycleMetadata{Destinations: destinations},
	})

	if conf.RetentionDays > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		s.stopRetention, s.retentionDone = cancel, make(chan struct{})
		go runRetention(ctx, store, s.recorder, conf.RetentionDays, s.retentionDone)
	}

	return s, nil
}

func (s *Service) Stop() {
	if s.stopRetention != nil {
		s.stopRetention()
		<-s.retentionDone
	}
	if s.enabled {
		s.exporter.Stop()
	}
}

func (s *Service) Recorder() Recorder {
	return s.recorder
}

func (s *Service) Wrap(h http.Handler) http.Handler {
	if !s.enabled {
		return h
	}
	return RequestMiddleware(s.trusted)(h)
}
