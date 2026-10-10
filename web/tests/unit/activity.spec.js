import { expect } from 'chai';
import Vue from 'vue';
import VueI18n from 'vue-i18n';
import Vuetify from 'vuetify';
import { mount } from '@vue/test-utils';
import formatActivityDescription from '@/lib/activity';
import Activity from '@/views/project/Activity.vue';
import { messages } from '@/lang';
import mockAxios from './helpers/axiosMock';

Vue.use(VueI18n);

const i18n = () => new VueI18n({ locale: 'ru', fallbackLocale: 'en', messages });
const event = (description) => ({ description });

describe('Project activity localization', () => {
  it('translates existing records and their task statuses without changing stored text', () => {
    const locale = i18n();
    const record = event('Task ID 123 (deploy (production)) finished with status ERROR');
    const translate = (key, params) => locale.t(key, params);
    expect(formatActivityDescription(record, translate))
      .to.equal('Задача #123 (deploy (production)) завершена со статусом: Ошибка');
    locale.locale = 'en';
    expect(formatActivityDescription(record, translate))
      .to.equal('Task #123 (deploy (production)) finished with status: Failed');
    expect(record.description).to.equal('Task ID 123 (deploy (production)) finished with status ERROR');
  });

  it('recognizes every resource event format emitted by the application', () => {
    const locale = i18n();
    const descriptions = ['Project created'];
    ['Repository', 'Environment', 'Inventory', 'Access Key', 'Host config for', 'View'].forEach((type) => {
      ['created', 'updated', 'deleted'].forEach((action) => descriptions.push(`${type} example ${action}`));
    });
    ['Template', 'Schedule'].forEach((type) => {
      ['created', 'updated', 'deleted'].forEach((action) => descriptions.push(`${type} ID 42 ${action}`));
    });
    descriptions.push(
      'Template ID 42 description updated',
      'Secret storage with ID 42 has been updated',
      'Secret storage example has been created',
      'User ID 42 added to team',
      'User ID 42 removed from team',
      'Changed role for User ID 42',
    );
    descriptions.forEach((description) => {
      const formatted = formatActivityDescription(
        event(description),
        (key, params) => locale.t(key, params),
      );
      expect(formatted, description).not.to.equal(description);
      expect(formatted, description).to.match(/[А-Яа-я]/);
      expect(formatted, description).not.to.match(/\{\w+\}|activity[A-Z]/);
    });
  });

  it('localizes every task status in all supported languages', () => {
    const locale = i18n();
    const statuses = [
      'WAITING', 'STARTING', 'WAITING_CONFIRMATION', 'CONFIRMED', 'REJECTED',
      'RUNNING', 'SUCCESS', 'ERROR', 'STOPPING', 'STOPPED',
    ];
    Object.keys(messages).forEach((language) => {
      locale.locale = language;
      statuses.forEach((status) => {
        const description = `Task ID 7 (unchanged-name) ${status}`;
        const formatted = formatActivityDescription(
          event(description),
          (key, params) => locale.t(key, params),
        );
        expect(formatted, `${language}/${status}`).not.to.equal(description);
        expect(formatted).to.contain('unchanged-name');
        expect(formatted).not.to.match(/status_|activity[A-Z]|\{\w+\}/);
      });
    });
  });

  it('preserves names, URLs, embedded action words and multiline text', () => {
    const locale = i18n();
    const translate = (key, params) => locale.t(key, params);
    const name = 'service has been created\n(host) {name}';
    expect(formatActivityDescription(event(`View ${name} updated`), translate))
      .to.equal(`Изменение: Представление «${name}»`);
    const url = 'https://git.example/team/updated.git';
    expect(formatActivityDescription(event(`Repository ${url} created`), translate))
      .to.equal(`Создание: Репозиторий «${url}»`);
    expect(formatActivityDescription(event(`Secret storage ${name} has been created`), translate))
      .to.equal(`Создание: Хранилище секретов «${name}»`);
  });

  it('retains unknown/imported descriptions and tolerates missing descriptions', () => {
    const translate = () => { throw new Error('Unknown event must not be translated'); };
    ['Custom event', 'Task ID 5 (job) FUTURE_STATUS', 'Создан пользователь'].forEach((description) => {
      expect(formatActivityDescription(event(description), translate)).to.equal(description);
    });
    expect(formatActivityDescription({}, translate)).to.equal('');
    expect(formatActivityDescription(event(null), translate)).to.equal('');
  });

  it('updates table text and headers on language switch and renders names as text', async () => {
    const http = mockAxios();
    const name = '<img src=x onerror=alert(1)>';
    const records = [{
      ...event(`View ${name} created`), created: '2026-10-10T10:00:00Z', username: 'test-user',
    }];
    http.respond(() => records);
    const locale = i18n();
    const wrapper = mount(Activity, {
      i18n: locale,
      vuetify: new Vuetify(),
      propsData: { projectId: 1 },
      stubs: { DashboardMenu: true },
    });
    try {
      await new Promise((resolve) => { setTimeout(resolve, 0); });
      expect(wrapper.text()).to.contain(`Создание: Представление «${name}»`);
      expect(wrapper.text()).to.contain('Описание');
      expect(wrapper.find('img').exists()).to.equal(false);
      locale.locale = 'en';
      await wrapper.vm.$nextTick();
      expect(wrapper.text()).to.contain(`Created: View «${name}»`);
      expect(wrapper.text()).to.contain('Description');
      expect(http.requests).to.have.length(1);
      expect(records[0].description).to.equal(`View ${name} created`);
    } finally {
      wrapper.destroy();
      http.restore();
    }
  });
});
