<template>
  <v-btn v-if="contextHelp" icon class="context-help-toggle"
    :class="{ 'context-help-toggle--active': contextHelp.enabled }"
    :color="contextHelp.enabled ? 'primary' : undefined"
    :aria-label="$t('contextHelpToggle')" :aria-pressed="String(contextHelp.enabled)"
    @click="contextHelp.enabled = !contextHelp.enabled">
    <v-icon>mdi-help-box</v-icon>
  </v-btn>
</template>
<script>
export default { inject: { contextHelp: { default: null } } };
</script>
<style lang="scss">
.context-help-toggle:not(.context-help-toggle--active) .v-icon { opacity: .5; }
</style>
