package server

import (
	"github.com/impishMD/jeh/db"
	"github.com/impishMD/jeh/pkg/metrics"
	"github.com/impishMD/jeh/pro_interfaces"
	"github.com/impishMD/jeh/util"
)

func NewAuditExporter(_ db.Store, _ *util.AuditConfig, _ pro_interfaces.AuditExportLeaser, _ *metrics.Metrics) pro_interfaces.AuditExporter {
	return auditExporterStub{}
}

type auditExporterStub struct{}

func (auditExporterStub) Start() error             { return nil }
func (auditExporterStub) Stop()                    {}
func (auditExporterStub) DestinationIDs() []string { return []string{} }
