<template>
  <v-menu :top="top" offset-y min-width="240" max-width="300" max-height="420">
    <template v-slot:activator="{ on, attrs }">
      <v-btn
        v-bind="attrs"
        v-on="on"
        icon
        width="36"
        height="36"
        :title="`${$t('interfaceLanguage')}: ${currentLanguage}`"
        :aria-label="`${$t('interfaceLanguage')}: ${currentLanguage}`"
        :data-testid="testId"
        class="language-menu__button"
      >
        <img :src="`flags/${locale}.svg`" alt="" />
      </v-btn>
    </template>

    <v-list dense :aria-label="$t('interfaceLanguage')">
      <v-list-item
        v-for="language in languages"
        :key="language.id"
        @click="$emit('select', language.id)"
        :data-locale="language.id || 'system'"
      >
        <v-list-item-icon>
          <img :src="`flags/${language.flag}.svg`" width="24" height="24" alt="" />
        </v-list-item-icon>
        <v-list-item-title>{{ language.title }}</v-list-item-title>
      </v-list-item>
    </v-list>
  </v-menu>
</template>

<script>
import { LANGUAGE_NAMES, normalizeLocale } from '@/lib/locale';

export default {
  props: {
    top: Boolean,
    testId: String,
  },
  computed: {
    locale() {
      return normalizeLocale(this.$i18n.locale);
    },
    currentLanguage() {
      return LANGUAGE_NAMES[this.locale];
    },
    languages() {
      return [
        { id: '', flag: normalizeLocale(navigator.language), title: this.$t('system') },
        ...Object.entries(LANGUAGE_NAMES).map(([id, title]) => ({ id, flag: id, title })),
      ];
    },
  },
};
</script>

<style scoped>
.language-menu__button img {
  width: 25px; height: 25px; border-radius: 50%; object-fit: cover;
}
</style>
