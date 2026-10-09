import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import axios from 'axios';
import VaultKeyMapping from '@/components/VaultKeyMapping.vue';
import KeyBindingsForm from '@/components/KeyBindingsForm.vue';

const settle = async (wrapper) => {
  await wrapper.vm.$nextTick();
  await wrapper.vm.$nextTick();
  await wrapper.vm.$nextTick();
};

describe('Vault field mappings and live variable values', () => {
  let originalGet;
  let originalPost;
  beforeEach(() => { originalGet = axios.get; originalPost = axios.post; });
  afterEach(() => { axios.get = originalGet; axios.post = originalPost; });

  it('loads only field names/types and validates required and optional credential mappings', async () => {
    axios.post = async (url, body) => {
      expect(url).to.equal('/api/project/1/secret_storages/2/fields');
      expect(body).to.deep.equal({ path: 'proxmox' });
      return { data: { fields: [{ name: 'user', type: 'string' }, { name: 'pem', type: 'string' }, { name: 'nested', type: 'object' }] } };
    };
    const wrapper = shallowMount(VaultKeyMapping, {
      propsData: {
        projectId: 1, storageId: 2, path: 'proxmox', type: 'ssh', value: { private_key: 'pem' },
      },
      mocks: { $t: (key) => key },
    });
    try {
      await settle(wrapper);
      expect(wrapper.vm.targets.map((t) => t.id)).to.deep.equal(['login', 'passphrase', 'private_key']);
      expect(wrapper.vm.options('private_key').map((o) => o.value)).to.deep.equal(['user', 'pem']);
      expect(wrapper.vm.validateField({ required: true }, 'missing')).to.equal('vaultFieldMissing');
      expect(wrapper.vm.validateField({ required: true }, 'nested')).to.equal('vaultFieldString');
      expect(wrapper.vm.validateField({ required: false }, null)).to.equal(true);
      expect(wrapper.vm.validateField({ required: true }, null)).to.equal('required');
      wrapper.vm.setField('login', 'user');
      expect(wrapper.emitted('input').pop()[0]).to.deep.equal({ private_key: 'pem', login: 'user' });
      await wrapper.setProps({ type: 'object' });
      expect(wrapper.vm.targets).to.have.length(0);
      expect(wrapper.emitted('input').pop()[0]).to.deep.equal({});
    } finally { wrapper.destroy(); }
  });

  it('automatically loads the completed path, skips directories and ignores stale responses', async () => {
    const requests = [];
    axios.post = (url, body) => new Promise((resolve) => {
      requests.push({ url, path: body.path, resolve });
    });
    const wrapper = shallowMount(VaultKeyMapping, {
      propsData: {
        projectId: 1, storageId: 2, path: '', type: 'string',
      },
      mocks: { $t: (key) => key },
    });
    const debounce = () => new Promise((resolve) => { setTimeout(resolve, 280); });
    try {
      expect(wrapper.find('v-btn-stub').exists()).to.equal(false);
      await wrapper.setProps({ path: 'apps/' });
      await debounce();
      expect(requests).to.have.length(0);
      await wrapper.setProps({ path: 'apps/d' });
      await wrapper.setProps({ path: 'apps/db' });
      await debounce();
      expect(requests.map((r) => r.path)).to.deep.equal(['apps/db']);
      await wrapper.setProps({ path: 'apps/ci' });
      requests[0].resolve({ data: { fields: [{ name: 'old', type: 'string' }] } });
      await settle(wrapper);
      expect(wrapper.vm.fields).to.deep.equal([]);
      await debounce();
      requests[1].resolve({ data: { fields: [{ name: 'token', type: 'string' }] } });
      await settle(wrapper);
      expect(wrapper.vm.options('value').map((o) => o.value)).to.deep.equal(['token']);
      expect(wrapper.vm.loaded).to.equal(true);
      expect(wrapper.vm.loading).to.equal(false);
      await wrapper.setProps({ projectId: 3, storageId: 4 });
      await debounce();
      expect(requests[2].url).to.equal('/api/project/3/secret_storages/4/fields');
      await wrapper.setProps({ path: '' });
      requests[2].resolve({ data: { fields: [{ name: 'late', type: 'string' }] } });
      await settle(wrapper);
      expect(wrapper.vm.fields).to.deep.equal([]);
      expect(wrapper.vm.loading).to.equal(false);
      await wrapper.setProps({ path: 'apps/pending' });
      wrapper.destroy();
      await debounce();
      expect(requests).to.have.length(3);
    } finally { wrapper.destroy(); }
  });

  it('retries a failed automatic load when the field selector is focused', async () => {
    axios.post = async () => { throw new Error('offline'); };
    const wrapper = shallowMount(VaultKeyMapping, {
      propsData: {
        projectId: 1, storageId: 2, path: 'proxmox', type: 'string',
      },
      mocks: { $t: (key) => key },
    });
    try {
      await settle(wrapper);
      expect(wrapper.vm.error).to.equal('offline');
      axios.post = async () => ({ data: { fields: [{ name: 'dns_server', type: 'string' }] } });
      wrapper.find('v-autocomplete-stub').vm.$emit('focus');
      await settle(wrapper);
      expect(wrapper.vm.fields.map((f) => f.name)).to.deep.equal(['dns_server']);
      expect(wrapper.vm.error).to.equal(null);
    } finally { wrapper.destroy(); }
  });

  it('refreshes actual values, preserves structured data and emits only references', async () => {
    axios.get = async () => ({ data: [{ id: 7, name: 'blabla', type: 'object' }, { id: 8, name: 'ssh', type: 'ssh' }] });
    let revision = 1;
    axios.post = async (url) => {
      expect(url).to.equal('/api/project/1/keys/7/preview');
      return { data: { value: { dns_server: `192.0.2.${revision}`, nested: { enabled: true } }, read_at: '2026-10-09T10:00:00Z' } };
    };
    const binding = {
      key_id: 7, name: 'blabla', type: 'var', field: null,
    };
    const wrapper = shallowMount(KeyBindingsForm, {
      propsData: { projectId: 1, value: [binding] }, mocks: { $t: (key) => key },
    });
    try {
      await settle(wrapper);
      expect(wrapper.vm.keys).to.have.length(1);
      expect(wrapper.vm.expression(binding)).to.equal('{{ blabla.dns_server }}');
      expect(wrapper.vm.preview(binding)).to.include('192.0.2.1');
      expect(wrapper.vm.fields(binding).find((o) => o.value === 'dns_server').text).to.include('192.0.2.1');
      const env = { ...binding, type: 'env' };
      expect(wrapper.vm.preview(env)).to.equal('$blabla\n$blabla_dns_server\n$blabla_nested');
      expect(wrapper.vm.fields(env).find((o) => o.value === 'dns_server').text).to.equal('dns_server');
      expect(wrapper.vm.preview({ ...env, field: 'dns_server' })).to.equal('$blabla');
      wrapper.vm.live[7].value['bad-name'] = 'not-for-output';
      expect(wrapper.vm.environmentError(env)).to.equal('keyBindingEnvFieldInvalid');
      delete wrapper.vm.live[7].value['bad-name'];
      wrapper.vm.live[7].value.JWT = 'not-for-output';
      expect(wrapper.vm.environmentError({ ...env, name: 'TASKEXEC' })).to.equal('keyBindingEnvFieldInvalid');
      delete wrapper.vm.live[7].value.JWT;
      revision = 2;
      await wrapper.vm.refresh(7);
      expect(wrapper.vm.preview(binding)).to.include('192.0.2.2');
      wrapper.vm.update(0, { field: 'dns_server' });
      expect(wrapper.emitted('input').pop()[0]).to.deep.equal([{ ...binding, field: 'dns_server' }]);
      expect(wrapper.vm.expression({ ...binding, field: 'dns_server' })).to.equal('{{ blabla }}');
      expect(wrapper.vm.missingField({ ...binding, field: 'removed' })).to.equal(true);
      axios.post = async () => { throw new Error('offline'); };
      await wrapper.vm.refresh(7);
      expect(wrapper.vm.state(binding)).not.to.have.property('value');
      expect(wrapper.vm.state(binding).error).to.equal('offline');
    } finally { wrapper.destroy(); }
  });
});
