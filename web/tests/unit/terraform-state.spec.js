import { expect } from 'chai';
import { shallowMount } from '@vue/test-utils';
import Vuetify from 'vuetify';
import TemplateTerraformState from '@/views/project/template/TemplateTerraformState.vue';
import { USER_PERMISSIONS } from '@/lib/constants';

describe('Terraform workspace state access', () => {
  let wrapper;
  afterEach(() => { if (wrapper) wrapper.destroy(); });

  function render(props = {}) {
    wrapper = shallowMount(TemplateTerraformState, {
      vuetify: new Vuetify(),
      propsData: {
        template: {
          id: 1, project_id: 1, inventory_id: 2, app: 'tofu',
        },
        features: { terraform_backend: true },
        ...props,
      },
      mocks: { $t: (key) => key },
      methods: {
        async loadAppsDataFromBackend() { return []; },
        async loadInventories() {
          this.inventories = [{ id: 2, inventory: 'default' }];
          this.inventoryId = 2;
        },
        async loadAliases() { this.aliases = [{ id: 'alias', url: 'https://example.test/api/terraform/alias' }]; },
        async loadStates() { this.states = [{ id: 3, task_id: 4 }]; },
      },
    });
    return new Promise((resolve) => { setTimeout(resolve, 0); });
  }

  it('shows history metadata but hides secret state and alias editing from guests', async () => {
    await render({ projectPermissions: 0 });
    expect(wrapper.find('[data-testid="terraform-add-alias"]').exists()).to.equal(false);
    expect(wrapper.findComponent({ name: 'v-data-table' }).props('showExpand')).to.equal(false);
    expect(wrapper.text()).to.include('uiStateHistory');
  });

  it('allows project resource managers to expand state and manage aliases', async () => {
    await render({ projectPermissions: USER_PERMISSIONS.manageProjectResources });
    expect(wrapper.find('[data-testid="terraform-add-alias"]').exists()).to.equal(true);
    expect(wrapper.findComponent({ name: 'v-data-table' }).props('showExpand')).to.equal(true);
  });

  it('allows instance administrators without a project role', async () => {
    await render({ isAdmin: true });
    expect(wrapper.find('[data-testid="terraform-add-alias"]').exists()).to.equal(true);
  });
});
