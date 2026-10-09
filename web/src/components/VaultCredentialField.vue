<template>
  <div class="VaultCredentialField">
    <div class="VaultCredentialField__heading">
      <span class="text-subtitle-2">{{ label }}</span>
      <v-select :value="value.source" :items="sources" :disabled="disabled"
        :aria-label="`${label}: ${$t('vaultCredentialSource')}`"
        dense outlined hide-details @change="changeSource" />
    </div>
    <component :is="multiline && value.source === 'database' ? 'v-textarea' : 'v-text-field'"
      :value="value.value" @input="changeValue"
      :label="inputLabel" :disabled="disabled" :rules="rules"
      :type="value.source === 'database' && !multiline ? 'password' : 'text'"
      :class="{ 'masked-secret-input': value.source === 'database' && !multiline }"
      :hint="hint" persistent-hint outlined dense rows="3" auto-grow
      autocomplete="new-password" :data-testid="`vault-credential-${field}`" />
  </div>
</template>

<script>
import { VTextField, VTextarea } from 'vuetify/lib';

export default {
  components: { VTextField, VTextarea },
  props: {
    value: { type: Object, required: true },
    field: String,
    label: String,
    disabled: Boolean,
    multiline: Boolean,
    optional: Boolean,
  },
  computed: {
    sources() {
      return [
        { value: 'database', text: this.$t('uiStoreInDB') },
        { value: 'env', text: this.$t('uiFromENV') },
        { value: 'file', text: this.$t('uiFromFile') },
      ];
    },
    inputLabel() {
      if (this.value.source === 'env') return this.$t('uiEnvVarName');
      if (this.value.source === 'file') return this.$t('uiPathToTheFile');
      return this.label;
    },
    hint() {
      if (this.value.source === 'file') return this.$t('vaultFileHint');
      if (this.value.source === 'env') return this.$t('vaultEnvHint');
      return this.$t(this.value.configured ? 'vaultCredentialSaved' : 'vaultCredentialEncrypted');
    },
    rules() {
      return [(v) => this.optional || !!v || this.value.configured || this.$t('valueRequired')];
    },
  },
  methods: {
    changeSource(source) {
      if (source === this.value.source) return;
      this.$emit('input', { source, value: '', configured: false });
    },
    changeValue(value) { this.$emit('input', { ...this.value, value, clear: false }); },
  },
};
</script>

<style scoped lang="scss">
.VaultCredentialField {
  margin-bottom: 20px;
  &__heading {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 10px;
    > span { flex: 1; }
    .v-input { flex: 0 1 220px; min-width: 140px; }
  }
}
</style>
