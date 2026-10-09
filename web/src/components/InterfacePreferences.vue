<template>
  <div class="interface-preferences" :class="{ 'interface-preferences--vertical': vertical }">
    <v-btn
      icon
      width="36"
      height="36"
      class="interface-preferences__theme"
      :title="$t('toggleColorTheme')"
      :aria-label="$t('toggleColorTheme')"
      :aria-pressed="String(darkMode)"
      :data-testid="`${testId}-theme`"
      @click="$emit('toggle-theme')"
    >
      <v-icon>{{ darkMode ? 'mdi-weather-night' : 'mdi-white-balance-sunny' }}</v-icon>
    </v-btn>
    <LanguageMenu
      :top="menuTop"
      :test-id="`${testId}-language`"
      @select="$emit('select-language', $event)"
    />
  </div>
</template>

<script>
import LanguageMenu from '@/components/LanguageMenu.vue';

export default {
  components: { LanguageMenu },
  props: {
    darkMode: Boolean,
    menuTop: Boolean,
    vertical: Boolean,
    testId: { type: String, required: true },
  },
};
</script>

<style scoped>
.interface-preferences { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.interface-preferences--vertical { flex-direction: column; }
.interface-preferences__theme.v-btn { color: var(--taskexec-text); }
.interface-preferences__theme .v-icon { color: inherit; }
</style>
