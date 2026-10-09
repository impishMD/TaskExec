import { expect } from 'chai';
import i18n from '@/plugins/i18';
import { getErrorMessage } from '@/lib/error';
import { rowToVarValue } from '@/lib/extraVars';

describe('localized UI errors', () => {
  let previousLocale;
  beforeEach(() => {
    previousLocale = i18n.locale;
    i18n.locale = 'ru';
  });
  afterEach(() => {
    i18n.locale = previousLocale;
  });

  [
    ['number', 'abc', 'число'],
    ['list', '{', 'список JSON'],
    ['dict', '[]', 'объект JSON'],
  ].forEach(([type, value, label]) => {
    it(`localizes ${type} validation and preserves the variable name`, () => {
      let error;
      try {
        rowToVarValue({ name: 'deploy_targets', type, value });
      } catch (err) {
        error = err;
      }
      expect(error).to.be.instanceOf(Error);
      expect(getErrorMessage(error)).to.include('deploy_targets').and.include(label);
    });
  });

  it('localizes transport errors', () => {
    expect(getErrorMessage(new Error('Network Error'))).to.equal(i18n.t('networkError'));
    expect(getErrorMessage({ code: 'ETIMEDOUT' })).to.equal(i18n.t('requestTimeout'));
    expect(getErrorMessage({ response: { status: 503 } }))
      .to.equal(i18n.t('httpError', { status: 503 }));
  });

  it('preserves diagnostic details supplied by the server', () => {
    const error = { response: { status: 400, data: { error: 'command exited with code 2' } } };
    expect(getErrorMessage(error)).to.equal('command exited with code 2');
  });
});

describe('Vault key validation localization', () => {
  it('translates known backend validation messages in the selected language', () => {
    const previous = i18n.locale;
    try {
      i18n.locale = 'ru';
      const message = getErrorMessage({
        response: {
          status: 400,
          data: {
            error: 'cannot modify secret in read-only storage',
          },
        },
      });
      expect(message).to.equal('Хранилище доступно только для чтения. Изменять значения секретов нельзя.');
      expect(getErrorMessage({
        response: {
          status: 400,
          data: {
            error: 'reference-only key cannot contain secret values',
          },
        },
      })).to.equal(i18n.t('keyReferenceValuesError'));
    } finally { i18n.locale = previous; }
  });
});
