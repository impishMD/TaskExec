<template>
  <v-form ref="form" lazy-validation v-model="formValid" v-if="item != null">
    <v-alert :value="formError" type="error" class="mb-6" dismissible>{{ formError }} </v-alert>

    <v-text-field
      v-model="item.name"
      :label="$t('name')"
      :rules="[(v) => !!v || $t('name_required')]"
      required
      :disabled="busy"
      outlined
      dense
    ></v-text-field>

    <v-text-field
      v-model="item.params.url"
      :label="$t('uiServerURL')"
      :disabled="busy"
      :rules="[(v) => !!v || $t('uiURLIsRequired')]"
      required
      data-testid="secretStorage-vaultURL"
      outlined
      dense
    ></v-text-field>

    <div class="VaultConnectionFields">
      <v-text-field v-model="item.params.mount" :label="$t('vaultKVMount')"
        :placeholder="defaultMount" :disabled="busy" outlined dense />
      <v-select v-model="item.params.kv_version" :items="[1, 2]"
        :label="$t('vaultKVVersion')" :disabled="busy" outlined dense
        data-testid="secretStorage-kvVersion" />
    </div>

    <h3 class="text-subtitle-1 mb-3">{{ $t('authentication') }}</h3>
    <div class="VaultAuthMethods" role="radiogroup" :aria-label="$t('authentication')">
      <button v-for="method in authMethods" :key="method.value" type="button" role="radio"
        :aria-checked="authMethod === method.value" :tabindex="authMethod === method.value ? 0 : -1"
        :disabled="busy"
        :class="{ 'VaultAuthMethods__selected': authMethod === method.value }"
        :data-testid="`vault-auth-${method.value}`" @click="selectMethod(method.value)"
        @keydown="navigateMethods($event, method.value)">
        <v-icon size="22">{{ method.icon }}</v-icon>
        <span>{{ method.text }}</span>
      </button>
    </div>
    <p class="text-body-2 text--secondary mt-3 mb-5">{{ methodHint }}</p>

    <div v-if="authMethod !== 'token'" class="VaultConnectionFields">
      <v-text-field v-model="item.params.auth_mount" :label="$t('vaultAuthMount')"
        :hint="$t('vaultAuthMountHint')" persistent-hint :disabled="busy"
        :placeholder="authMethod" outlined dense data-testid="vault-auth-mount" />
      <v-text-field v-if="['kubernetes', 'jwt', 'cert'].includes(authMethod)"
        v-model="item.params.auth_role" :label="$t('role')" :disabled="busy"
        :rules="authMethod === 'cert' ? [] : [(v) => !!v || $t('role_required')]"
        outlined dense data-testid="vault-auth-role" />
    </div>

    <VaultCredentialField v-for="field in credentialFields" :key="`${authMethod}-${field.name}`"
      v-model="item.credentials[field.name]" :field="field.name" :label="field.label"
      :multiline="field.multiline" :disabled="busy" />
    <v-checkbox v-if="authMethod === 'approle'" v-model="item.params.without_secret_id"
      :label="$t('vaultWithoutSecretID')" :disabled="busy" class="mt-0 mb-4"
      hide-details />

    <v-expansion-panels flat class="mb-6">
      <v-expansion-panel>
        <v-expansion-panel-header>{{ $t('advanced') }}</v-expansion-panel-header>
        <v-expansion-panel-content>
          <v-text-field v-model="item.params.namespace" :label="$t('uiNamespace')"
            :hint="$t('vaultNamespaceSupport')" persistent-hint :disabled="busy" outlined dense />
          <v-checkbox v-model="customCA" :label="$t('vaultCustomCA')" :disabled="busy" />
          <VaultCredentialField v-if="customCA" v-model="item.credentials.ca_cert"
            field="ca_cert" :label="$t('vaultCACertificate')" multiline :disabled="busy" />
        </v-expansion-panel-content>
      </v-expansion-panel>
    </v-expansion-panels>

    <div class="mb-6">
      <v-btn outlined color="primary" :loading="testing" :disabled="busy"
        @click="testAuthentication" data-testid="vault-test-auth">
        <v-icon left>mdi-connection</v-icon>{{ $t('vaultTestAuth') }}
      </v-btn>
      <v-alert v-if="testResult" :type="testResult.type" text class="mt-3 mb-0" role="status">
        {{ testResult.message }}
      </v-alert>
    </div>

    <v-checkbox
      v-model="item.readonly"
      :label="$t('uiReadOnly')"
      :disabled="busy"
      hide-details
      class="mt-0 mb-4"
    />

  </v-form>
</template>
<script>
import axios from 'axios';
import ItemFormBase from '@/components/ItemFormBase';
import VaultCredentialField from '@/components/VaultCredentialField.vue';
import { getErrorMessage } from '@/lib/error';

const emptyCredential = () => ({ source: 'database', value: '', configured: false });
const fieldsFor = (method) => ({
  token: ['token'],
  approle: ['role_id', 'secret_id'],
  kubernetes: ['jwt'],
  jwt: ['jwt'],
  cert: ['client_cert', 'client_key'],
}[method]);

export default {
  components: { VaultCredentialField },
  props: { itemType: String },
  mixins: [ItemFormBase],
  data() {
    return {
      defaultMount: 'secret',
      customCA: false,
      testing: false,
      testResult: null,
      savedMethod: 'token',
      savedCredentials: {},
      savedAuthParams: {},
    };
  },
  computed: {
    busy() { return this.formSaving || this.testing; },
    authMethod() { return this.item?.params.auth_method || 'token'; },
    authMethods() {
      return [
        { value: 'token', text: this.$t('uiToken'), icon: 'mdi-key-outline' },
        { value: 'approle', text: 'AppRole', icon: 'mdi-account-key-outline' },
        { value: 'kubernetes', text: 'Kubernetes', icon: 'mdi-kubernetes' },
        { value: 'jwt', text: 'JWT', icon: 'mdi-shield-key-outline' },
        { value: 'cert', text: this.$t('vaultCertificate'), icon: 'mdi-certificate-outline' },
      ];
    },
    methodHint() {
      return this.$t({
        token: 'vaultTokenAuthHint',
        approle: 'vaultAppRoleHint',
        kubernetes: 'vaultKubernetesHint',
        jwt: 'vaultJWTHint',
        cert: 'vaultCertHint',
      }[this.authMethod]);
    },
    credentialFields() {
      const labels = {
        token: this.$t('uiToken'),
        role_id: 'Role ID',
        secret_id: 'Secret ID',
        jwt: 'JWT',
        client_cert: this.$t('vaultClientCertificate'),
        client_key: this.$t('privateKey'),
      };
      return fieldsFor(this.authMethod)
        .filter((name) => name !== 'secret_id' || !this.item.params.without_secret_id)
        .map((name) => ({ name, label: labels[name], multiline: name.startsWith('client_') }));
    },
  },
  methods: {
    getNewItem() {
      return {
        type: 'vault',
        params: { kv_version: 2, auth_method: 'token' },
        readonly: true,
        credentials: {},
      };
    },
    afterLoadData() {
      this.item.type = 'vault';
      this.$set(this.item, 'params', { kv_version: 2, auth_method: 'token', ...this.item.params });
      const credentials = this.item.credentials || {
        token: {
          source: this.item.source_storage_type || 'database',
          value: this.item.secret || '',
          configured: !this.isNew,
        },
      };
      this.savedMethod = this.authMethod;
      this.savedAuthParams = { ...this.item.params };
      this.savedCredentials = JSON.parse(JSON.stringify(credentials));
      this.$set(this.item, 'credentials', { ...credentials });
      [...fieldsFor(this.authMethod), 'ca_cert'].forEach((field) => {
        if (!this.item.credentials[field]) {
          this.$set(this.item.credentials, field, emptyCredential());
        }
      });
      this.customCA = !!credentials.ca_cert?.configured;
      delete this.item.secret;
      delete this.item.source_storage_type;
    },
    selectMethod(method) {
      if (method === this.authMethod) return;
      this.item.params.auth_method = method;
      this.$set(this.item.params, 'auth_mount', method === 'token' ? '' : method);
      this.$set(this.item.params, 'auth_role', '');
      this.$delete(this.item.params, 'without_secret_id');
      if (method === this.savedMethod) {
        ['auth_mount', 'auth_role', 'without_secret_id'].forEach((key) => {
          if (this.savedAuthParams[key] !== undefined) {
            this.$set(this.item.params, key, this.savedAuthParams[key]);
          }
        });
      }
      const credentials = { ca_cert: this.item.credentials.ca_cert };
      fieldsFor(method).forEach((field) => {
        credentials[field] = method === this.savedMethod && this.savedCredentials[field]
          ? { ...this.savedCredentials[field] } : emptyCredential();
        if (method === 'kubernetes' && !credentials[field].configured) {
          credentials[field].source = 'file';
        }
      });
      this.$set(this.item, 'credentials', credentials);
      this.$refs.form.resetValidation();
    },
    navigateMethods(event, method) {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      event.preventDefault();
      const index = this.authMethods.findIndex((m) => m.value === method);
      const next = {
        Home: 0, End: 4, ArrowLeft: (index + 4) % 5, ArrowRight: (index + 1) % 5,
      }[event.key];
      this.selectMethod(this.authMethods[next].value);
      event.currentTarget.parentElement.children[next].focus();
    },
    beforeSave() {
      if (!this.customCA) this.item.credentials.ca_cert = { ...emptyCredential(), clear: true };
      if (this.item.params.without_secret_id) {
        this.item.credentials.secret_id = { ...emptyCredential(), clear: true };
      }
    },
    async testAuthentication() {
      if (!this.$refs.form.validate()) return;
      this.beforeSave();
      this.testing = true;
      this.testResult = null;
      try {
        await axios.post(`${this.getItemsUrl()}/test`, { ...this.item, project_id: this.projectId });
        this.testResult = { type: 'success', message: this.$t('vaultAuthSucceeded') };
      } catch (error) {
        this.testResult = { type: 'error', message: getErrorMessage(error) };
      } finally { this.testing = false; }
    },
    getItemsUrl() { return `/api/project/${this.projectId}/secret_storages`; },
    getSingleItemUrl() { return `${this.getItemsUrl()}/${this.itemId}`; },
  },
  watch: {
    item: { deep: true, handler() { this.testResult = null; } },
  },
};
</script>

<style scoped lang="scss">
.VaultConnectionFields {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 16px;
  margin-bottom: 8px;
}
.VaultAuthMethods {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
  button {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px; padding: 14px 6px; min-height: 86px; border-radius: 14px;
    border: 1px solid var(--taskexec-border); color: var(--taskexec-muted);
    font-size: 13px; font-weight: 600; overflow-wrap: anywhere;
    .v-icon { color: inherit; }
    &:focus-visible { outline: 2px solid var(--taskexec-accent); outline-offset: 3px; }
    &:disabled { opacity: .55; cursor: default; }
  }
  .VaultAuthMethods__selected {
    background: var(--taskexec-accent-soft); border-color: var(--taskexec-accent);
    color: var(--taskexec-accent);
  }
}
@media (max-width: 599px) {
  .VaultAuthMethods { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .VaultConnectionFields { grid-template-columns: 1fr; gap: 0; }
}
</style>
