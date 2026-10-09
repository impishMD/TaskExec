import { expect } from 'chai';
import { mount, shallowMount } from '@vue/test-utils';
import Vuetify from 'vuetify';
import SecretStorageForm from '@/components/SecretStorageForm.vue';
import VaultCredentialField from '@/components/VaultCredentialField.vue';
import DisplayLabelsMixin from '@/components/DisplayLabelsMixin';

const mountForm = (options = {}) => shallowMount(SecretStorageForm, {
  vuetify: new Vuetify(),
  propsData: {
    itemId: 'new', itemType: 'vault', projectId: 1, ...options.props,
  },
  mocks: { $t: (key) => key },
  ...options.mount,
});
const settled = async (wrapper) => {
  await wrapper.vm.$nextTick();
  await wrapper.vm.$nextTick();
};
const choose = async (wrapper, method) => {
  const form = wrapper.vm.$refs.form;
  form.resetValidation = () => {};
  await wrapper.find(`[data-testid="vault-auth-${method}"]`).trigger('click');
  await settled(wrapper);
};

describe('Vault authentication form', () => {
  it('defaults to read-only KV v2 with token auth', async () => {
    const wrapper = mountForm();
    try {
      await settled(wrapper);
      expect(wrapper.vm.item.type).to.equal('vault');
      expect(wrapper.vm.item.readonly).to.equal(true);
      expect(wrapper.vm.item.params.kv_version).to.equal(2);
      expect(wrapper.vm.authMethod).to.equal('token');
      expect(wrapper.findAll('[role="radio"]').length).to.equal(5);
      expect(wrapper.vm.credentialFields.map((f) => f.name)).to.deep.equal(['token']);
      expect(DisplayLabelsMixin.methods.storageTypeTitle('vault')).to.equal('HashiCorp Vault');
    } finally { wrapper.destroy(); }
  });

  it('shows only relevant credentials when switching methods', async () => {
    const wrapper = mountForm();
    try {
      await settled(wrapper);
      wrapper.vm.item.credentials.token.value = 'unsaved-token';
      await choose(wrapper, 'approle');
      expect(wrapper.vm.credentialFields.map((f) => f.name)).to.deep.equal(['role_id', 'secret_id']);
      expect(wrapper.vm.item.credentials).not.to.have.property('token');
      expect(wrapper.vm.item.params.auth_mount).to.equal('approle');
      await choose(wrapper, 'kubernetes');
      expect(wrapper.vm.item.credentials.jwt.source).to.equal('file');
      expect(wrapper.find('[data-testid="vault-auth-role"]').exists()).to.equal(true);
      await choose(wrapper, 'jwt');
      expect(wrapper.vm.credentialFields.map((f) => f.name)).to.deep.equal(['jwt']);
      expect(wrapper.vm.item.credentials.jwt.value).to.equal('');
      await choose(wrapper, 'cert');
      expect(wrapper.vm.credentialFields.map((f) => f.name)).to.deep.equal(['client_cert', 'client_key']);
      expect(wrapper.vm.credentialFields.every((f) => f.multiline)).to.equal(true);
      await choose(wrapper, 'token');
      expect(wrapper.find('[data-testid="vault-auth-role"]').exists()).to.equal(false);
    } finally { wrapper.destroy(); }
  });

  it('loads Vault file references without losing their source', async () => {
    const wrapper = mountForm({
      props: { itemId: 10 },
      mount: {
        methods: {
          loadEndpoint: async () => ({
            id: 10,
            type: 'vault',
            readonly: false,
            source_storage_type: 'file',
            secret: '/secrets/vault-token',
            params: { kv_version: 1 },
          }),
        },
      },
    });
    try {
      await settled(wrapper);
      await settled(wrapper);
      expect(wrapper.vm.item.type).to.equal('vault');
      expect(wrapper.vm.item.credentials.token.value).to.equal('/secrets/vault-token');
      expect(wrapper.vm.item.credentials.token.source).to.equal('file');
      expect(wrapper.vm.item.params.kv_version).to.equal(1);
      expect(wrapper.vm.item.readonly).to.equal(false);
      expect(wrapper.vm.item.secret).to.equal(undefined);
    } finally { wrapper.destroy(); }
  });

  it('restores masked credentials and custom mount when returning to the saved method', async () => {
    const wrapper = mountForm({
      props: { itemId: 10 },
      mount: {
        methods: {
          loadEndpoint: async () => ({
            id: 10,
            type: 'vault',
            params: { auth_method: 'jwt', auth_mount: 'custom', auth_role: 'role' },
            credentials: { jwt: { source: 'database', configured: true } },
          }),
        },
      },
    });
    try {
      await settled(wrapper);
      await settled(wrapper);
      await choose(wrapper, 'approle');
      expect(wrapper.vm.item.credentials.secret_id.configured).to.equal(false);
      await choose(wrapper, 'jwt');
      expect(wrapper.vm.item.credentials.jwt.configured).to.equal(true);
      expect(wrapper.vm.item.params.auth_mount).to.equal('custom');
      expect(wrapper.vm.item.params.auth_role).to.equal('role');
    } finally { wrapper.destroy(); }
  });

  it('explicitly clears an optional CA or Secret ID when disabled', async () => {
    const wrapper = mountForm();
    try {
      await settled(wrapper);
      await choose(wrapper, 'approle');
      wrapper.vm.$set(wrapper.vm.item.params, 'without_secret_id', true);
      await settled(wrapper);
      expect(wrapper.vm.credentialFields.map((f) => f.name)).to.deep.equal(['role_id']);
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.credentials.secret_id.clear).to.equal(true);
      expect(wrapper.vm.item.credentials.ca_cert.clear).to.equal(true);
    } finally { wrapper.destroy(); }
  });
});

describe('Vault credential field', () => {
  const mountField = (value) => shallowMount(VaultCredentialField, {
    propsData: { value, field: 'jwt', label: 'JWT' },
    mocks: { $t: (key) => key },
  });
  it('preserves a masked value but requires a new reference after changing sources', () => {
    const wrapper = mountField({ source: 'database', configured: true, value: '' });
    try {
      expect(wrapper.vm.rules[0]('')).to.equal(true);
      wrapper.vm.changeSource('file');
      expect(wrapper.emitted('input')[0][0]).to.deep.equal({
        source: 'file', value: '', configured: false,
      });
    } finally { wrapper.destroy(); }
  });
  it('uses the file label and hint and cancels a previous clear on input', () => {
    const wrapper = mountField({ source: 'file', value: '', clear: true });
    try {
      expect(wrapper.vm.inputLabel).to.equal('uiPathToTheFile');
      expect(wrapper.vm.hint).to.equal('vaultFileHint');
      expect(wrapper.vm.rules[0]('')).to.equal('valueRequired');
      wrapper.vm.changeValue('/secrets/token');
      expect(wrapper.emitted('input')[0][0].clear).to.equal(false);
    } finally { wrapper.destroy(); }
  });
});

describe('Vault credential input rendering', () => {
  it('renders actual Vuetify inputs for password, file and PEM values', async () => {
    const wrapper = mount(VaultCredentialField, {
      vuetify: new Vuetify(),
      propsData: { value: { source: 'database', value: '' }, field: 'client_key', label: 'Key' },
      mocks: { $t: (key) => key },
    });
    try {
      expect(wrapper.find('input[type="password"]').exists()).to.equal(true);
      await wrapper.setProps({ multiline: true });
      expect(wrapper.find('textarea').exists()).to.equal(true);
      await wrapper.setProps({ value: { source: 'file', value: '/secrets/key' } });
      expect(wrapper.find('textarea').exists()).to.equal(false);
      expect(wrapper.find('input[data-testid="vault-credential-client_key"]').element.value)
        .to.equal('/secrets/key');
    } finally { wrapper.destroy(); }
  });
});
