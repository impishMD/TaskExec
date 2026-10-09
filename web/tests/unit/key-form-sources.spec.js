import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import Vuetify from 'vuetify';
import KeyForm from '@/components/KeyForm.vue';

describe('KeyForm source tabs', () => {
  [false, true].forEach((supportStorages) => {
    it(`selects the source matching the visible tab (external storage: ${supportStorages})`, async () => {
      const wrapper = shallowMount(KeyForm, {
        vuetify: new Vuetify(),
        propsData: { itemId: 'new', projectId: 1, supportStorages },
        mocks: { $t: (key) => key },
        methods: { beforeLoadData() { this.secretStorages = []; } },
      });
      try {
        await wrapper.vm.$nextTick();
        await wrapper.vm.$nextTick();
        const tabs = wrapper.findAll('v-tab-stub').wrappers.map((tab) => tab.text());
        expect(tabs).to.deep.equal(supportStorages
          ? ['uiLocal', 'uiStorage', 'uiEnv', 'uiFile'] : ['uiLocal', 'uiEnv', 'uiFile']);
        const select = async (label, source) => {
          // Vuetify emits the visible index, which changes when the storage tab is hidden.
          wrapper.find('v-tabs-stub').vm.$emit('change', tabs.indexOf(label));
          await wrapper.vm.$nextTick();
          expect(wrapper.vm.item.source_storage_type).to.equal(source);
        };
        await select('uiEnv', 'env');
        await select('uiFile', 'file');
        await select('uiLocal', undefined);
      } finally {
        wrapper.destroy();
      }
    });
  });
});

describe('Editing key sources', () => {
  const makeForm = (overrides = {}) => shallowMount(KeyForm, {
    vuetify: new Vuetify(),
    propsData: { itemId: 7, projectId: 1, supportStorages: true },
    mocks: { $t: (key) => key },
    methods: {
      loadProjectResources: async () => [{ id: 2, readonly: false }],
      loadEndpoint: async () => ({
        id: 7,
        name: 'existing',
        type: 'login_password',
        ssh: {},
        login_password: {},
        ...overrides,
      }),
    },
  });
  const settle = async (wrapper) => {
    await wrapper.vm.$nextTick();
    await wrapper.vm.$nextTick();
    await wrapper.vm.$nextTick();
  };
  const selectSource = async (wrapper, label) => {
    const tabs = wrapper.findAll('v-tab-stub').wrappers;
    expect(tabs.every((tab) => tab.attributes('disabled') !== 'true')).to.equal(true);
    wrapper.find('v-tabs-stub').vm.$emit('change', tabs.findIndex((tab) => tab.text() === label));
    await wrapper.vm.$nextTick();
  };
  it('switches a saved local key to every reference source without override', async () => {
    const wrapper = makeForm();
    try {
      await settle(wrapper);
      expect(wrapper.vm.canEditSecrets).to.equal(false);
      expect(wrapper.find('v-checkbox-stub').exists()).to.equal(true);
      await selectSource(wrapper, 'uiStorage');
      expect(wrapper.find('v-autocomplete-stub').attributes('disabled')).to.equal(undefined);
      expect(wrapper.find('v-checkbox-stub').exists()).to.equal(false);
      wrapper.vm.item.source_storage_id = 2;
      wrapper.vm.item.source_storage_key = 'proxmox#dns_server';
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.reference_only).to.equal(true);
      expect(wrapper.vm.item.override_secret).to.equal(false);
      await selectSource(wrapper, 'uiEnv');
      expect(wrapper.vm.item.source_storage_key).to.equal('');
      wrapper.vm.item.source_storage_key = 'TASKEXEC_TEST_SECRET';
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.override_secret).to.equal(true);
      expect(wrapper.vm.item.source_storage_id).to.equal(null);
      await selectSource(wrapper, 'uiFile');
      expect(wrapper.vm.item.source_storage_key).to.equal('');
      wrapper.vm.item.source_storage_key = 'test-secret';
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.override_secret).to.equal(true);
      await selectSource(wrapper, 'uiEnv');
      expect(wrapper.vm.item.source_storage_key).to.equal('TASKEXEC_TEST_SECRET');
      // Even after a failed save, returning to the original source must preserve its secret.
      await selectSource(wrapper, 'uiLocal');
      expect(wrapper.vm.canEditSecrets).to.equal(false);
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.override_secret).to.equal(false);
      expect(wrapper.vm.item.reference_only).to.equal(false);
    } finally { wrapper.destroy(); }
  });
  ['vault', 'env', 'file'].forEach((source) => {
    it(`accepts a new local value when switching from ${source}, without an override checkbox`, async () => {
      const wrapper = makeForm({
        source_storage_type: source, source_storage_id: 2, source_storage_key: 'old-reference',
      });
      try {
        await settle(wrapper);
        await selectSource(wrapper, 'uiLocal');
        expect(wrapper.find('v-checkbox-stub').exists()).to.equal(false);
        const password = wrapper.findAll('v-text-field-stub').wrappers
          .find((field) => field.attributes('label') === 'password');
        expect(password.attributes('disabled')).to.equal(undefined);
        expect(password.props('rules')[0]('')).to.equal('password_required');
        wrapper.vm.item.login_password.password = 'new local value';
        wrapper.vm.beforeSave();
        expect(wrapper.vm.item.override_secret).to.equal(true);
        expect(wrapper.vm.item.reference_only).to.equal(false);
        expect(wrapper.vm.item.source_storage_id).to.equal(null);
        expect(wrapper.vm.item.source_storage_key).to.equal(null);
        expect(wrapper.vm.item.login_password.password).to.equal('new local value');
      } finally { wrapper.destroy(); }
    });
  });
  it('keeps local drafts separate and requires explicit override only for the original local secret', async () => {
    const wrapper = makeForm();
    try {
      await settle(wrapper);
      wrapper.find('v-checkbox-stub').vm.$emit('change', true);
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.canEditSecrets).to.equal(true);
      wrapper.vm.item.login_password.password = 'local draft';
      await selectSource(wrapper, 'uiStorage');
      expect(wrapper.vm.item.login_password).to.deep.equal({});
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.override_secret).to.equal(false);
      await selectSource(wrapper, 'uiLocal');
      expect(wrapper.vm.item.login_password.password).to.equal('local draft');
      expect(wrapper.vm.replaceLocalSecret).to.equal(true);
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.override_secret).to.equal(true);
      wrapper.find('v-checkbox-stub').vm.$emit('change', false);
      await wrapper.vm.$nextTick();
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.override_secret).to.equal(false);
      expect(wrapper.vm.canEditSecrets).to.equal(false);
    } finally { wrapper.destroy(); }
  });
});

[true, false].forEach((readonly) => {
  describe(`Vault reference form (read-only: ${readonly})`, () => {
    const makeForm = (overrides = {}) => shallowMount(KeyForm, {
      vuetify: new Vuetify(),
      propsData: { itemId: 7, projectId: 1, supportStorages: true },
      mocks: { $t: (key) => key },
      methods: {
        loadProjectResources: async () => [{ id: 2, readonly }],
        loadEndpoint: async () => ({
          id: 7,
          name: 'test',
          type: 'string',
          source_storage_type: 'vault',
          source_storage_id: 2,
          source_storage_key: 'proxmox#dns_server',
          ssh: {},
          login_password: {},
          ...overrides,
        }),
      },
    });
    const settle = async (wrapper) => {
      await wrapper.vm.$nextTick();
      await wrapper.vm.$nextTick();
      await wrapper.vm.$nextTick();
    };
    it('edits the type without override and never shows remote secret fields', async () => {
      const wrapper = makeForm();
      try {
        await settle(wrapper);
        expect(wrapper.vm.inventoryTypes.map((type) => type.id))
          .to.deep.equal(['string', 'object', 'ssh', 'login_password']);
        // Exercise successive changes in the same open form.
        // eslint-disable-next-line no-restricted-syntax
        for (const type of ['ssh', 'login_password', 'string']) {
          // eslint-disable-next-line no-await-in-loop
          await wrapper.setData({ item: { ...wrapper.vm.item, type } });
          expect(wrapper.find('v-select-stub').attributes('disabled')).to.equal(undefined);
          expect(wrapper.find('v-textarea-stub').exists()).to.equal(false);
          const fields = wrapper.findAll('v-text-field-stub').wrappers.map((field) => field.attributes('label'));
          expect(fields).to.deep.equal(['keyName']);
          expect(wrapper.find('vaultsecretpath-stub').exists()).to.equal(true);
          expect(wrapper.find('v-checkbox-stub').exists()).to.equal(false);
        }
        wrapper.vm.item.ssh.private_key = 'stale value from another source';
        wrapper.vm.item.override_secret = true;
        wrapper.vm.beforeSave();
        expect(wrapper.vm.item.reference_only).to.equal(true);
        expect(wrapper.vm.item.override_secret).to.equal(false);
        expect(wrapper.vm.item.generate_ssh_key).to.equal(false);
        expect(wrapper.vm.item.ssh).to.deep.equal({});
        expect(wrapper.vm.item.string).to.equal('');
      } finally { wrapper.destroy(); }
    });
    it('allows editing references imported by the retired synchronization feature', async () => {
      const wrapper = makeForm({ synchronized: true });
      try {
        await settle(wrapper);
        wrapper.findAll('v-text-field-stub').wrappers.forEach((field) => {
          expect(field.attributes('disabled')).to.equal(undefined);
        });
        expect(wrapper.find('v-select-stub').attributes('disabled')).to.equal(undefined);
      } finally { wrapper.destroy(); }
    });
  });
});
