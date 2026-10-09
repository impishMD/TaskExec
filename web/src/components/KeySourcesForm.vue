<template>
  <div class="key-sources mb-5">
    <div class="d-flex align-center mb-2">
      <span class="text-body-2 text--secondary">{{ $t('keySources') }}</span>
      <v-spacer />
      <v-btn icon color="primary" :disabled="disabled" :aria-label="$t('keyAddBinding')"
        data-testid="key-source-add" @click="add"><v-icon>mdi-plus</v-icon></v-btn>
    </div>
    <v-alert v-if="error" type="error" text dense>{{ error }}</v-alert>
    <div v-for="(source, index) in value" :key="index" class="key-sources__row">
      <v-autocomplete :value="source.key_id" :items="keys" item-value="id" item-text="name"
        :label="$t('accessKey')" outlined dense :disabled="disabled"
        :rules="[(v) => !!v || $t('required')]" @input="selectKey(index, $event)">
        <template v-slot:item="{ item }">
          <v-icon small class="mr-2">mdi-key-outline</v-icon>{{ item.name }}
        </template>
      </v-autocomplete>
      <v-text-field :value="source.prefix" :label="$t('keySourcePrefix')" outlined dense
        :disabled="disabled" :rules="[(v) => prefixRule(v, index)]"
        @input="update(index, {prefix: $event})" />
      <div class="d-flex">
        <v-btn icon :disabled="disabled || !source.key_id" :loading="state(source).loading"
          :aria-label="$t('keySourceRefresh')" @click="loadFields(source.key_id, true)">
          <v-icon>mdi-refresh</v-icon>
        </v-btn>
        <v-btn icon :disabled="disabled" :aria-label="$t('deleteKey')" @click="remove(index)">
          <v-icon>mdi-delete-outline</v-icon>
        </v-btn>
      </div>
      <v-alert v-if="state(source).error" class="key-sources__error" type="error" text dense>
        {{ state(source).error }}
      </v-alert>
    </div>
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
    options() {
      return this.value.filter((source, index) => this.prefixRule(source.prefix, index) === true)
        .flatMap((source) => (this.state(source).paths || [''])
          .map((path) => `{{ ${source.prefix}${path} }}`));
    },
  },
  watch: {
    value: { deep: true, handler() { this.loadSources(); } },
    options: { immediate: true, handler(value) { this.$emit('options', value); } },
  },
  async created() {
    try {
      const { data } = await axios.get(`/api/project/${this.projectId}/keys`);
      if (!this.active) return;
      this.keys = data.filter((key) => ['string', 'object'].includes(key.type));
      this.loadSources();
    } catch (err) { if (this.active) this.error = getErrorMessage(err); }
  },
  beforeDestroy() { this.active = false; },
  methods: {
    state(source) { return this.live[source.key_id] || {}; },
    prefixRule(prefix, index) {
      if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(prefix || '') || prefix.length > 255
        || ['__proto__', 'prototype', 'constructor'].includes(prefix)) return this.$t('keySourceInvalid');
      return this.value.some((source, i) => i !== index && source.prefix === prefix)
        ? this.$t('keySourceDuplicate') : true;
    },
    update(index, changes) {
      this.$emit('input', this.value.map((source, i) => (i === index ? { ...source, ...changes } : source)));
    },
    add() { this.$emit('input', [...this.value, { key_id: null, prefix: '' }]); },
    remove(index) { this.$emit('input', this.value.filter((source, i) => i !== index)); },
    selectKey(index, id) {
      const key = this.keys.find((item) => item.id === id);
      const prefix = this.value[index].prefix || key?.name?.replace(/[^A-Za-z0-9_]/g, '_') || '';
      this.update(index, { key_id: id, prefix: /^\d/.test(prefix) ? `_${prefix}` : prefix });
    },
    loadSources() {
      this.value.forEach((source) => { if (source.key_id) this.loadFields(source.key_id); });
    },
    async loadFields(id, force = false) {
      if (this.live[id]?.loading || (!force && this.live[id]?.paths)) return;
      this.$set(this.live, id, { loading: true });
      try {
        const { data } = await axios.post(`/api/project/${this.projectId}/keys/${id}/fields`);
        if (this.active) this.$set(this.live, id, { paths: data.paths, loading: false });
      } catch (err) {
        if (this.active) this.$set(this.live, id, { error: getErrorMessage(err), loading: false });
      }
    },
  },
};
</script>

<style lang="scss">
.key-sources__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 0 12px;
  align-items: start;
  .key-sources__error { grid-column: 1 / -1; }
}
@media (max-width: 600px) {
  .key-sources__row {
    grid-template-columns: minmax(0, 1fr) auto;
    > .v-input:first-child { grid-column: 1 / -1; }
  }
}
</style>
