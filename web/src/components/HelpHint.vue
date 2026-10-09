<template>
  <v-menu v-if="shown" v-model="open" bottom offset-y :max-width="400"
    :close-on-content-click="false" content-class="context-help-popup">
    <template v-slot:activator="{ on, attrs }">
      <v-btn icon x-small v-bind="attrs" v-on="on" :aria-label="label || $t('contextHelpOpen')"
        @keydown.esc="closeOnEscape">
        <v-icon small>mdi-help-box</v-icon>
      </v-btn>
    </template>
    <v-card class="pa-4 text-body-2" role="note" @keydown.esc="closeOnEscape">
      <slot />
    </v-card>
  </v-menu>
</template>
<script>
export default {
  inject: { contextHelp: { default: null } },
  props: { visible: { type: Boolean, default: undefined }, label: String },
  data: () => ({ open: false }),
  computed: {
    shown() { return this.visible === undefined ? this.contextHelp?.enabled : this.visible; },
  },
  watch: { shown(value) { if (!value) this.open = false; } },
  methods: {
    closeOnEscape(event) {
      if (!this.open) return;
      event.stopPropagation();
      this.open = false;
    },
  },
};
</script>
<style lang="scss">
.context-help-popup {
  max-width: min(400px, calc(100vw - 32px)) !important;
  .v-card { color: var(--taskexec-text); line-height: 1.6; overflow-wrap: anywhere; }
}
</style>
