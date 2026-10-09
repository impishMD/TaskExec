import { expect } from 'chai';
import { mount } from '@vue/test-utils';
import Vuetify from 'vuetify';
import axios from 'axios';
import TaskStatus from '@/components/TaskStatus.vue';

const tick = () => new Promise((resolve) => { setTimeout(resolve, 30); });

describe('Task error popup', () => {
  let wrapper;
  let originalGet;
  let calls;
  beforeEach(() => {
    originalGet = axios.get;
    calls = [];
    axios.get = async (url) => { calls.push(url); return { data: [{ output: 'ERROR: unavailable' }] }; };
  });
  afterEach(() => { if (wrapper) wrapper.destroy(); axios.get = originalGet; });
  function render(props = {}) {
    wrapper = mount(TaskStatus, {
      propsData: {
        status: 'error', projectId: 1, taskId: 2, ...props,
      },
      vuetify: new Vuetify(),
      mocks: { $t: (key) => key },
      attachTo: document.body,
    });
  }
  it('loads on click and closes when clicking the popup content', async () => {
    render();
    expect(calls).to.have.length(0);
    await wrapper.find('.v-chip').trigger('click');
    await tick();
    expect(wrapper.vm.errorOpen).to.equal(true);
    expect(calls).to.deep.equal(['/api/project/1/tasks/2/output']);
    expect(wrapper.vm.diagnostic.text).to.equal('ERROR: unavailable');
    document.querySelector('[data-testid="task-error-popup"]').click();
    await tick();
    expect(wrapper.vm.errorOpen).to.equal(false);
  });
  it('uses loaded output and never interprets the error as HTML', async () => {
    render({ output: [{ output: 'ERROR: <img src=x onerror=alert(1)>' }] });
    await wrapper.find('.v-chip').trigger('click');
    await tick();
    expect(calls).to.have.length(0);
    const popup = document.querySelector('[data-testid="task-error-popup"]');
    expect(popup.querySelector('img')).to.equal(null);
    expect(popup.textContent).to.contain('<img src=x');
  });
  it('does not activate for a successful task', async () => {
    render({ status: 'success' });
    await wrapper.find('.v-chip').trigger('click');
    await tick();
    expect(wrapper.vm.errorOpen).to.equal(false);
    expect(calls).to.have.length(0);
  });
  it('discards a late response after switching tasks', async () => {
    let resolve;
    axios.get = () => new Promise((done) => { resolve = done; });
    render();
    await wrapper.find('.v-chip').trigger('click');
    await tick();
    await wrapper.setProps({ taskId: 3 });
    resolve({ data: [{ output: 'ERROR: old task' }] });
    await tick();
    expect(wrapper.vm.records).to.deep.equal([]);
    expect(wrapper.vm.errorLoading).to.equal(false);
    expect(wrapper.vm.errorOpen).to.equal(false);
  });
});
