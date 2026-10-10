<template>
  <div class="project-tokens" data-testid="project-tokens">
    <p class="text-body-2 text--secondary mb-4">{{ $t('projectTokensHint') }}</p>
    <div class="project-tokens__toolbar mb-4">
      <v-tabs v-model="tokenFilter" class="project-tokens__filters" show-arrows>
        <v-tab href="#all" data-testid="project-token-filter-all">{{ $t('all') }}</v-tab>
        <v-tab href="#active" data-testid="project-token-filter-active">
          {{ $t('projectTokensActive') }}
        </v-tab>
        <v-tab href="#revoked" data-testid="project-token-filter-revoked">
          {{ $t('projectTokensRevoked') }}
        </v-tab>
      </v-tabs>
      <v-btn color="primary" :disabled="loading || busy || !!error" @click="openCreate()"
        data-testid="project-token-new">{{ $t('uiNewToken') }}</v-btn>
    </div>
    <v-alert v-if="error" type="error" text>{{ error }}</v-alert>
    <v-btn v-if="error" text @click="load">{{ $t('alertsRetry') }}</v-btn>
    <v-data-table :headers="headers" :items="filteredItems" :loading="loading"
      :items-per-page="10" :page.sync="page"
      class="project-tokens__table">
      <template v-slot:item.name="{ item }">
        <div class="py-3">{{ item.name }}<br><code>{{ item.id.slice(0, 8) }}…</code></div>
      </template>
      <template v-slot:item.scopes="{ item }">
        <div class="py-2">
          <div v-for="scope in item.scopes" :key="scope">{{ scopeLabel(scope) }}</div>
          <span class="text--secondary">{{ item.all_templates ? $t('projectTokenAllTemplates')
            : templateNames(item.template_ids) }}</span>
          <div v-if="(item.overrides || []).length" class="text--secondary">
            {{ $t('projectTokenOverrides') }}: {{ overrideNames(item.overrides) }}
          </div>
        </div>
      </template>
      <template v-slot:item.creator_name="{ item }">
        <div>{{ item.creator_name }}</div>
        <small class="text--secondary">
          {{ $t('uiCreated') }}: {{ item.created | formatDate }}
        </small>
      </template>
      <template v-slot:item.expires_at="{ item }">
        {{ item.expires_at ? $options.filters.formatDate(item.expires_at)
          : $t('projectTokenNoExpiry') }}
      </template>
      <template v-slot:item.last_used_at="{ item }">
        {{ item.last_used_at ? $options.filters.formatDate(item.last_used_at)
          : $t('projectTokenNeverUsed') }}
      </template>
      <template v-slot:item.status="{ item }">
        <v-chip small :color="item.revoked_at ? 'error' : isExpired(item) ? 'warning' : 'success'">
          {{ item.revoked_at ? $t('uiRevoked')
            : isExpired(item) ? $t('projectTokenExpired') : $t('projectTokenActive') }}
        </v-chip>
      </template>
      <template v-slot:item.actions="{ item }">
        <div class="d-flex" v-if="!item.revoked_at">
          <v-btn icon :disabled="busy" :title="$t('projectTokenRotate')"
            :aria-label="$t('projectTokenRotate')" @click="openCreate(item)">
            <v-icon>mdi-key-change</v-icon>
          </v-btn>
          <v-btn icon color="error" :disabled="busy" :title="$t('projectTokenRevoke')"
            :aria-label="$t('projectTokenRevoke')"
            @click="revokeTarget = item; revokeDialog = true">
            <v-icon>mdi-key-remove</v-icon>
          </v-btn>
        </div>
      </template>
    </v-data-table>

    <YesNoDialog v-model="revokeDialog" :title="$t('projectTokenRevoke')"
      :text="$t('projectTokenConfirmRevoke', { name: (revokeTarget || {}).name })"
      :yes-button-title="$t('projectTokenRevoke')" @yes="revoke" :max-width="480" />

    <v-dialog v-model="dialog" max-width="800" scrollable :persistent="busy">
      <v-card>
        <v-card-title>
          {{ replacing ? $t('projectTokenRotate') : $t('uiNewToken') }}
          <v-spacer />
          <v-btn icon :disabled="busy" @click="dialog = false" :aria-label="$t('close')">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </v-card-title>
        <v-card-text v-if="issuedToken" class="pt-4" data-testid="project-token-issued">
          <v-alert type="success" text>{{ $t('projectTokenCopyHint') }}</v-alert>
          <div class="d-flex align-center" style="gap: 8px">
            <code class="project-token-secret">{{ issuedToken }}</code>
            <CopyClipboardButton :text="issuedToken" />
          </div>
        </v-card-text>
        <v-card-text v-else class="pt-4">
          <v-alert v-if="formError" type="error" text>{{ formError }}</v-alert>
          <v-alert v-if="replacing" type="info" text>{{ $t('projectTokenRotateHint') }}</v-alert>
          <v-form ref="form" :disabled="busy" @submit.prevent="create">
            <v-text-field v-model="form.name" :label="$t('name')" outlined dense maxlength="100"
              :rules="[required]" data-testid="project-token-name" />
            <v-select v-model="preset" :items="presets" :label="$t('projectTokenPreset')"
              outlined dense @change="applyPreset" data-testid="project-token-preset" />
            <v-select v-model="form.scopes" :items="scopes" :label="$t('permissions')"
              multiple small-chips outlined dense :rules="[required]"
              class="project-token-multiselect"
              data-testid="project-token-scopes" />
            <v-checkbox v-model="form.all_templates" :label="$t('projectTokenAllTemplates')"
              class="mt-0" data-testid="project-token-all" />
            <v-autocomplete v-if="!form.all_templates" v-model="form.template_ids"
              :items="templates" item-text="name" item-value="id"
              multiple small-chips deletable-chips class="project-token-multiselect"
              :label="$t('projectTokenTemplates')" outlined dense :rules="[required]"
              data-testid="project-token-templates" />
            <v-select v-model="form.overrides" :items="overrides"
              multiple small-chips outlined dense data-testid="project-token-overrides"
              :disabled="!form.scopes.includes('tasks:run')" :label="$t('projectTokenOverrides')"
              :hint="$t('projectTokenOverridesHint')" persistent-hint
              class="project-token-multiselect mb-4" />
            <v-select v-model="expiresDays" :items="expiryOptions" outlined dense
              :label="$t('projectTokenLifetime')" data-testid="project-token-expiry" />
          </v-form>
        </v-card-text>
        <v-divider />
        <v-card-actions class="pa-4 justify-end">
          <v-btn v-if="issuedToken" color="primary" @click="dialog = false">
            {{ $t('close') }}
          </v-btn>
          <template v-else>
            <v-btn text color="primary" :disabled="busy" @click="dialog = false">
              {{ $t('cancel') }}
            </v-btn>
            <v-btn color="primary" :loading="busy" @click="create"
              data-testid="project-token-create">
              {{ replacing ? $t('projectTokenRotate') : $t('create') }}
            </v-btn>
          </template>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';
import CopyClipboardButton from '@/components/CopyClipboardButton.vue';
import YesNoDialog from '@/components/YesNoDialog.vue';

const emptyForm = () => ({
  name: '',
  scopes: ['templates:read', 'tasks:run', 'tasks:read'],
  all_templates: false,
  template_ids: [],
  overrides: [],
});
const scopeKeys = {
  'templates:read': 'projectTokenScopeTemplates',
  'tasks:run': 'projectTokenScopeRun',
  'tasks:read': 'projectTokenScopeTasks',
  'tasks:logs': 'projectTokenScopeLogs',
  'tasks:stop': 'projectTokenScopeStop',
};

export default {
  components: { CopyClipboardButton, YesNoDialog },
  props: { projectId: { type: Number, required: true } },
  data: () => ({
    items: [],
    tokenFilter: 'all',
    page: 1,
    templates: [],
    loading: false,
    busy: false,
    error: '',
    formError: '',
    dialog: false,
    replacing: null,
    issuedToken: '',
    form: emptyForm(),
    preset: 'ci',
    expiresDays: 90,
    revokeDialog: false,
    revokeTarget: null,
  }),
  computed: {
    base() { return `/api/project/${this.projectId}/tokens`; },
    filteredItems() {
      if (this.tokenFilter === 'active') {
        return this.items.filter((item) => !item.revoked_at && !this.isExpired(item));
      }
      if (this.tokenFilter === 'revoked') return this.items.filter((item) => !!item.revoked_at);
      return this.items;
    },
    scopes() {
      return Object.entries(scopeKeys).map(([value, key]) => ({ value, text: this.$t(key) }));
    },
    overrides() {
      return [
        ['git_branch', 'branch'], ['playbook', 'playbook'], ['inventory_id', 'inventory'],
        ['arguments', 'uiArgs'], ['environment', 'extraVariables'], ['params', 'projectTokenAppParams'],
      ].map(([value, key]) => ({ value, text: this.$t(key) }));
    },
    presets() {
      return [
        { value: 'read', text: this.$t('uiReadOnly') },
        { value: 'ci', text: this.$t('projectTokenPresetCI') },
        { value: 'operate', text: this.$t('projectTokenPresetOperate') },
        { value: 'custom', text: this.$t('uiCustom') },
      ];
    },
    expiryOptions() {
      return [
        { value: 30, text: this.$t('ui30Days') }, { value: 90, text: this.$t('ui90Days') },
        { value: 365, text: this.$t('ui1Year') }, { value: 0, text: this.$t('projectTokenNoExpiry') },
      ];
    },
    headers() {
      return [
        ['name', 'name', 140], ['scopes', 'permissions', 240], ['creator_name', 'projectTokenCreatedBy', 140],
        ['expires_at', 'projectTokenExpiresAt', 140], ['last_used_at', 'projectTokenLastUsed', 160], ['status', 'status', 100],
      ].map(([value, label, width]) => ({
        value, text: this.$t(label), width, class: 'project-tokens__heading',
      }))
        .concat([{
          value: 'actions', text: '', sortable: false, width: 96,
        }]);
    },
  },
  watch: {
    dialog(open) { if (!open) { this.issuedToken = ''; this.formError = ''; } },
    tokenFilter() { this.page = 1; },
    projectId() {
      this.dialog = false; this.items = []; this.tokenFilter = 'all'; this.page = 1; this.load();
    },
  },
  created() { this.load(); },
  methods: {
    required(value) { return (Array.isArray(value) ? value.length > 0 : !!value?.trim()) || this.$t('required'); },
    scopeLabel(value) { return this.$t(scopeKeys[value] || value); },
    isExpired(item) { return item.expires_at && Date.parse(item.expires_at) <= Date.now(); },
    templateNames(ids) { return ids.map((id) => this.templates.find((t) => t.id === id)?.name || `#${id}`).join(', '); },
    overrideNames(names) { return names.map((value) => this.overrides.find((o) => o.value === value)?.text || value).join(', '); },
    async load() {
      const project = this.projectId;
      this.loading = true; this.error = '';
      try {
        const [tokens, templates] = await Promise.all([
          axios.get(this.base), axios.get(`/api/project/${project}/templates`),
        ]);
        if (this.projectId === project) {
          this.items = tokens.data; this.templates = templates.data;
        }
      } catch (e) { if (this.projectId === project) this.error = getErrorMessage(e); } finally {
        if (this.projectId === project) this.loading = false;
      }
    },
    openCreate(item = null) {
      this.replacing = item;
      this.form = item ? {
        name: item.name,
        scopes: [...item.scopes],
        all_templates: item.all_templates,
        template_ids: [...(item.template_ids || [])],
        overrides: [...(item.overrides || [])],
      } : emptyForm();
      this.preset = item ? 'custom' : 'ci'; this.expiresDays = 90;
      this.issuedToken = ''; this.formError = ''; this.dialog = true;
      this.$nextTick(() => this.$refs.form?.resetValidation());
    },
    applyPreset(preset) {
      if (preset === 'custom') return;
      this.form.scopes = ['templates:read', 'tasks:read'];
      if (preset !== 'read') this.form.scopes.push('tasks:run');
      if (preset === 'operate') this.form.scopes.push('tasks:stop');
    },
    async create() {
      if (this.busy || !this.$refs.form.validate()) return;
      this.busy = true; this.formError = '';
      const project = this.projectId;
      try {
        const url = this.replacing ? `${this.base}/${this.replacing.id}/rotate` : this.base;
        const response = await axios.post(url, {
          ...this.form,
          template_ids: this.form.all_templates ? [] : this.form.template_ids,
          overrides: this.form.scopes.includes('tasks:run') ? this.form.overrides : [],
          expires_at: this.expiresDays
            ? new Date(Date.now() + this.expiresDays * 86400000).toISOString() : null,
        });
        if (this.projectId !== project) return;
        this.issuedToken = response.data.token;
        await this.load();
      } catch (e) {
        if (this.projectId === project) this.formError = getErrorMessage(e);
      } finally { this.busy = false; }
    },
    async revoke() {
      if (this.busy || !this.revokeTarget) return;
      this.busy = true;
      try { await axios.delete(`${this.base}/${this.revokeTarget.id}`); await this.load(); } catch (e) {
        this.error = getErrorMessage(e);
      } finally { this.busy = false; this.revokeTarget = null; }
    },
  },
};
</script>

<style scoped>
.project-tokens__toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}
.project-tokens__filters {
  flex: 1 1 340px;
  min-width: 0;
  width: auto;
}
.project-token-multiselect.v-select.v-text-field--outlined.v-input--dense
  ::v-deep .v-select__selections {
  min-width: 0;
  padding: 12px 0;
  gap: 6px;
}
.project-token-multiselect.v-select ::v-deep .v-chip {
  max-width: 100%;
  height: auto;
  min-height: 24px;
  margin: 0;
  padding: 3px 10px;
}
.project-token-multiselect.v-select ::v-deep .v-chip__content {
  min-width: 0;
  height: auto;
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 18px;
}
.project-token-multiselect.v-select ::v-deep .v-chip__close {
  flex-shrink: 0;
}
.project-token-multiselect.v-select.v-input--is-dirty:not(.v-autocomplete)
  ::v-deep .v-select__selections input {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  opacity: 0;
}
.project-tokens__table.v-data-table ::v-deep .v-data-table__wrapper th,
.project-tokens__table.v-data-table ::v-deep .v-data-table__wrapper td {
  white-space: normal;
  overflow-wrap: anywhere;
}
.project-tokens .project-tokens__table.v-data-table
  ::v-deep .v-data-table__wrapper > table > .v-data-table-header > tr > th.project-tokens__heading {
  text-transform: none;
  letter-spacing: normal;
  font-size: 13px !important;
  overflow-wrap: normal;
  line-height: 1.4;
  padding-top: 12px;
  padding-bottom: 12px;
}
.project-tokens__table.v-data-table ::v-deep .v-data-table__mobile-row {
  height: auto !important;
  min-height: 0;
  padding: 12px 22px !important;
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
}
.project-tokens__table.v-data-table ::v-deep .v-data-table__mobile-row__header,
.project-tokens__table.v-data-table ::v-deep .v-data-table__mobile-row__cell {
  min-width: 0;
  max-width: 100%;
  text-align: left;
}
.project-token-secret {
  overflow-wrap: anywhere;
  white-space: normal;
  min-width: 0;
  flex: 1;
  padding: 12px;
}
</style>
