/* eslint-disable no-restricted-syntax, no-await-in-loop */
import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import EnvironmentForm from '@/components/EnvironmentForm.vue';
import mockAxios from './helpers/axiosMock';

const flush = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Variable group format switching', () => {
  let http;
  let wrapper;
  beforeEach(async () => {
    http = mockAxios();
    http.respond(() => []);
    wrapper = shallowMount(EnvironmentForm, {
      propsData: { projectId: 1, itemId: 'new' },
      stubs: { 'v-form': { template: '<form><slot /></form>', methods: { resetValidation() {} } } },
      mocks: { $t: (key) => key, $vuetify: { theme: { dark: true } } },
    });
    await flush();
  });
  afterEach(() => { wrapper.destroy(); http.restore(); });
  it('switches an empty form in every direction and saves JSON/YAML as an empty object', async () => {
    for (const mode of ['json', 'yaml', 'table', 'yaml', 'json', 'table']) {
      wrapper.vm.extraVarsEditMode = mode;
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.extraVarsEditMode).to.equal(mode);
      expect(wrapper.vm.formError).to.equal(null);
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.json).to.equal('{}');
    }
    for (const mode of ['yaml', 'json']) {
      wrapper.vm.extraVarsEditMode = mode;
      wrapper.vm[mode] = '';
      wrapper.vm.beforeSave();
      expect(wrapper.vm.item.json).to.equal('{}');
    }
  });
  it('does not replace a non-finite table value with null', () => {
    wrapper.vm.extraVars = [{ name: 'count', type: 'number', value: 'Infinity' }];
    wrapper.vm.extraVarsEditMode = 'json';
    expect(wrapper.vm.extraVarsEditMode).to.equal('table');
    expect(wrapper.vm.formError).to.equal('extraVarsFiniteRequired');
  });
  it('keeps malformed input in its original editor, recovers after correction, and resets on reopening', async () => {
    wrapper.vm.extraVarsEditMode = 'yaml';
    wrapper.vm.yaml = '{broken';
    wrapper.vm.extraVarsEditMode = 'json';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.extraVarsEditMode).to.equal('yaml');
    expect(wrapper.vm.yaml).to.equal('{broken');
    expect(wrapper.vm.formError).not.to.equal(null);
    wrapper.vm.yaml = 'host: example.test';
    wrapper.vm.extraVarsEditMode = 'json';
    expect(JSON.parse(wrapper.vm.json)).to.deep.equal({ host: 'example.test' });
    expect(wrapper.vm.formError).to.equal(null);
    wrapper.vm.json = '{broken';
    wrapper.vm.extraVarsEditMode = 'table';
    expect(wrapper.vm.extraVarsEditMode).to.equal('json');
    await wrapper.vm.reset();
    expect(wrapper.vm.extraVarsEditMode).to.equal('table');
    expect(wrapper.vm.formError).to.equal(null);
    expect(wrapper.vm.extraVars).to.deep.equal([]);
    wrapper.vm.extraVarsEditMode = 'yaml';
    expect(wrapper.vm.formError).to.equal(null);
  });
});
