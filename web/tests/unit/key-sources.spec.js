import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import axios from 'axios';
import KeySourcesForm from '@/components/KeySourcesForm.vue';
import EnvironmentForm from '@/components/EnvironmentForm.vue';

const settle = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Variable group key sources', () => {
  let get;
  let post;
  beforeEach(() => {
    get = axios.get; post = axios.post;
    axios.get = async () => ({ data: [{ id: 1, name: 'database', type: 'object' }] });
    axios.post = async () => ({ data: { paths: ['', '.username', '.password', '["dns.servers"][0]'] } });
  });
  afterEach(() => { axios.get = get; axios.post = post; });

  it('loads current field paths only and rejects repeated prefixes', async () => {
    const wrapper = shallowMount(KeySourcesForm, {
      propsData: { projectId: 1, value: [{ key_id: 1, prefix: 'test' }] },
      mocks: { $t: (key) => key },
    });
    try {
      await settle();
      expect(wrapper.vm.options).to.deep.equal(['{{ test }}', '{{ test.username }}', '{{ test.password }}', '{{ test["dns.servers"][0] }}']);
      await wrapper.setProps({ value: [{ key_id: 1, prefix: 'test' }, { key_id: 2, prefix: 'test' }] });
      expect(wrapper.vm.prefixRule('test', 1)).to.equal('keySourceDuplicate');
      expect(wrapper.vm.prefixRule('bad-prefix', 0)).to.equal('keySourceInvalid');
      expect(wrapper.vm.prefixRule('unique', 1)).to.equal(true);
      axios.post = async () => ({ data: { paths: ['', '.rotated'] } });
      await wrapper.vm.loadFields(1, true);
      await wrapper.setProps({ value: [{ key_id: 1, prefix: 'renamed' }] });
      expect(wrapper.vm.options).to.deep.equal(['{{ renamed }}', '{{ renamed.rotated }}']);
    } finally { wrapper.destroy(); }
  });

  it('keeps expressions through JSON/YAML switching and separates secret references from literal secrets', async () => {
    const wrapper = shallowMount(EnvironmentForm, {
      propsData: { projectId: 1, itemId: 'new' },
      mocks: { $t: (key) => key, $vuetify: { theme: { dark: true } } },
    });
    try {
      await settle();
      wrapper.vm.item.key_sources = [{ key_id: 1, prefix: 'test' }];
      wrapper.vm.extraVars = [{ name: 'database_user', type: 'string', value: '{{ test.username }}' }];
      wrapper.vm.extraVarsEditMode = 'json';
      expect(JSON.parse(wrapper.vm.json)).to.deep.equal({ database_user: '{{ test.username }}' });
      wrapper.vm.extraVarsEditMode = 'yaml';
      wrapper.vm.extraVarsEditMode = 'table';
      expect(wrapper.vm.extraVars[0].value).to.equal('{{ test.username }}');
      wrapper.vm.env = [{ name: 'DATABASE_URL', value: 'db://{{ test.username }}@host' }];
      wrapper.vm.secrets = [
        {
          name: 'PASSWORD', type: 'env', value: '{{ test.password }}', new: true,
        },
        {
          name: 'literal', type: 'var', value: 'test.password', new: true,
        },
      ];
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.secret_expressions).to.deep.equal([{ name: 'PASSWORD', type: 'env', expression: '{{ test.password }}' }]);
      expect(wrapper.vm.item.secrets).to.deep.equal([{
        id: undefined, name: 'literal', type: 'var', secret: 'test.password', operation: 'create',
      }]);
      expect(wrapper.vm.item.key_sources).to.deep.equal([{ key_id: 1, prefix: 'test' }]);
      wrapper.vm.item.secrets = [{ id: 7, name: 'literal', type: 'var' }];
      await wrapper.vm.afterLoadData();
      expect(wrapper.vm.secrets.find((secret) => secret.name === 'PASSWORD').value).to.equal('{{ test.password }}');
      expect(wrapper.vm.secrets.find((secret) => secret.name === 'literal').value).to.equal('');
      wrapper.vm.removeSecret(wrapper.vm.secrets.find((secret) => secret.name === 'PASSWORD'));
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.secret_expressions).to.deep.equal([]);
      expect(wrapper.vm.item.secrets).to.have.length(1);
    } finally { wrapper.destroy(); }
  });
});
