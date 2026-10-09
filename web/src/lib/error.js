import i18n from '@/plugins/i18';

// Translate known form errors; preserve arbitrary backend/command diagnostics.
const formErrors = {
  'invalid project icon': 'projectIconInvalid',
  'invalid key source prefix': 'keySourceInvalid',
  'duplicate key source prefix': 'keySourceDuplicate',
  'invalid key expression': 'keyExpressionInvalid',
  'key source prefix is still referenced': 'keySourceInUse',
  'secret not found in Vault': 'vaultSecretNotFound',
  'Vault request failed; check connectivity, TLS and server availability': 'vaultReadFailed',
  'Vault path must not contain a field selector': 'vaultPathNoField',
  'invalid Vault field mapping': 'vaultMappingInvalid',
  'required Vault field mapping is missing': 'vaultMappingInvalid',
  'invalid key variable binding': 'keyBindingInvalid',
  'duplicate key variable binding': 'keyBindingConflict',
  'key binding conflicts with a variable': 'keyBindingConflict',
  'key type cannot be used by variable binding': 'keyBindingTypeError',
  'key value must be an object': 'extraVarsObjectRequired',
  'cannot modify secret in read-only storage': 'keyReadOnlyError',
  'cannot override secret storage': 'keyStorageChangeError',
  'reference-only key cannot contain secret values': 'keyReferenceValuesError',
  'invalid Vault reference type': 'keyReferenceInvalidError',
  'environment secrets must use string type': 'keyReferenceInvalidError',
  'vault storage id is required': 'keyReferenceInvalidError',
  'Vault secret path is required': 'keyReferenceInvalidError',
  'invalid Vault path': 'keyReferenceInvalidError',
  'invalid Vault field reference': 'keyReferenceInvalidError',
};
// eslint-disable-next-line import/prefer-default-export
export function getErrorMessage(err) {
  if (err.i18nKey) return i18n.t(err.i18nKey, err.i18nParams);
  if (err.code === 'ECONNABORTED' || err.code === 'ETIMEDOUT') {
    return i18n.t('requestTimeout');
  }
  if (err.message === 'Network Error') return i18n.t('networkError');
  if (err.response) {
    if (err.response.data && err.response.data.error) {
      const message = err.response.data.error;
      const vaultStatus = message.match(/^Vault request failed \(HTTP (\d+)\)$/);
      if (vaultStatus) return i18n.t('httpError', { status: vaultStatus[1] });
      const fieldErrors = {
        'invalid environment field name: ': 'keyBindingEnvFieldInvalid',
        'key expression field is missing: ': 'keyExpressionMissing',
        'key binding conflicts with a variable: ': 'keyBindingConflict',
        'Vault mapped field is missing: ': 'vaultFieldMissing',
        'Vault mapped field must be scalar: ': 'vaultFieldScalar',
        'Vault mapped field must be a string: ': 'vaultFieldString',
        'Vault mapped field must not be empty: ': 'vaultFieldEmpty',
      };
      const prefix = Object.keys(fieldErrors).find((key) => message.startsWith(key));
      if (prefix) return i18n.t(fieldErrors[prefix], { field: message.slice(prefix.length) });
      return Object.prototype.hasOwnProperty.call(formErrors, message)
        ? i18n.t(formErrors[message]) : message;
    }

    if (err.message && !err.message.startsWith('Request failed with status code ')) {
      return err.message;
    }

    return i18n.t('httpError', { status: err.response.status });
  }

  return err.message;
}
