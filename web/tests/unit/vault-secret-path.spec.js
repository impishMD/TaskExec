import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import axios from 'axios';
import VaultSecretPath from '@/components/VaultSecretPath.vue';

const flush = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Vault path completion', () => {
  let post;
  let wrapper;
  beforeEach(() => { post = axios.post; });
  afterEach(() => { wrapper.destroy(); axios.post = post; });
  const mount = () => shallowMount(VaultSecretPath, {
    propsData: { value: '', projectId: 1, storageId: 2 }, mocks: { $t: (key) => key },
  });
  it('loads root and only the typed directory, filters locally and permits manual input after a LIST failure', async () => {
    const requests = [];
    axios.post = async (url, body) => {
      requests.push({ url, ...body });
      if (body.path === 'private/') throw new Error('denied');
      return { data: { paths: body.path ? ['apps/db', 'apps/web'] : ['apps/', 'proxmox'] } };
    };
    wrapper = mount();
    await flush();
    expect(wrapper.vm.items).to.deep.equal(['apps/', 'proxmox']);
    await wrapper.setData({ search: 'ap' });
    expect(wrapper.vm.items).to.deep.equal(['apps/']);
    expect(requests).to.have.length(1);
    await wrapper.setData({ search: 'apps/d' });
    await wrapper.vm.loadPaths();
    expect(wrapper.vm.items).to.deep.equal(['apps/db']);
    expect(requests[1]).to.deep.equal({ url: '/api/project/1/secret_storages/2/paths', path: 'apps/' });
    await wrapper.setData({ search: 'private/key' });
    await wrapper.vm.loadPaths();
    expect(wrapper.vm.error).to.equal('denied');
    expect(wrapper.emitted('input').pop()[0]).to.equal('private/key');
  });
  it('ignores an old directory response after a storage switch', async () => {
    let finish;
    axios.post = () => new Promise((resolve) => { finish = resolve; });
    wrapper = mount();
    const old = finish;
    await wrapper.setProps({ storageId: 3 });
    old({ data: { paths: ['old-secret'] } });
    await flush();
    expect(wrapper.vm.paths).to.deep.equal([]);
  });
});
