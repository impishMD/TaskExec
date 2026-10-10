import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import ManagementMenu from '@/components/ManagementMenu.vue';

describe('ManagementMenu', () => {
  let wrapper;

  function render(propsData = {}) {
    wrapper = shallowMount(ManagementMenu, { propsData, mocks: { $t: (key) => key } });
    return wrapper;
  }

  afterEach(() => wrapper.destroy());

  it('shows the version without exposing personal tokens or administrative controls', () => {
    render({ isAdmin: false, version: 'dev' });
    expect(wrapper.find('[data-testid="sidebar-tokens"]').exists()).to.equal(false);
    expect(wrapper.find('[data-testid="management-users"]').exists()).to.equal(false);
    expect(wrapper.find('[data-testid="management-settings"]').exists()).to.equal(false);
    expect(wrapper.find('[data-testid="management-alerts"]').exists()).to.equal(false);
    expect(wrapper.find('[data-testid="management-roles"]').exists()).to.equal(false);
    expect(wrapper.find('[data-testid="management-system-info"]').exists()).to.equal(false);
    expect(wrapper.text()).to.include('TaskExec dev');
  });

  it('links administrators to global resources without adding a project scope', () => {
    render({ isAdmin: true });
    ['users', 'runners', 'tasks', 'apps', 'alerts', 'roles', 'settings'].forEach((id) => {
      expect(wrapper.find(`[data-testid="management-${id}"]`).attributes('to'))
        .to.equal(`/${id}`);
    });
  });

  it('opens system information through the application dialog', () => {
    render({ isAdmin: true });
    wrapper.findComponent('[data-testid="management-system-info"]').vm.$emit('click');
    expect(wrapper.emitted('system-info')).to.have.length(1);
  });
});
