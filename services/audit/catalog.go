package audit

import (
	"fmt"
	"reflect"
	"slices"
)

type Event struct {
	Kind Kind
	// Empty means success.
	Outcome   Outcome
	Reason    Reason
	Target    *Target
	ProjectID int
	Metadata  any
}

func (e Event) outcome() Outcome {
	if e.Outcome == "" {
		return OutcomeSuccess
	}
	return e.Outcome
}

type Entry struct {
	Kind    Kind
	Type    Type
	Reasons []Reason
	// Reasons allowed on a success whose follow-up step failed.
	Partial  []Reason
	Metadata any
}

func Catalog() []Entry {
	return []Entry{
		{Kind: AuthLogin, Type: TypeStart, Metadata: AuthMethodMetadata{}, Reasons: []Reason{
			ReasonInvalidCredentials, ReasonUserNotFound, ReasonMethodDisabled, ReasonInvalidState, ReasonProviderError, ReasonInternalError,
		}},
		{Kind: AuthLogout, Type: TypeEnd},
		{Kind: AuthMFAVerifyTOTP, Type: TypeInfo, Reasons: []Reason{ReasonInvalidPasscode}},
		{Kind: AuthMFAVerifyEmail, Type: TypeInfo, Reasons: []Reason{ReasonInvalidPasscode, ReasonCodeExpired, ReasonTooManyAttempts}},
		{Kind: AuthMFARecover, Type: TypeInfo, Reasons: []Reason{ReasonInvalidRecoveryCode}},
		{Kind: AuthAPITokenReject, Type: TypeDenied, Reasons: []Reason{ReasonTokenUnknown, ReasonTokenExpired}},
		{Kind: AuthAuthorizationDeny, Type: TypeDenied, Metadata: DenyMetadata{}, Reasons: []Reason{ReasonForbidden}},
		{Kind: AuthCSRFBlock, Type: TypeDenied, Metadata: DenyMetadata{}, Reasons: []Reason{ReasonCrossOrigin}},

		{Kind: IAMUserCreate, Type: TypeCreation, Metadata: UserCreateMetadata{}},
		{Kind: IAMUserUpdate, Type: TypeChange, Metadata: UserUpdateMetadata{}},
		{Kind: IAMUserDelete, Type: TypeDeletion},
		{Kind: IAMUserAutoProvision, Type: TypeCreation, Metadata: AuthMethodMetadata{}},
		{Kind: IAMUserPasswordChange, Type: TypeChange, Reasons: []Reason{ReasonInvalidCurrentPassword}},
		{Kind: IAMUserPasswordAdminReset, Type: TypeChange},
		{Kind: IAMMFAEnable, Type: TypeChange},
		{Kind: IAMMFADisable, Type: TypeChange},
		{Kind: IAMMFAViewQR, Type: TypeAccess},
		{Kind: IAMExternalIdentityLink, Type: TypeChange, Metadata: AuthMethodMetadata{}},
		{Kind: IAMExternalIdentityUnlink, Type: TypeChange, Metadata: AuthMethodMetadata{}},
		{Kind: IAMAPITokenCreate, Type: TypeCreation},
		{Kind: IAMAPITokenDelete, Type: TypeDeletion},
		{Kind: IAMProjectTokenCreate, Type: TypeCreation, Metadata: ProjectTokenMetadata{}},
		{Kind: IAMProjectTokenRevoke, Type: TypeChange},
		{Kind: IAMProjectTokenRotate, Type: TypeChange, Metadata: ProjectTokenMetadata{}},
		{Kind: IAMMembershipAdd, Type: TypeChange, Metadata: MembershipMetadata{}},
		{Kind: IAMMembershipRemove, Type: TypeChange, Metadata: MembershipMetadata{}, Reasons: []Reason{ReasonOwnerSelfChange}},
		{Kind: IAMProjectRoleChange, Type: TypeChange, Metadata: ProjectRoleMetadata{}, Reasons: []Reason{ReasonOwnerSelfChange}},
		{Kind: IAMRoleCreate, Type: TypeCreation, Metadata: RoleMetadata{}},
		{Kind: IAMRoleUpdate, Type: TypeChange, Metadata: RoleMetadata{}},
		{Kind: IAMRoleDelete, Type: TypeDeletion},
		{Kind: IAMProjectRoleDefinitionCreate, Type: TypeCreation, Metadata: RoleMetadata{}},
		{Kind: IAMProjectRoleDefinitionUpdate, Type: TypeChange, Metadata: RoleMetadata{}},
		{Kind: IAMProjectRoleDefinitionDelete, Type: TypeDeletion},
		{Kind: IAMTemplatePermissionCreate, Type: TypeCreation, Metadata: TemplatePermissionMetadata{}},
		{Kind: IAMTemplatePermissionUpdate, Type: TypeChange, Metadata: TemplatePermissionMetadata{}},
		{Kind: IAMTemplatePermissionDelete, Type: TypeDeletion, Metadata: TemplatePermissionMetadata{}},

		{Kind: ResourceProjectCreate, Type: TypeCreation, Metadata: ProjectCreateMetadata{}, Partial: []Reason{ReasonSetupFailed}},
		{Kind: ResourceProjectUpdate, Type: TypeChange},
		{Kind: ResourceProjectDelete, Type: TypeDeletion},
		{Kind: ResourceProjectBackupExport, Type: TypeAccess},
		{Kind: ResourceProjectBackupRestore, Type: TypeCreation, Metadata: BackupRestoreMetadata{}, Partial: []Reason{ReasonRestoreFailed}},
		{Kind: ResourceInventoryCreate, Type: TypeCreation},
		{Kind: ResourceInventoryUpdate, Type: TypeChange},
		{Kind: ResourceInventoryDelete, Type: TypeDeletion},
		{Kind: TerraformAliasCreate, Type: TypeCreation, Metadata: TerraformMetadata{}},
		{Kind: TerraformAliasUpdate, Type: TypeChange, Metadata: TerraformMetadata{}},
		{Kind: TerraformAliasDelete, Type: TypeDeletion, Metadata: TerraformMetadata{}},
		{Kind: TerraformStateWrite, Type: TypeChange, Metadata: TerraformMetadata{}},
		{Kind: TerraformStateReset, Type: TypeDeletion, Metadata: TerraformMetadata{}},
		{Kind: TerraformStateDelete, Type: TypeDeletion, Metadata: TerraformMetadata{}},
		{Kind: TerraformStateLock, Type: TypeChange, Metadata: TerraformMetadata{}},
		{Kind: TerraformStateUnlock, Type: TypeChange, Metadata: TerraformMetadata{}},
		{Kind: ResourceRepositoryCreate, Type: TypeCreation},
		{Kind: ResourceRepositoryUpdate, Type: TypeChange},
		{Kind: ResourceRepositoryDelete, Type: TypeDeletion},
		{Kind: ResourceTemplateCreate, Type: TypeCreation, Metadata: TemplateMetadata{}, Partial: []Reason{ReasonInventoryFailed}},
		{Kind: ResourceTemplateUpdate, Type: TypeChange, Metadata: TemplateMetadata{}},
		{Kind: ResourceTemplateDelete, Type: TypeDeletion},
		{Kind: ResourceTemplateAttachInventory, Type: TypeChange, Metadata: TemplateInventoryMetadata{}},
		{Kind: ResourceTemplateDetachInventory, Type: TypeChange, Metadata: TemplateInventoryMetadata{}},
		{Kind: ResourceTemplateSetDefaultInventory, Type: TypeChange, Metadata: TemplateInventoryMetadata{}},
		{Kind: ResourceScheduleCreate, Type: TypeCreation, Metadata: ScheduleMetadata{}},
		{Kind: ResourceScheduleUpdate, Type: TypeChange, Metadata: ScheduleMetadata{}},
		{Kind: ResourceScheduleDelete, Type: TypeDeletion},
		{Kind: ResourceScheduleActivate, Type: TypeChange, Metadata: ScheduleMetadata{}},
		{Kind: ResourceScheduleDeactivate, Type: TypeChange, Metadata: ScheduleMetadata{}},
		{Kind: ResourceIntegrationCreate, Type: TypeCreation, Metadata: IntegrationMetadata{}},
		{Kind: ResourceIntegrationUpdate, Type: TypeChange, Metadata: IntegrationMetadata{}},
		{Kind: ResourceIntegrationDelete, Type: TypeDeletion},
		{Kind: ResourceIntegrationMatcherCreate, Type: TypeCreation, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationMatcherUpdate, Type: TypeChange, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationMatcherDelete, Type: TypeDeletion, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationExtractorCreate, Type: TypeCreation, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationExtractorUpdate, Type: TypeChange, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationExtractorDelete, Type: TypeDeletion, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationAliasCreate, Type: TypeCreation, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceIntegrationAliasDelete, Type: TypeDeletion, Metadata: IntegrationPartMetadata{}},
		{Kind: ResourceHostConfigCreate, Type: TypeCreation, Metadata: HostConfigMetadata{}},
		{Kind: ResourceHostConfigUpdate, Type: TypeChange, Metadata: HostConfigMetadata{}},
		{Kind: ResourceHostConfigDelete, Type: TypeDeletion},
		{Kind: ResourceWorkflowCreate, Type: TypeCreation, Metadata: WorkflowMetadata{}},
		{Kind: ResourceWorkflowUpdate, Type: TypeChange, Metadata: WorkflowMetadata{}},
		{Kind: ResourceWorkflowDelete, Type: TypeDeletion, Metadata: WorkflowMetadata{}},
		{Kind: WorkflowStart, Type: TypeStart, Metadata: WorkflowMetadata{}},
		{Kind: WorkflowStop, Type: TypeEnd, Metadata: WorkflowMetadata{}},
		{Kind: WorkflowComplete, Type: TypeEnd, Metadata: WorkflowMetadata{}},
		{Kind: WorkflowApprovalApprove, Type: TypeChange, Metadata: WorkflowMetadata{}},
		{Kind: WorkflowApprovalReject, Type: TypeChange, Metadata: WorkflowMetadata{}},
		{Kind: WorkflowApprovalTimeout, Type: TypeChange, Metadata: WorkflowMetadata{}},
		{Kind: ResourceEnvironmentCreate, Type: TypeCreation, Metadata: EnvironmentMetadata{}, Partial: []Reason{ReasonSecretFailed}},
		{Kind: ResourceEnvironmentUpdate, Type: TypeChange, Metadata: EnvironmentMetadata{}, Partial: []Reason{ReasonSecretFailed}},
		{Kind: ResourceEnvironmentDelete, Type: TypeDeletion, Metadata: DeleteMetadata{}, Partial: []Reason{ReasonSecretFailed}},
		{Kind: ResourceEnvironmentSync, Type: TypeChange},

		{Kind: SecretCredentialCreate, Type: TypeCreation, Metadata: CredentialMetadata{}},
		{Kind: SecretCredentialUpdate, Type: TypeChange, Metadata: CredentialMetadata{}},
		{Kind: SecretCredentialDelete, Type: TypeDeletion},
		{Kind: SecretStorageCreate, Type: TypeCreation, Metadata: SecretStorageMetadata{}},
		{Kind: SecretStorageUpdate, Type: TypeChange, Metadata: SecretStorageMetadata{}},
		{Kind: SecretStorageDelete, Type: TypeDeletion, Metadata: DeleteMetadata{}},
		{Kind: SecretStorageSync, Type: TypeChange},

		{Kind: TaskExecutionCreate, Type: TypeCreation, Metadata: TaskCreateMetadata{}},
		{Kind: TaskExecutionComplete, Type: TypeEnd, Metadata: TaskCompleteMetadata{}},
		{Kind: TaskApprovalRequest, Type: TypeInfo, Metadata: TaskMetadata{}},
		{Kind: TaskApprovalApprove, Type: TypeChange, Metadata: TaskMetadata{}},
		{Kind: TaskApprovalReject, Type: TypeChange, Metadata: TaskMetadata{}},
		{Kind: TaskControlStop, Type: TypeChange, Metadata: TaskMetadata{}},
		{Kind: TaskControlForceStop, Type: TypeChange, Metadata: TaskMetadata{}},
		{Kind: TaskControlStopAll, Type: TypeChange},
		{Kind: TaskHistoryDelete, Type: TypeDeletion, Metadata: TaskMetadata{}},

		{Kind: RunnerLifecycleCreate, Type: TypeCreation},
		{Kind: RunnerLifecycleUpdate, Type: TypeChange},
		{Kind: RunnerLifecycleDelete, Type: TypeDeletion},
		{Kind: RunnerLifecycleEnable, Type: TypeChange},
		{Kind: RunnerLifecycleDisable, Type: TypeChange},
		{Kind: RunnerLifecycleRegister, Type: TypeCreation, Metadata: RunnerRegisterMetadata{}, Reasons: []Reason{ReasonInvalidRegistrationToken}},
		{Kind: RunnerLifecycleUnregister, Type: TypeDeletion},
		{Kind: RunnerCredentialRotate, Type: TypeChange},
		{Kind: RunnerCacheClear, Type: TypeDeletion},
		{Kind: RunnerProgressReject, Type: TypeDenied, Reasons: []Reason{ReasonInvalidStatus}},

		{Kind: SystemSettingsUpdate, Type: TypeChange, Metadata: SettingsMetadata{}},

		{Kind: AuditLifecycleStart, Type: TypeStart, Metadata: LifecycleMetadata{}},
		{Kind: AuditRetentionDelete, Type: TypeDeletion, Metadata: RetentionMetadata{}},
	}
}

func Lookup(kind Kind) (Entry, bool) {
	for _, entry := range Catalog() {
		if entry.Kind == kind {
			return entry, true
		}
	}
	return Entry{}, false
}

func Known(kind Kind) bool {
	_, ok := Lookup(kind)
	return ok
}

func Validate(event Event) error {
	entry, ok := Lookup(event.Kind)
	if !ok {
		return fmt.Errorf("unknown audit kind %q", event.Kind)
	}

	switch event.outcome() {
	case OutcomeSuccess:
		if entry.Type == TypeDenied {
			return fmt.Errorf("%s is always a failure", event.Kind)
		}
		if event.Reason != ReasonNone && !slices.Contains(entry.Partial, event.Reason) {
			return fmt.Errorf("%s success cannot have reason %q", event.Kind, event.Reason)
		}
	case OutcomeFailure:
		if len(entry.Reasons) == 0 {
			return fmt.Errorf("%s cannot be recorded as a failure", event.Kind)
		}
		if !slices.Contains(entry.Reasons, event.Reason) {
			return fmt.Errorf("%s: reason %q is not allowed", event.Kind, event.Reason)
		}
	default:
		return fmt.Errorf("%s: invalid outcome %q", event.Kind, event.Outcome)
	}

	if reflect.TypeOf(event.Metadata) != reflect.TypeOf(entry.Metadata) {
		return fmt.Errorf("%s: metadata must be %T, got %T", event.Kind, entry.Metadata, event.Metadata)
	}

	return nil
}
