<template>
  <div class="SecretSourceToggle mb-2">
    <b class="d-block text-caption mb-1">{{ label }}</b>
    <v-btn-toggle
        class="SecretSourceToggle__buttons"
        :value="value"
        @change="$emit('input', $event)"
        tile
        group
        mandatory
    >
      <v-btn
        v-for="source in sources"
        :key="source.value"
        :value="source.value"
        :disabled="disabled"
        small
        class="ma-0"
      >
        {{ source.text }}
      </v-btn>
    </v-btn-toggle>
  </div>
</template>
<script>
export default {
  props: {
    /** Where the secret is taken from: 'database', 'env' or 'file'. */
    value: String,
    /** Name of the secret, shown on the left of the toggle. */
    label: String,
    disabled: Boolean,
  },

  data() {
    return {
      sources: [
        { value: 'database', text: this.$t('uiStoreInDB') },
        { value: 'env', text: this.$t('uiFromENV') },
        { value: 'file', text: this.$t('uiFromFile') },
      ],
    };
  },
};
</script>

<style scoped lang="scss">
.SecretSourceToggle__buttons {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 4px;
  width: 100%;

  .v-btn {
    min-width: 0;
    height: auto;
    min-height: 44px;
    padding: 6px;
    border-radius: 8px !important;
  }

  ::v-deep .v-btn__content {
    display: block;
    flex: 0 1 auto;
    min-width: 0;
    width: 100%;
    white-space: normal;
    overflow-wrap: anywhere;
    line-height: 1.3;
  }
}
</style>
