import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import AnsibleStageView from '@/components/AnsibleStageView.vue';
import TaskLogView from '@/components/TaskLogView.vue';

const settle = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Ansible task summaries', () => {
  let wrapper;
  afterEach(() => { if (wrapper) wrapper.destroy(); });

  function render(loadEndpoint) {
    wrapper = shallowMount(AnsibleStageView, {
      propsData: { projectId: 1, taskId: 1 },
      mocks: { $t: (key) => key },
      methods: { loadEndpoint },
    });
  }

  it('counts final recap hosts without duplicating unreachable errors on refresh', async () => {
    render(async (url) => (url.endsWith('/hosts')
      ? [{ host: 'ok', failed: 0, unreachable: 0 }, { host: 'bad', failed: 0, unreachable: 1 }]
      : [{ id: 1, host: 'bad', error: 'Connection refused' }]));
    await settle();
    expect(wrapper.vm.okServers).to.equal(1);
    expect(wrapper.vm.notOkServers).to.equal(1);
    expect(wrapper.vm.failedTasks).to.have.length(1);
    await wrapper.vm.loadData();
    expect(wrapper.vm.okServers).to.equal(1);
    expect(wrapper.vm.notOkServers).to.equal(1);
    expect(wrapper.vm.failedTasks).to.have.length(1);
  });

  it('shows an honest empty state for old runs or missing recaps', async () => {
    render(async () => null);
    await settle();
    expect(wrapper.text()).to.include('summaryNoRecap');
    expect(wrapper.vm.hosts).to.deep.equal([]);
    expect(wrapper.vm.failedTasks).to.deep.equal([]);
    expect(wrapper.text()).not.to.include('prod.example.com');
  });

  it('handles request failures and allows retry', async () => {
    let fail = true;
    render(async () => {
      if (fail) throw new Error('unavailable');
      return [];
    });
    await settle();
    expect(wrapper.vm.loadFailed).to.equal(true);
    expect(wrapper.text()).to.include('summaryLoadFailed');
    fail = false;
    await wrapper.vm.loadData();
    expect(wrapper.vm.loadFailed).to.equal(false);
  });

  it('ignores late responses after switching tasks', async () => {
    const pending = [];
    render((url) => new Promise((resolve) => { pending.push({ url, resolve }); }));
    await wrapper.setProps({ taskId: 2 });
    pending.filter(({ url }) => url.includes('/tasks/2/')).forEach(({ url, resolve }) => {
      resolve(url.endsWith('/hosts') ? [{ host: 'new', failed: 0 }] : []);
    });
    await settle();
    pending.filter(({ url }) => url.includes('/tasks/1/')).forEach(({ resolve }) => resolve([]));
    await settle();
    expect(wrapper.vm.hosts.map((host) => host.host)).to.deep.equal(['new']);
  });

  it('limits the summary tab to Ansible when the server supports it', () => {
    const available = TaskLogView.computed.summaryAvailable;
    expect(available.call({ app: 'ansible', systemInfo: { features: { task_summary: true } } }))
      .to.equal(true);
    ['bash', 'terraform', 'tofu', undefined].forEach((app) => {
      expect(available.call({ app, systemInfo: { features: { task_summary: true } } }))
        .to.equal(false);
    });
    expect(available.call({ app: 'ansible', systemInfo: {} })).to.equal(false);
  });
});
