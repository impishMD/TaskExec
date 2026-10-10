import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import Vuetify from 'vuetify';
import TemplateVaults from '@/components/TemplateVaults.vue';
import mockAxios from './helpers/axiosMock';

describe('Ansible Vault keys', () => {
  let http;
  beforeEach(() => {
    http = mockAxios();
    http.respond(() => [
      { id: 1, name: 'Vault string', type: 'string' },
      { id: 2, name: 'Vault password', type: 'login_password' },
      { id: 3, name: 'SSH', type: 'ssh' },
      { id: 4, name: 'Object', type: 'object' },
      { id: 5, name: 'None', type: 'none' },
    ]);
  });
  afterEach(() => http.restore());

  [1, 2].forEach((keyId) => {
    it(`selects, saves and reopens vault key ${keyId}`, async () => {
      const wrapper = shallowMount(TemplateVaults, {
        vuetify: new Vuetify(),
        propsData: { projectId: 1, vaults: [] },
        mocks: { $t: (key) => key },
        stubs: {
          VForm: {
            template: '<form><slot /></form>',
            methods: { validate: () => true, resetValidation: () => {} },
          },
        },
      });
      try {
        await new Promise((resolve) => { setTimeout(resolve, 0); });
        expect(http.requests[0].url).to.equal('/api/project/1/keys');
        wrapper.find('v-chip-stub').vm.$emit('click');
        await wrapper.vm.$nextTick();
        const select = wrapper.findAll('v-select-stub').at(1);
        expect(select.props('items').map((key) => key.id)).to.deep.equal([1, 2]);
        select.vm.$emit('input', keyId);
        await wrapper.vm.$nextTick();
        wrapper.findAll('v-btn-stub').at(1).vm.$emit('click');
        await wrapper.vm.$nextTick();
        expect(wrapper.emitted('change')[0][0]).to.deep.equal([{
          name: null, type: 'password', vault_key_id: keyId, script: null,
        }]);
        wrapper.find('v-chip-stub').vm.$emit('click');
        await wrapper.vm.$nextTick();
        expect(wrapper.findAll('v-select-stub').at(1).props('value')).to.equal(keyId);
      } finally { wrapper.destroy(); }
    });
  });
});
