import { expect } from 'chai';
import {
  LANGUAGE_NAMES, normalizeLocale, slavicPlural, polishPlural, czechPlural,
} from '@/lib/locale';
import { languages } from '@/lang';

describe('locale selection', () => {
  it('offers every translated language', () => {
    expect(Object.keys(LANGUAGE_NAMES).sort()).to.deep.equal([...languages].sort());
  });

  it('resolves browser locales and persisted regional variants', () => {
    const cases = {
      'ru-RU': 'ru',
      'de-DE': 'de',
      'pt-BR': 'pt_br',
      'pt-PT': 'pt',
      'zh-Hant-HK': 'zh_tw',
      'zh-HK': 'zh_tw',
      zh: 'zh_cn',
      'zh-Hans': 'zh_cn',
      'ja-JP': 'ja',
      'ko-KR': 'ko',
      'uk-UA': 'en',
      uk: 'en',
      unknown: 'en',
      '': 'en',
    };
    Object.entries(cases).forEach(([input, expected]) => {
      expect(normalizeLocale(input), input).to.equal(expected);
    });
    languages.forEach((locale) => expect(normalizeLocale(locale)).to.equal(locale));
  });

  it('selects the correct count forms for Slavic languages', () => {
    expect([0, 1, 2, 5, 11, 21, 22, 25, 101, 111].map((n) => slavicPlural(n, 3)))
      .to.deep.equal([2, 0, 1, 2, 2, 0, 1, 2, 0, 2]);
    expect([1, 2, 5, 21, 22].map((n) => polishPlural(n, 3))).to.deep.equal([0, 1, 2, 2, 1]);
    expect([1, 2, 4, 5, 21].map((n) => czechPlural(n, 3))).to.deep.equal([0, 1, 1, 2, 2]);
  });
});
