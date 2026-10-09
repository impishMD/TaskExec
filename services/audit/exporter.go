package audit

// Exporter is the optional delivery boundary for audit events. The database
// recorder works independently; no external exporter is implemented yet.
type Exporter interface {
	Start() error
	Stop()
	DestinationIDs() []string
}

type localOnlyExporter struct{}

func (localOnlyExporter) Start() error             { return nil }
func (localOnlyExporter) Stop()                    {}
func (localOnlyExporter) DestinationIDs() []string { return []string{} }
