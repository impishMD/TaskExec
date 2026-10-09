package features

type Features struct {
	ProjectRunners            bool `json:"project_runners"`
	TerraformBackend          bool `json:"terraform_backend"`
	TaskSummary               bool `json:"task_summary"`
	SecretStorages            bool `json:"secret_storages"`
	SecretStorageManagement   bool `json:"secret_storage_management"`
	SecretStorageManagementEx bool `json:"secret_storage_management_ex"`
	CustomRolesManagement     bool `json:"custom_roles_management"`
	Workflows                 bool `json:"workflows"`
	DockerExecutor            bool `json:"docker_executor"`
	K8sExecutor               bool `json:"k8s_executor"`
}

// Available reports implemented capabilities. Deferred providers remain unavailable.
func Available() Features {
	return Features{ProjectRunners: true, CustomRolesManagement: true, TaskSummary: true, DockerExecutor: true, SecretStorages: true, SecretStorageManagement: true, TerraformBackend: true, Workflows: true}
}
