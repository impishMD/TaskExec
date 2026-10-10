import { expect } from 'chai';
import { mount } from '@vue/test-utils';
import axios from 'axios';
import Vuetify from 'vuetify';
import ProjectTokens from '@/components/ProjectTokens.vue';

const tick = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Project tokens', () => {
  let wrapper;
  let original;
  let calls;
  const item = {
    id: 'test-token-id',
    name: 'CI',
    scopes: ['tasks:run'],
    template_ids: [7],
    overrides: [],
    all_templates: false,
    creator_name: 'owner',
    created: '2026-01-01T00:00:00Z',
  };
  beforeEach(() => {
    original = { get: axios.get, post: axios.post, delete: axios.delete };
    calls = [];
    axios.get = async (url) => ({ data: url.endsWith('/templates') ? [{ id: 7, name: 'Deploy' }] : [item] });
    axios.post = async (url, body) => {
      calls.push({ url, body });
      return { data: { ...item, token: 'texp_one-time-secret' } };
    };
    axios.delete = async (url) => { calls.push({ url }); };
  });
  afterEach(() => {
    if (wrapper) wrapper.destroy();
    Object.assign(axios, original);
  });
  async function render() {
    wrapper = mount(ProjectTokens, {
      propsData: { projectId: 1 },
      vuetify: new Vuetify(),
      mocks: { $t: (key) => key },
      stubs: { 'v-dialog': { template: '<div><slot /></div>' } },
    });
    await tick();
    wrapper.vm.openCreate();
    await tick();
  }
  it('requires a name, permissions and selected templates by default', async () => {
    await render();
    expect(wrapper.vm.form.all_templates).to.equal(false);
    await wrapper.vm.create();
    expect(calls).to.have.length(0);
    wrapper.vm.form.name = 'CI';
    await tick();
    await wrapper.vm.create();
    expect(calls).to.have.length(0);
    wrapper.vm.form.template_ids = [7];
    await tick();
    await wrapper.vm.create();
    expect(calls).to.have.length(1);
    expect(calls[0].body.template_ids).to.deep.equal([7]);
    expect(calls[0].body.overrides).to.deep.equal([]);
  });
  it('shows the secret once and clears it when the dialog closes', async () => {
    await render();
    wrapper.vm.form.name = 'CI';
    wrapper.vm.form.all_templates = true;
    await tick();
    await wrapper.vm.create();
    expect(wrapper.vm.issuedToken).to.equal('texp_one-time-secret');
    expect(wrapper.vm.items[0]).not.to.have.property('token');
    wrapper.vm.dialog = false;
    await tick();
    expect(wrapper.vm.issuedToken).to.equal('');
  });
  it('removes launch overrides when switching to read-only permissions', async () => {
    await render();
    wrapper.vm.form.name = 'CI';
    wrapper.vm.form.all_templates = true;
    wrapper.vm.form.template_ids = [7];
    wrapper.vm.form.overrides = ['git_branch'];
    wrapper.vm.applyPreset('read');
    await tick();
    await wrapper.vm.create();
    expect(calls[0].body.scopes).not.to.include('tasks:run');
    expect(calls[0].body.overrides).to.deep.equal([]);
    expect(calls[0].body.template_ids).to.deep.equal([]);
  });
  it('rotates with one atomic request and preserves errors without losing the form', async () => {
    await render();
    wrapper.vm.openCreate(item);
    await tick();
    axios.post = async (url) => { calls.push({ url }); throw new Error('unavailable'); };
    await wrapper.vm.create();
    expect(calls[0].url).to.equal('/api/project/1/tokens/test-token-id/rotate');
    expect(wrapper.vm.formError).to.equal('unavailable');
    expect(wrapper.vm.dialog).to.equal(true);
    expect(wrapper.vm.issuedToken).to.equal('');
    expect(wrapper.vm.busy).to.equal(false);
  });
  it('does not issue duplicate requests while creating', async () => {
    await render();
    wrapper.vm.form.name = 'CI';
    wrapper.vm.form.all_templates = true;
    await tick();
    let finish;
    axios.post = (url) => {
      calls.push({ url });
      return new Promise((resolve) => { finish = resolve; });
    };
    const pending = wrapper.vm.create();
    await tick();
    await wrapper.vm.create();
    expect(calls).to.have.length(1);
    finish({ data: { ...item, token: 'secret' } });
    await pending;
  });
  it('revokes the selected project token without deleting its history', async () => {
    await render();
    wrapper.vm.revokeTarget = item;
    await wrapper.vm.revoke();
    expect(calls).to.deep.equal([{ url: '/api/project/1/tokens/test-token-id' }]);
    expect(wrapper.vm.revokeTarget).to.equal(null);
  });
  it('filters active and revoked tokens, keeping expired tokens in All', async () => {
    await render();
    wrapper.vm.dialog = false;
    const tokens = [
      { ...item, id: 'active', name: 'Active token' },
      {
        ...item, id: 'future', name: 'Future expiry', expires_at: new Date(Date.now() + 86400000).toISOString(),
      },
      {
        ...item, id: 'expired', name: 'Expired token', expires_at: '2020-01-01T00:00:00Z',
      },
      {
        ...item, id: 'revoked', name: 'Revoked token', revoked_at: '2020-01-01T00:00:00Z',
      },
      {
        ...item,
        id: 'both',
        name: 'Revoked and expired',
        revoked_at: '2020-01-01T00:00:00Z',
        expires_at: '2020-01-01T00:00:00Z',
      },
    ];
    await wrapper.setData({ items: tokens });
    const table = () => wrapper.findComponent({ name: 'v-data-table' });
    const ids = () => table().props('items').map((token) => token.id);
    expect(ids()).to.deep.equal(['active', 'future', 'expired', 'revoked', 'both']);
    await wrapper.find('[data-testid="project-token-filter-active"]').trigger('click');
    expect(ids()).to.deep.equal(['active', 'future']);
    await wrapper.find('[data-testid="project-token-filter-revoked"]').trigger('click');
    expect(ids()).to.deep.equal(['revoked', 'both']);
    await wrapper.find('[data-testid="project-token-filter-all"]').trigger('click');
    expect(ids()).to.deep.equal(['active', 'future', 'expired', 'revoked', 'both']);
    expect(calls).to.have.length(0);
  });
  it('returns to the first page when switching token filters', async () => {
    await render();
    await wrapper.setData({
      items: Array.from({ length: 25 }, (_, i) => ({ ...item, id: `token-${i}` })),
    });
    await wrapper.setData({ page: 3 });
    await wrapper.find('[data-testid="project-token-filter-active"]').trigger('click');
    expect(wrapper.findComponent({ name: 'v-data-table' }).props('page')).to.equal(1);
  });
});
