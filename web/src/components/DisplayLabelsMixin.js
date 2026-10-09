import { USER_ROLES } from '@/lib/constants';

const STATUS_KEYS = {
  waiting: 'status_waiting',
  pending: 'workflowNodePending',
  starting: 'status_starting',
  running: 'running',
  approval: 'workflowApprovalPending',
  approved: 'status_approved',
  confirmed: 'status_confirmed',
  waiting_confirmation: 'status_waiting_confirmation',
  rejected: 'status_rejected',
  success: 'status_success',
  error: 'status_failed',
  failed: 'status_failed',
  stopping: 'status_stopping',
  stopped: 'status_stopped',
  skipped: 'uiSkipped',
};

export default {
  methods: {
    viewTitle(view) {
      return view.type === 'all' && view.title === 'All' ? this.$t('all') : view.title;
    },
    inventoryTypeTitle(type) {
      const keys = { static: 'staticInventory', 'static-yaml': 'staticYamlInventory', file: 'uiFile' };
      if (keys[type]) return this.$t(keys[type]);
      if (['terraform-workspace', 'tofu-workspace'].includes(type)) {
        return this.$t('appWorkspace', { app: type === 'tofu-workspace' ? 'OpenTofu' : 'Terraform' });
      }
      return type;
    },
    keyTypeTitle(type) {
      const keys = {
        ssh: 'keyFormSshKey',
        login_password: 'keyFormLoginPassword',
        none: 'keyFormNone',
        string: 'uiString',
        object: 'keyVariableSet',
      };
      return keys[type] ? this.$t(keys[type]) : type;
    },
    storageTypeTitle(type) {
      return {
        vault: 'HashiCorp Vault',
        dvls: 'Devolutions Server',
        aws_sm: 'AWS Secrets Manager',
        azure_kv: 'Azure Key Vault',
      }[type] || type;
    },
    integrationValueTitle(value) {
      const keys = {
        body: 'uiBody',
        header: 'uiHeader',
        string: 'uiString',
        contains: 'uiContains',
        environment: 'uiVariables',
        task: 'uiTaskParams',
      };
      if (keys[value]) return this.$t(keys[value]);
      return { equals: '==', unequals: '!=', json: 'JSON' }[value] || value || '—';
    },
    statusTitle(status) {
      return STATUS_KEYS[status] ? this.$t(STATUS_KEYS[status]) : status;
    },
    roleTitle(role) {
      const slug = typeof role === 'string' ? role : role?.slug;
      const builtin = USER_ROLES.find((item) => item.slug === slug);
      return builtin ? this.$t(builtin.titleKey) : (role?.name || slug);
    },
  },
};
