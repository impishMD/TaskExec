package export

import "github.com/impishMD/taskexec/db"

// Keep a separate load of the groups: Environment must be restored before
// owned access keys, and its exporter is cleared before this second phase.
type EnvironmentBindingsExporter struct{ EnvironmentExporter }

func (e *EnvironmentBindingsExporter) getName() string { return EnvironmentBindings }
func (e *EnvironmentBindingsExporter) importDependsOn() []string {
	return []string{Project, Environment, AccessKey}
}
func (e *EnvironmentBindingsExporter) restore(store db.Store, exporter DataExporter, progress Progress) error {
	return e.restoreValues(store, exporter, progress, e)
}
func (e *EnvironmentBindingsExporter) restoreValue(val EntityObject[db.Environment], store db.Store, exporter DataExporter) error {
	if len(val.value.KeyBindings) == 0 && len(val.value.KeySources) == 0 && len(val.value.SecretExpressions) == 0 {
		return nil
	}
	project, err := exporter.getNewKeyInt(Project, GlobalScope, val.value.ProjectID)
	if err != nil {
		return err
	}
	id, err := exporter.getNewKeyInt(Environment, val.scope, val.value.ID)
	if err != nil {
		return err
	}
	env, err := store.GetEnvironment(project, id)
	if err != nil {
		return err
	}
	for _, binding := range val.value.KeyBindings {
		binding.KeyID, err = exporter.getNewKeyInt(AccessKey, val.scope, binding.KeyID)
		if err != nil {
			return err
		}
		env.KeyBindings = append(env.KeyBindings, binding)
	}
	for _, source := range val.value.KeySources {
		source.KeyID, err = exporter.getNewKeyInt(AccessKey, val.scope, source.KeyID)
		if err != nil {
			return err
		}
		env.KeySources = append(env.KeySources, source)
	}
	env.SecretExpressions = val.value.SecretExpressions
	return store.UpdateEnvironment(env)
}
