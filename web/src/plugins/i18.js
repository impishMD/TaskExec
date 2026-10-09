import Vue from 'vue';
import VueI18n from 'vue-i18n';
import { messages } from '../lang';
import {
  normalizeLocale, slavicPlural, polishPlural, czechPlural,
} from '../lib/locale';

Vue.use(VueI18n);

const locale = normalizeLocale(localStorage.getItem('lang') || navigator.language);

export default new VueI18n({
  fallbackLocale: 'en',
  locale,
  messages,
  pluralizationRules: {
    ru: slavicPlural,
    pl: polishPlural,
    cs: czechPlural,
  },
});
