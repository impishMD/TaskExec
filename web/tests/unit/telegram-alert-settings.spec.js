import { expect } from 'chai';
import { mount } from '@vue/test-utils';
import axios from 'axios';
import Vuetify from 'vuetify';
import TelegramAlertSettings from '@/components/TelegramAlertSettings.vue';

const tick = () => new Promise((resolve) => { setTimeout(resolve, 0); });

describe('Telegram alert settings', () => {
  let wrapper;
  let originalGet;
  let originalPut;
  let saved;
  const defaults = {
    channel: 'telegram',
    enabled: false,
    chat_id: '',
    has_token: false,
    global_token_configured: true,
  };
  async function render(settings = {}, projectId = 1) {
    axios.get = async () => ({ data: { ...defaults, ...settings } });
    axios.put = async (url, data) => {
      saved = { url, data };
      return {
        data: {
          ...defaults, ...settings, ...data, has_token: !!data.token,
        },
      };
    };
    wrapper = mount(TelegramAlertSettings, {
      propsData: { projectId }, vuetify: new Vuetify(), mocks: { $t: (key) => key },
    });
    await tick();
    return wrapper;
  }
  beforeEach(() => {
    originalGet = axios.get;
    originalPut = axios.put;
    saved = null;
  });
  afterEach(() => {
    if (wrapper) wrapper.destroy();
    axios.get = originalGet;
    axios.put = originalPut;
  });

  it('starts disabled, then unlocks the chat and optional override', async () => {
    await render();
    expect(wrapper.find('[data-testid="alerts-enable-telegram"]').attributes('disabled'))
      .to.equal(undefined);
    expect(wrapper.find('[data-testid="alerts-chat-id"]').attributes('disabled')).to.equal('disabled');
    expect(wrapper.find('[data-testid="alerts-save"]').attributes('disabled')).to.equal('disabled');
    await wrapper.setData({ enabled: true });
    expect(wrapper.find('[data-testid="alerts-chat-id"]').attributes('disabled')).to.equal(undefined);
    expect(wrapper.find('[data-testid="alerts-bot-token"]').attributes('disabled')).to.equal('disabled');
    await wrapper.setData({ useGlobalToken: false });
    expect(wrapper.find('[data-testid="alerts-bot-token"]').attributes('disabled')).to.equal(undefined);
  });

  it('can save disabling an enabled channel while keeping its token', async () => {
    await render({ enabled: true, chat_id: '-100123', has_token: true });
    await wrapper.setData({ enabled: false });
    expect(wrapper.vm.dirty).to.equal(true);
    await wrapper.vm.save();
    expect(saved.data.enabled).to.equal(false);
    expect(saved.data).not.to.have.property('token');
    expect(wrapper.emitted('saved')).to.have.length(1);
  });

  it('explicitly removes the override when switching back to the global token', async () => {
    await render({ enabled: true, chat_id: '-100123', has_token: true });
    await wrapper.setData({ useGlobalToken: true });
    await wrapper.vm.save();
    expect(saved.url).to.equal('/api/project/1/alerts/telegram');
    expect(saved.data.token).to.equal('');
  });

  it('saves a global token and clears plaintext from the form afterwards', async () => {
    await render({}, 0);
    await wrapper.setData({ token: ' 123:test-token ' });
    await wrapper.vm.save();
    expect(saved.url).to.equal('/api/alerts/telegram');
    expect(saved.data.token).to.equal('123:test-token');
    expect(wrapper.vm.token).to.equal('');
    expect(Boolean(wrapper.vm.dirty)).to.equal(false);
  });

  it('does not submit an empty new project override', async () => {
    await render();
    await wrapper.setData({ enabled: true, chatId: '-100123', useGlobalToken: false });
    await wrapper.vm.save();
    expect(saved).to.equal(null);
  });

  it('keeps the dialog cancellable if loading fails', async () => {
    await render();
    axios.get = async () => { throw new Error('Network unavailable'); };
    await wrapper.vm.load();
    expect(wrapper.vm.settings).to.equal(null);
    const cancel = wrapper.findAll('button').wrappers.find((button) => button.text() === 'cancel');
    expect(cancel).not.to.equal(undefined);
    await cancel.trigger('click');
    expect(wrapper.emitted('cancel')).to.have.length(1);
  });
});
