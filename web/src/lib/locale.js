// Locale IDs also match persisted language preferences.
export const LANGUAGE_NAMES = {
  en: 'English',
  cs: 'Čeština',
  de: 'Deutsch',
  es: 'Español',
  fr: 'Français',
  it: 'Italiano',
  ja: '日本語',
  ko: '한국어',
  nl: 'Nederlands',
  pl: 'Polski',
  pt: 'Português',
  pt_br: 'Português do Brasil',
  ru: 'Русский',
  zh_cn: '简体中文',
  zh_tw: '繁體中文',
};

export function normalizeLocale(value) {
  const locale = String(value || '').replace(/-/g, '_').toLowerCase();
  if (LANGUAGE_NAMES[locale]) return locale;
  if (/^zh_(tw|hk|mo|hant)(_|$)/.test(locale)) return 'zh_tw';
  if (/^zh(_|$)/.test(locale)) return 'zh_cn';
  return LANGUAGE_NAMES[locale.split('_')[0]] ? locale.split('_')[0] : 'en';
}

export function slavicPlural(choice, choicesLength) {
  if (choicesLength < 3) return choice === 1 ? 0 : 1;
  const n = Math.abs(choice);
  if (n % 10 === 1 && n % 100 !== 11) return 0;
  if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14)) return 1;
  return 2;
}

export function polishPlural(choice, choicesLength) {
  if (choice === 1) return 0;
  if (choicesLength < 3) return 1;
  return slavicPlural(choice, choicesLength) === 1 ? 1 : 2;
}

export function czechPlural(choice, choicesLength) {
  if (choice === 1) return 0;
  if (choicesLength < 3) return 1;
  return choice >= 2 && choice <= 4 ? 1 : 2;
}
