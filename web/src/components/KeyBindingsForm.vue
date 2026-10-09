<template>
  <div class="key-bindings">
    <p class="text-body-2 text--secondary">{{ $t('keyBindingsHelp') }}</p>
    <v-alert v-if="error" text type="error">{{ error }}</v-alert>
    <v-card v-for="(binding, index) in value" :key="index" outlined class="pa-4 mb-4">
      <div class="d-flex align-start">
        <v-autocomplete :value="binding.key_id" :items="keys" item-value="id" item-text="name"
          :label="$t('accessKey')" :rules="[(v) => !!v || $t('required')]"
          :disabled="disabled" outlined dense @change="selectKey(index, $event)" />
        <v-btn icon :disabled="disabled" :aria-label="$t('deleteKey')" @click="remove(index)">
          <v-icon>mdi-delete-outline</v-icon>
        </v-btn>
      </div>
      <v-text-field :value="binding.name" @input="update(index, {name: $event})"
        :label="$t('keyVariableName')" outlined dense :disabled="disabled"
        :rules="[(v) => /^[A-Za-z_][A-Za-z0-9_]*$/.test(v || '') || $t('keyBindingInvalid'),
          () => environmentError(binding) || true]" />
      <v-select :value="binding.type" :items="targets" :label="$t('keyBindingTarget')"
        @change="update(index, {type: $event})" :disabled="disabled" outlined dense />
      <v-select v-if="isObject(binding)" :value="binding.field == null ? '' : binding.field"
        :items="fields(binding)" :label="$t('vaultValueField')" outlined dense :disabled="disabled"
        @change="update(index, {field: $event === '' ? null : $event})">
        <template v-slot:item="{ item }">
          <v-icon small class="mr-2">
            {{ item.value === '' ? 'mdi-code-json' : 'mdi-key-outline' }}
          </v-icon>
          {{ item.text }}
        </template>
      </v-select>
      <div class="d-flex align-center flex-wrap mb-3">
        <v-btn small text color="primary" :disabled="disabled || !binding.key_id"
          :loading="state(binding).loading" @click="refresh(binding.key_id)">
          <v-icon left small>mdi-refresh</v-icon>{{ $t('keyCurrentValue') }}
        </v-btn>
        <span v-if="state(binding).read_at" class="caption text--secondary ml-2">
          {{ $t('keyReadAt', {time: readTime(binding)}) }}
        </span>
      </div>
      <v-alert v-if="state(binding).error" type="error" dense text>
        {{ state(binding).error }}
      </v-alert>
      <v-alert v-else-if="missingField(binding)" type="error" dense text>
        {{ $t('vaultFieldMissing', {field: binding.field}) }}
      </v-alert>
      <v-alert v-else-if="environmentError(binding)" type="error" dense text>
        {{ environmentError(binding) }}
      </v-alert>
      <pre v-else-if="state(binding).read_at"
        class="key-bindings__value">{{ preview(binding) }}</pre>
      <div class="mt-3 text-body-2" v-if="binding.name">
        <span class="text--secondary">{{ $t('keyUsage') }}:</span>
        <code class="ml-2">{{ expression(binding) }}</code>
      </div>
    </v-card>
    <v-btn outlined color="primary" :disabled="disabled" @click="add">
      <v-icon left>mdi-plus</v-icon>{{ $t('keyAddBinding') }}
    </v-btn>
  </div>
</template>

<script>
import axios from 'axios';
import { getErrorMessage } from '@/lib/error';

export default {
  props: {
    value: { type: Array, default: () => [] },
    projectId: [Number, String],
    disabled: Boolean,
  },
  data: () => ({
    keys: [], live: {}, error: null, active: true,
  }),
  computed: {
    targets() {
      return [
        { value: 'var', text: this.$t('extraVariables') },
        { value: 'env', text: this.$t('environmentVariables') },
      ];
    },
  },
  async created() {
    try {
      const { data } = await axios.get(`/api/project/${this.projectId}/keys`);
      if (!this.active) return;
      this.keys = data.filter((key) => ['string', 'object'].includes(key.type));
      await Promise.all([...new Set(this.value.map((b) => b.key_id).filter(Boolean))]
        .map((id) => this.refresh(id)));
    } catch (err) { if (this.active) this.error = getErrorMessage(err); }
  },
  beforeDestroy() { this.active = false; this.live = {}; },
  methods: {
    update(index, changes) {
      this.$emit('input', this.value.map((b, i) => (i === index ? { ...b, ...changes } : b)));
    },
    add() {
      this.$emit('input', [...this.value, {
        key_id: null, name: '', type: 'var', field: null,
      }]);
    },
    remove(index) { this.$emit('input', this.value.filter((b, i) => i !== index)); },
    async selectKey(index, id) {
      const key = this.keys.find((k) => k.id === id);
      this.update(index, { key_id: id, name: key?.name || '', field: null });
      if (id) await this.refresh(id);
    },
    isObject(binding) { return this.keys.some((k) => k.id === binding.key_id && k.type === 'object'); },
    state(binding) { return this.live[binding.key_id] || {}; },
    fields(binding) {
      const value = this.state(binding).value;
      const items = [{ text: this.$t('keyWholeValue'), value: '' }];
      if (value && typeof value === 'object') {
        Object.keys(value).sort().forEach((name) => items.push({
          text: binding.type === 'env' ? name : `${name}: ${JSON.stringify(value[name]).slice(0, 70)}`,
          value: name,
        }));
      }
      if (binding.field != null && !items.some((item) => item.value === binding.field)) {
        items.push({ text: binding.field, value: binding.field });
      }
      return items;
    },
    missingField(binding) {
      const state = this.state(binding);
      return state.read_at && binding.field != null
        && !Object.prototype.hasOwnProperty.call(state.value || {}, binding.field);
    },
    preview(binding) {
      if (binding.type === 'env') return this.environmentNames(binding).map((name) => `$${name}`).join('\n');
      const value = this.state(binding).value;
      const selected = binding.field == null ? value : value?.[binding.field];
      return typeof selected === 'string' ? selected : JSON.stringify(selected, null, 2);
    },
    environmentNames(binding) {
      const value = this.state(binding).value;
      const fields = binding.field == null && value && typeof value === 'object' && !Array.isArray(value)
        ? Object.keys(value).sort().map((field) => `${binding.name}_${field}`) : [];
      return [binding.name, ...fields];
    },
    environmentError(binding) {
      if (binding.type !== 'env') return null;
      const invalid = this.environmentNames(binding).slice(1)
        .find((name) => !/^[A-Za-z_][A-Za-z0-9_]*$/.test(name) || name.length > 255
          || ['TASKEXEC_JWT', 'taskexec_vars'].includes(name));
      return invalid ? this.$t('keyBindingEnvFieldInvalid', { field: invalid.slice(binding.name.length + 1) }) : null;
    },
    expression(binding) {
      if (binding.type === 'env') return `$${binding.name}`;
      const value = this.state(binding).value;
      const first = binding.field == null && value && typeof value === 'object'
        ? Object.keys(value)[0] : null;
      if (first && /^[A-Za-z_][A-Za-z0-9_]*$/.test(first)
        && !['items', 'keys', 'values', 'get', 'update', 'copy'].includes(first)) {
        return `{{ ${binding.name}.${first} }}`;
      }
      if (first) return `{{ ${binding.name}[${JSON.stringify(first)}] }}`;
      return `{{ ${binding.name} }}`;
    },
    readTime(binding) { return new Date(this.state(binding).read_at).toLocaleTimeString(); },
    async refresh(id) {
      if (this.live[id]?.loading) return;
      this.$set(this.live, id, { loading: true });
      try {
        const { data } = await axios.post(`/api/project/${this.projectId}/keys/${id}/preview`);
        if (this.active) this.$set(this.live, id, { ...data, loading: false });
      } catch (err) {
        if (this.active) this.$set(this.live, id, { error: getErrorMessage(err), loading: false });
      }
    },
  },
};
</script>

<style lang="scss">
.key-bindings__value {
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  background: var(--taskexec-surface);
  border: 1px solid var(--taskexec-border);
  border-radius: 12px;
  padding: 12px;
  scrollbar-width: thin;
  scrollbar-color: var(--taskexec-border) transparent;
}
</style>
