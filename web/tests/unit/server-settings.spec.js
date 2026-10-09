import { expect } from 'chai';
import { mount } from '@vue/test-utils';
import axios from 'axios';
import Vuetify from 'vuetify';
import EventBus from '@/event-bus';
import ServerSettings from '@/views/ServerSettings.vue';

const tick = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Server settings', () => {
  let wrapper;
  let originalGet;
  let originalPut;
  let saved;
  let updates;
  const onUpdate = (settings) => { updates.push(settings); };
  const render = () => {
    wrapper = mount(ServerSettings, {
      vuetify: new Vuetify(), mocks: { $t: (key) => key }, stubs: ['router-link'],
    });
  };

  beforeEach(() => {
    originalGet = axios.get;
    originalPut = axios.put;
    saved = [];
    updates = [];
    axios.get = async (url) => {
      expect(url).to.equal('/api/settings');
      return { data: { use_remote_runner: false } };
    };
    axios.put = async (url, data) => {
      saved.push({ url, data });
      return { data };
    };
    EventBus.$on('i-server-settings', onUpdate);
  });

  afterEach(() => {
    if (wrapper) wrapper.destroy();
    axios.get = originalGet;
    axios.put = originalPut;
    EventBus.$off('i-server-settings', onUpdate);
  });

  it('loads the server value and saves both enabling and disabling', async () => {
    render();
    await tick();
    expect(wrapper.find('[data-testid="server-settings-save"]').attributes('disabled'))
      .to.equal('disabled');
    const setAndSave = async (enabled) => {
      await wrapper.find('[data-testid="server-remote-runners"]').setChecked(enabled);
      expect(wrapper.vm.dirty).to.equal(true);
      await wrapper.find('form').trigger('submit');
      await tick();
      expect(saved[saved.length - 1]).to.deep.equal({
        url: '/api/settings', data: { use_remote_runner: enabled },
      });
      expect(updates[updates.length - 1]).to.deep.equal({ use_remote_runner: enabled });
      expect(wrapper.vm.dirty).to.equal(false);
    };
    await setAndSave(true);
    await setAndSave(false);
  });

  it('offers retry without showing editable defaults when loading fails', async () => {
    axios.get = async () => { throw new Error('Load failed'); };
    render();
    expect(wrapper.find('[data-testid="server-remote-runners"]').exists()).to.equal(false);
    await tick();
    expect(wrapper.find('[data-testid="server-settings-error"]').text()).to.equal('Load failed');
    expect(wrapper.find('[data-testid="server-remote-runners"]').exists()).to.equal(false);
    axios.get = async () => ({ data: { use_remote_runner: true } });
    await wrapper.find('[data-testid="server-settings-retry"]').trigger('click');
    await tick();
    expect(wrapper.vm.useRemoteRunner).to.equal(true);
    expect(wrapper.vm.dirty).to.equal(false);
  });

  it('keeps an unsaved change after a failed write and leaves the app state unchanged', async () => {
    render();
    await tick();
    axios.put = async () => { throw new Error('Save failed'); };
    await wrapper.find('[data-testid="server-remote-runners"]').setChecked(true);
    await wrapper.find('form').trigger('submit');
    await tick();
    expect(wrapper.find('[data-testid="server-settings-error"]').text()).to.equal('Save failed');
    expect(wrapper.vm.useRemoteRunner).to.equal(true);
    expect(wrapper.vm.dirty).to.equal(true);
    expect(updates).to.deep.equal([{ use_remote_runner: false }]);
  });

  it('locks editing and duplicate submissions while saving', async () => {
    render();
    await tick();
    let finishSave;
    axios.put = (url, data) => {
      saved.push({ url, data });
      return new Promise((resolve) => { finishSave = () => resolve({ data }); });
    };
    await wrapper.find('[data-testid="server-remote-runners"]').setChecked(true);
    await wrapper.find('form').trigger('submit');
    expect(wrapper.find('[data-testid="server-remote-runners"]').attributes('disabled'))
      .to.equal('disabled');
    await wrapper.find('form').trigger('submit');
    expect(saved).to.have.length(1);
    finishSave();
    await tick();
    expect(wrapper.vm.dirty).to.equal(false);
  });
});
