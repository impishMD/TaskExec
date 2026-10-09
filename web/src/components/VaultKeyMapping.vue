<template>
  <div class="vault-key-mapping mb-5">
    <v-alert v-if="error" type="error" text dense>{{ error }}</v-alert>
    <p v-if="type === 'object'" class="text-body-2 text--secondary">
      {{ $t('vaultObjectHelp') }}
    </p>
    <v-autocomplete v-for="target in targets" :key="target.id"
      :value="value[target.id] || null" @input="setField(target.id, $event)"
      :items="options(target.id)" :label="target.label"
      :rules="[(v) => validateField(target, v)]"
      :disabled="disabled || !canLoadFields" :clearable="!target.required" :loading="loading"
      class="mb-3" outlined dense @focus="ensureFields"
    >
      <template v-slot:item="{ item }">
        <v-icon small class="mr-2">mdi-key-outline</v-icon>
        {{ item.text }}
      </template>
      <template v-slot:no-data>
        <v-list-item><v-list-item-content class="text-body-2">
          {{ $t('noValues') }}
        </v-list-item-content></v-list-item>
      </template>
    </v-autocomplete>
  </div>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';

export default {
  props: {
    value: { type: Object, default: () => ({}) },
    projectId: [Number, String],
    storageId: [Number, String],
    path: String,
    type: String,
    disabled: Boolean,
  },
  data: () => ({
    fields: [], loaded: false, loading: false, error: null, request: 0, timer: null,
  }),
  computed: {
    source() { return `${this.projectId}:${this.storageId}:${this.path}`; },
    canLoadFields() {
      return !!(this.projectId && this.storageId && this.path?.trim()
        && !this.path.endsWith('/') && !this.path.includes('#'));
    },
    targets() {
      switch (this.type) {
        case 'string': return [{ id: 'value', label: this.$t('vaultValueField'), required: true }];
        case 'login_password': return [
          { id: 'login', label: this.$t('usernameOptional') },
          { id: 'password', label: this.$t('password'), required: true },
        ];
        case 'ssh': return [
          { id: 'login', label: this.$t('usernameOptional') },
          { id: 'passphrase', label: this.$t('uiPassphraseOptional') },
          { id: 'private_key', label: this.$t('privateKey'), required: true },
        ];
        default: return [];
      }
    },
  },
  watch: {
    source() {
      clearTimeout(this.timer);
      this.request += 1;
      this.fields = [];
      this.loaded = false;
      this.error = null;
      this.loading = this.canLoadFields;
      if (this.canLoadFields) this.timer = setTimeout(() => this.loadFields(), 250);
    },
    type() { this.$emit('input', {}); },
  },
  mounted() {
    this.ensureFields();
  },
  beforeDestroy() { clearTimeout(this.timer); this.request += 1; },
  methods: {
    ensureFields() {
      if (this.canLoadFields && !this.loaded && !this.loading) this.loadFields();
    },
    validateField(target, value) {
      if (!value) return !target.required || this.$t('required');
      if (!this.loaded) return true;
      const field = this.fields.find((item) => item.name === value);
      if (!field) return this.$t('vaultFieldMissing', { field: value });
      if (this.type === 'string') {
        return ['string', 'number', 'boolean'].includes(field.type)
          || this.$t('vaultFieldScalar', { field: value });
      }
      return field.type === 'string' || this.$t('vaultFieldString', { field: value });
    },
    options(target) {
      const items = this.fields.filter((field) => (this.type === 'string'
        ? ['string', 'number', 'boolean'].includes(field.type) : field.type === 'string'))
        .map((field) => ({ text: field.name, value: field.name }));
      const selected = this.value[target];
      if (selected && !items.some((item) => item.value === selected)) {
        items.push({ text: selected, value: selected });
      }
      return items;
    },
    setField(target, field) {
      const value = { ...this.value };
      if (field) value[target] = field;
      else delete value[target];
      this.$emit('input', value);
    },
    async loadFields() {
      clearTimeout(this.timer);
      if (!this.canLoadFields) return;
      this.request += 1;
      const request = this.request;
      this.loading = true;
      this.loaded = false;
      this.error = null;
      try {
        const { data } = await axios.post(
          `/api/project/${this.projectId}/secret_storages/${this.storageId}/fields`,
          { path: this.path },
        );
        if (request === this.request) {
          this.fields = data.fields;
          this.loaded = true;
        }
      } catch (err) {
        if (request === this.request) this.error = getErrorMessage(err);
      } finally {
        if (request === this.request) this.loading = false;
      }
    },
  },
};
</script>
